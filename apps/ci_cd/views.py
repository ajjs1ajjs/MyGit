from django.shortcuts import get_object_or_404
from django.utils import timezone
from rest_framework import status, viewsets
from rest_framework.decorators import action
from rest_framework.exceptions import PermissionDenied
from rest_framework.permissions import IsAuthenticated
from rest_framework.response import Response

from apps.git_service.backend import GitBackend, GitServiceError
from apps.repositories.models import Repository, RepositoryAccess

from .config_parser import CiConfigError, build_default_config, parse_ci_config
from .models import Job, Pipeline
from .serializers import (
    JobLogUpdateSerializer,
    JobSerializer,
    PipelineCreateSerializer,
    PipelineSerializer,
)


def _get_config_for_pipeline(pipeline):
    try:
        backend = GitBackend(pipeline.repository.disk_path)
        content, _ = backend.get_blob_by_path(pipeline.sha, ".mygit-ci.yml")
        return parse_ci_config(content)
    except Exception:
        return build_default_config()


def _get_job_script(pipeline, job_name):
    config = _get_config_for_pipeline(pipeline)
    job_cfg = config.get("jobs", {}).get(job_name, {})
    return job_cfg.get("script", [])


def _get_job_image(pipeline, job_name):
    config = _get_config_for_pipeline(pipeline)
    job_cfg = config.get("jobs", {}).get(job_name, {})
    return job_cfg.get("image", "python:3.12-slim")


def _update_pipeline_status(pipeline):
    jobs = pipeline.jobs.all()
    if all(j.status == Job.Status.SUCCESS for j in jobs):
        pipeline.status = Pipeline.Status.SUCCESS
    elif any(j.status == Job.Status.FAILED for j in jobs):
        pipeline.status = Pipeline.Status.FAILED
    elif any(j.status == Job.Status.CANCELED for j in jobs):
        pipeline.status = Pipeline.Status.CANCELED
    elif all(
        j.status in (Job.Status.SUCCESS, Job.Status.FAILED, Job.Status.CANCELED) for j in jobs
    ):
        pipeline.status = Pipeline.Status.SUCCESS
    else:
        pipeline.status = Pipeline.Status.RUNNING
    pipeline.finished_at = timezone.now()
    pipeline.save(update_fields=["status", "finished_at"])


class PipelineViewSet(viewsets.GenericViewSet):
    permission_classes = [IsAuthenticated]
    lookup_field = "id"

    def get_serializer_class(self):
        return PipelineSerializer

    def get_queryset(self):
        return Pipeline.objects.filter(repository=self._get_repo()).prefetch_related("jobs")

    def _get_repo(self):
        repo_id = self.kwargs.get("project_id", "")
        repo = get_object_or_404(Repository, id=repo_id)
        self._ensure_repo_access(repo)
        return repo

    def _ensure_repo_access(self, repo):
        user = self.request.user
        is_public = repo.visibility == Repository.Visibility.PUBLIC
        is_owner = str(repo.owner_id) == str(user.id)
        has_access = RepositoryAccess.objects.filter(
            repository=repo, user=user, role__gte=RepositoryAccess.Role.GUEST
        ).exists()
        if not is_public and not is_owner and not has_access:
            raise PermissionDenied(
                "You do not have access to this repository."
            )

    def _get_ci_config(self, repo, sha: str) -> dict:
        try:
            backend = GitBackend(repo.disk_path)
            content, _ = backend.get_blob_by_path(sha, ".mygit-ci.yml")
            return parse_ci_config(content)
        except (GitServiceError, CiConfigError, KeyError):
            return build_default_config()

    def list(self, request, project_id=None):
        queryset = self.get_queryset()
        page = self.paginate_queryset(queryset)
        if page is not None:
            return self.get_paginated_response(PipelineSerializer(page, many=True).data)
        return Response(PipelineSerializer(queryset, many=True).data)

    def retrieve(self, request, project_id=None, id=None):
        pipeline = get_object_or_404(self.get_queryset(), id=id)
        return Response(PipelineSerializer(pipeline).data)

    def create(self, request, project_id=None):
        repo = self._get_repo()
        serializer = PipelineCreateSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)

        ref = serializer.validated_data["ref"]
        sha = serializer.validated_data["sha"]

        try:
            ci_config = self._get_ci_config(repo, sha)
        except CiConfigError as e:
            return Response({"detail": str(e)}, status=status.HTTP_400_BAD_REQUEST)

        pipeline = Pipeline.objects.create(
            repository=repo,
            author=request.user,
            ref=ref,
            sha=sha,
            status=Pipeline.Status.PENDING,
            stages=ci_config.get("stages", []),
        )

        for job_name, job_config in ci_config.get("jobs", {}).items():
            Job.objects.create(
                pipeline=pipeline,
                name=job_name,
                stage=job_config.get("stage", "test"),
                status=Job.Status.PENDING,
            )

        return Response(PipelineSerializer(pipeline).data, status=status.HTTP_201_CREATED)

    @action(methods=["post"], detail=True)
    def cancel(self, request, project_id=None, id=None):
        pipeline = get_object_or_404(self.get_queryset(), id=id)
        if pipeline.status in (Pipeline.Status.RUNNING, Pipeline.Status.PENDING):
            pipeline.status = Pipeline.Status.CANCELED
            pipeline.save(update_fields=["status"])
            pipeline.jobs.filter(status__in=["pending", "running"]).update(
                status=Job.Status.CANCELED
            )
        return Response(PipelineSerializer(pipeline).data)


