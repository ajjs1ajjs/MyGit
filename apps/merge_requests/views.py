from django.shortcuts import get_object_or_404
from django.utils import timezone
from rest_framework import status, viewsets
from rest_framework.decorators import action
from rest_framework.permissions import IsAuthenticated
from rest_framework.response import Response

from apps.api.mixins import ensure_repo_access
from apps.git_service.backend import GitBackend, GitServiceError
from apps.repositories.models import ProtectedBranch, Repository, RepositoryAccess

from .models import MergeRequest, MergeRequestComment, MergeRequestReview
from .serializers import (
    MergeRequestCommentSerializer,
    MergeRequestCreateSerializer,
    MergeRequestDetailSerializer,
    MergeRequestListSerializer,
    MergeRequestReviewSerializer,
)


class MergeRequestViewSet(viewsets.GenericViewSet):
    permission_classes = [IsAuthenticated]
    lookup_field = "number"

    def get_serializer_class(self):
        if self.action == "create":
            return MergeRequestCreateSerializer
        if self.action in ("comments", "create_comment"):
            return MergeRequestCommentSerializer
        if self.action in ("reviews", "create_review"):
            return MergeRequestReviewSerializer
        if self.action in ("retrieve", "partial_update"):
            return MergeRequestDetailSerializer
        return MergeRequestListSerializer

    def get_queryset(self):
        repo = self._get_repo()
        return MergeRequest.objects.filter(repository=repo).select_related(
            "author", "assignee", "merged_by"
        )

    def _get_repo(self):
        repo_id = self.kwargs.get("project_id", "")
        repo = get_object_or_404(Repository, id=repo_id)
        self._ensure_repo_access(repo)
        return repo

    def _ensure_repo_access(self, repo):
        ensure_repo_access(repo, self.request.user, allow_public_read=True)

    def _get_backend(self, repo):
        backend = GitBackend(repo.disk_path)
        if not backend.exists():
            raise GitServiceError("Repository not found on disk.")
        return backend

    def list(self, request, project_id=None):
        queryset = self.get_queryset()
        state = request.query_params.get("state")
        if state:
            queryset = queryset.filter(state=state)

        page = self.paginate_queryset(queryset)
        if page is not None:
            return self.get_paginated_response(MergeRequestListSerializer(page, many=True).data)
        return Response(MergeRequestListSerializer(queryset, many=True).data)

    def create(self, request, project_id=None):
        repo = self._get_repo()
        ensure_repo_access(
            repo,
            request.user,
            min_role=RepositoryAccess.Role.DEVELOPER,
            allow_public_read=False,
        )
        serializer = MergeRequestCreateSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        mr = serializer.save(repository=repo, author=request.user)
        return Response(MergeRequestDetailSerializer(mr).data, status=status.HTTP_201_CREATED)

    def retrieve(self, request, project_id=None, number=None):
        mr = get_object_or_404(self.get_queryset(), number=number)
        return Response(MergeRequestDetailSerializer(mr).data)

    def partial_update(self, request, project_id=None, number=None):
        mr = get_object_or_404(self.get_queryset(), number=number)
        serializer = MergeRequestDetailSerializer(mr, data=request.data, partial=True)
        serializer.is_valid(raise_exception=True)
        serializer.save()
        return Response(MergeRequestDetailSerializer(mr).data)

    @action(methods=["post"], detail=True)
    def merge(self, request, project_id=None, number=None):
        mr = get_object_or_404(self.get_queryset(), number=number)
        repo = self._get_repo()
        ensure_repo_access(
            repo,
            request.user,
            min_role=RepositoryAccess.Role.DEVELOPER,
            allow_public_read=False,
        )

        if mr.state not in (MergeRequest.State.OPEN, MergeRequest.State.DRAFT):
            return Response(
                {"detail": "Only open merge requests can be merged."},
                status=status.HTTP_400_BAD_REQUEST,
            )

        protected = _matching_protected_branch(repo, mr.target_branch)
        if protected:
            approvals = mr.reviews.filter(approved=True).exclude(author=mr.author).count()
            if approvals < protected.required_approvals:
                return Response(
                    {
                        "detail": (
                            f"Protected branch requires {protected.required_approvals} "
                            f"approval(s); got {approvals}."
                        )
                    },
                    status=status.HTTP_409_CONFLICT,
                )

        try:
            backend = self._get_backend(repo)
            merge_method = request.data.get("method", "merge-commit")

            if mr.target_branch == mr.source_branch:
                return Response(
                    {"detail": "Source and target branches are the same."},
                    status=status.HTTP_400_BAD_REQUEST,
                )

            if merge_method == "fast-forward":
                result = backend.fast_forward_merge(mr.source_branch, mr.target_branch)
            else:
                result = backend.merge_commit(mr.source_branch, mr.target_branch, mr.title)

            mr.state = MergeRequest.State.MERGED
            mr.merge_commit_sha = result.get("sha", "")
            mr.merged_at = timezone.now()
            mr.merged_by = request.user
            mr.save(update_fields=["state", "merge_commit_sha", "merged_at", "merged_by"])

            for issue in mr.closes_issues.all():
                issue.state = "closed"
                issue.closed_at = timezone.now()
                issue.save(update_fields=["state", "closed_at"])

            return Response(MergeRequestDetailSerializer(mr).data)

        except GitServiceError as e:
            return Response({"detail": str(e)}, status=status.HTTP_409_CONFLICT)

    @action(methods=["get"], detail=True)
    def diff(self, request, project_id=None, number=None):
        mr = get_object_or_404(self.get_queryset(), number=number)
        repo = self._get_repo()
        try:
            backend = self._get_backend(repo)
            diffs = backend.get_merge_request_diff(mr.target_branch, mr.source_branch)
            return Response({"diffs": diffs})
        except GitServiceError as e:
            return Response({"detail": str(e)}, status=404)

    @action(methods=["get", "post"], detail=True)
    def comments(self, request, project_id=None, number=None):
        mr = get_object_or_404(self.get_queryset(), number=number)

        if request.method == "GET":
            comments = MergeRequestComment.objects.filter(merge_request=mr)
            page = self.paginate_queryset(comments)
            if page is not None:
                return self.get_paginated_response(
                    MergeRequestCommentSerializer(page, many=True).data
                )
            return Response(MergeRequestCommentSerializer(comments, many=True).data)

        serializer = MergeRequestCommentSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        comment = serializer.save(merge_request=mr, author=request.user)
        return Response(
            MergeRequestCommentSerializer(comment).data,
            status=status.HTTP_201_CREATED,
        )

    @action(methods=["get", "post"], detail=True)
    def reviews(self, request, project_id=None, number=None):
        mr = get_object_or_404(self.get_queryset(), number=number)

        if request.method == "GET":
            reviews = MergeRequestReview.objects.filter(merge_request=mr)
            return Response(MergeRequestReviewSerializer(reviews, many=True).data)

        serializer = MergeRequestReviewSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        review, created = MergeRequestReview.objects.update_or_create(
            merge_request=mr,
            author=request.user,
            defaults={
                "body": serializer.validated_data.get("body", ""),
                "approved": serializer.validated_data.get("approved", False),
            },
        )
        return Response(
            MergeRequestReviewSerializer(review).data,
            status=status.HTTP_201_CREATED,
        )


def _matching_protected_branch(repo: Repository, branch: str) -> ProtectedBranch | None:
    for rule in repo.protected_branches.all():
        if rule.matches(branch):
            return rule
    return None
