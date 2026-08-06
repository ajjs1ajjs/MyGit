from django.conf import settings
from django.db import transaction
from django.shortcuts import get_object_or_404
from django.utils import timezone
from django.utils.crypto import constant_time_compare
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
    else:
        pipeline.status = Pipeline.Status.RUNNING

    if pipeline.status in (
        Pipeline.Status.SUCCESS,
        Pipeline.Status.FAILED,
        Pipeline.Status.CANCELED,
    ):
        pipeline.finished_at = timezone.now()
    else:
        pipeline.finished_at = None

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
        project_id = self.kwargs.get("project_id", "")
        return Job.objects.filter(
            pipeline_id=pipeline_id, pipeline__repository_id=project_id
        )

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
            raise PermissionDenied("You do not have access to this repository.")

    def _require_internal_token(self, request):
        expected = getattr(settings, "MYGIT_INTERNAL_API_TOKEN", "")
        token = request.META.get("HTTP_AUTHORIZATION", "")
        if not expected or not constant_time_compare(token, f"Bearer {expected}"):
            raise PermissionDenied("Runner authentication required.")

    def retrieve(self, request, project_id=None, pipeline_id=None, id=None):
        self._get_repo()
        job = get_object_or_404(self.get_queryset(), id=id)
        return Response(JobSerializer(job).data)

    @action(methods=["post"], detail=True)
    def claim(self, request, project_id=None, pipeline_id=None, id=None):
        self._require_internal_token(request)
        with transaction.atomic():
            job = get_object_or_404(
                Job.objects.select_for_update(), id=id, pipeline_id=pipeline_id
            )
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
        self._require_internal_token(request)
        job = get_object_or_404(self.get_queryset(), id=id)
        serializer = JobLogUpdateSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)

        if "log" in serializer.validated_data:
            MAX_LOG_SIZE = 10 * 1024 * 1024  # 10 MB
            new_log = serializer.validated_data["log"]
            if len(job.log) + len(new_log) > MAX_LOG_SIZE:
                job.log = job.log[-MAX_LOG_SIZE:] + "\n... (truncated) ...\n" + new_log
            else:
                job.log += new_log
        if "status" in serializer.validated_data:
            new_status = serializer.validated_data["status"]
            job.status = new_status
            if new_status in ("success", "failed"):
                job.finished_at = timezone.now()

        job.save(update_fields=["log", "status", "finished_at"])

        if job.status in (Job.Status.SUCCESS, Job.Status.FAILED):
            _update_pipeline_status(job.pipeline)

        # Broadcast log update to WebSocket (best-effort; never fail the request
        # when the channel layer is briefly unavailable).
        try:
            from asgiref.sync import async_to_sync
            from channels.layers import get_channel_layer
            channel_layer = get_channel_layer()
            async_to_sync(channel_layer.group_send)(
                f"job_{job.id}",
                {
                    "type": "job.log",
                    "data": serializer.validated_data.get("log", ""),
                    "status": new_status if "status" in serializer.validated_data else job.status,
                },
            )
        except Exception:
            pass

        return Response(JobSerializer(job).data)

    @action(methods=["get"], detail=False)
    def pending(self, request, project_id=None, pipeline_id=None):
        self._require_internal_token(request)
        jobs = (
            self.get_queryset()
            .filter(status=Job.Status.PENDING)
            .order_by("stage", "created_at")[:1]
        )
        return Response(JobSerializer(jobs, many=True).data)

    @action(methods=["post"], detail=True, url_path="artifacts")
    def upload_artifact(self, request, project_id=None, pipeline_id=None, id=None):
        self._require_internal_token(request)
        job = get_object_or_404(self.get_queryset(), id=id)
        file = request.FILES.get("file")
        name = request.data.get("name", "")
        if not file or not name:
            return Response({"detail": "file and name are required."}, status=status.HTTP_400_BAD_REQUEST)

        import os

        sanitized_name = os.path.basename(name)
        if not sanitized_name:
            return Response({"detail": "Invalid file name."}, status=status.HTTP_400_BAD_REQUEST)

        artifacts_dir = settings.BASE_DIR / "artifacts" / str(job.pipeline.repository_id) / str(pipeline_id) / str(job.id)
        artifacts_dir.mkdir(parents=True, exist_ok=True)

        file_path = artifacts_dir / sanitized_name
        with open(file_path, "wb+") as f:
            for chunk in file.chunks():
                f.write(chunk)

        current = job.artifacts or []
        if sanitized_name not in current:
            current.append(sanitized_name)
        job.artifacts = current
        job.save(update_fields=["artifacts"])

        return Response({"detail": "OK", "name": sanitized_name, "size": file.size})

    @action(methods=["get"], detail=True, url_path="artifacts/(?P<artifact_name>.+)")
    def download_artifact(self, request, project_id=None, pipeline_id=None, id=None, artifact_name=None):
        self._get_repo()
        job = get_object_or_404(self.get_queryset(), id=id)
        import os
        sanitized_name = os.path.basename(artifact_name)
        if not sanitized_name:
            return Response({"detail": "Invalid file name."}, status=status.HTTP_400_BAD_REQUEST)
        artifacts_dir = settings.BASE_DIR / "artifacts" / str(job.pipeline.repository_id) / str(pipeline_id) / str(job.id)
        file_path = artifacts_dir / sanitized_name
        if not file_path.exists():
            return Response({"detail": "Artifact not found."}, status=status.HTTP_404_NOT_FOUND)

        from django.http import FileResponse
        response = FileResponse(open(file_path, "rb"))
        response["Content-Disposition"] = f'attachment; filename="{sanitized_name}"'
        return response