class JobViewSet(viewsets.GenericViewSet):
    permission_classes = [IsAuthenticated]
    lookup_field = "id"
    serializer_class = JobSerializer

    def get_queryset(self):
        pipeline_id = self.kwargs.get("pipeline_id", "")
        return Job.objects.filter(pipeline_id=pipeline_id)

    def retrieve(self, request, project_id=None, pipeline_id=None, id=None):
        job = get_object_or_404(self.get_queryset(), id=id)
        return Response(JobSerializer(job).data)

    @action(methods=["post"], detail=True)
    def claim(self, request, project_id=None, pipeline_id=None, id=None):
        job = get_object_or_404(self.get_queryset(), id=id)
        if job.status != Job.Status.PENDING:
            return Response(
                {"detail": "Job is not pending."},
                status=status.HTTP_409_CONFLICT,
            )

        runner_id = request.data.get("runner_id", "")
        job.status = Job.Status.RUNNING
        job.runner_id = runner_id
        job.started_at = timezone.now()
        job.save(update_fields=["status", "runner_id", "started_at"])

        pipeline = job.pipeline
        if pipeline.status == Pipeline.Status.PENDING:
            pipeline.status = Pipeline.Status.RUNNING
            pipeline.started_at = timezone.now()
            pipeline.save(update_fields=["status", "started_at"])

        return Response(
            {
                "id": str(job.id),
                "name": job.name,
                "stage": job.stage,
                "script": _get_job_script(pipeline, job.name),
                "image": _get_job_image(pipeline, job.name),
            }
        )

    @action(methods=["post"], detail=True)
    def update_log(self, request, project_id=None, pipeline_id=None, id=None):
        job = get_object_or_404(self.get_queryset(), id=id)
        serializer = JobLogUpdateSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)

        if "log" in serializer.validated_data:
            job.log += serializer.validated_data["log"]
        if "status" in serializer.validated_data:
            new_status = serializer.validated_data["status"]
            job.status = new_status
            if new_status in ("success", "failed"):
                job.finished_at = timezone.now()

        job.save(update_fields=["log", "status", "finished_at"])

        if job.status in (Job.Status.SUCCESS, Job.Status.FAILED):
            _update_pipeline_status(job.pipeline)

        return Response(JobSerializer(job).data)

    @action(methods=["get"], detail=False)
    def pending(self, request, project_id=None, pipeline_id=None):
        jobs = (
            self.get_queryset()
            .filter(status=Job.Status.PENDING)
            .order_by("stage", "created_at")[:1]
        )
        return Response(JobSerializer(jobs, many=True).data)
