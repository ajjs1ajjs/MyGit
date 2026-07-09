from django.shortcuts import get_object_or_404
from django.utils import timezone
from rest_framework import status, viewsets
from rest_framework.decorators import action
from rest_framework.permissions import IsAuthenticated
from rest_framework.response import Response

from apps.api.mixins import ensure_repo_access
from apps.repositories.models import Repository, RepositoryAccess

from .models import Issue, IssueComment, Label, Milestone
from .serializers import (
    IssueCommentSerializer,
    IssueCreateSerializer,
    IssueDetailSerializer,
    IssueListSerializer,
    LabelSerializer,
    MilestoneSerializer,
)


class IssueViewSet(viewsets.GenericViewSet):
    permission_classes = [IsAuthenticated]
    lookup_field = "number"

    def get_serializer_class(self):
        if self.action == "create":
            return IssueCreateSerializer
        if self.action in ("comments", "create_comment"):
            return IssueCommentSerializer
        if self.action in ("retrieve", "partial_update"):
            return IssueDetailSerializer
        if self.action in ("labels", "milestones"):
            return LabelSerializer
        return IssueListSerializer

    def get_queryset(self):
        repo = self._get_repo()
        return (
            Issue.objects.filter(repository=repo)
            .select_related("author", "assignee", "milestone")
            .prefetch_related("labels")
        )

    def _get_repo(self):
        repo_id = self.kwargs.get("project_id", "")
        repo = get_object_or_404(Repository, id=repo_id)
        self._ensure_repo_access(repo)
        return repo

    def _ensure_repo_access(self, repo):
        ensure_repo_access(repo, self.request.user, allow_public_read=True)

    def list(self, request, project_id=None):
        queryset = self.get_queryset()

        state = request.query_params.get("state")
        if state:
            queryset = queryset.filter(state=state)

        milestone = request.query_params.get("milestone")
        if milestone:
            queryset = queryset.filter(milestone__id=milestone)

        label = request.query_params.get("label")
        if label:
            queryset = queryset.filter(labels__id=label)

        search = request.query_params.get("search", "")
        if search:
            queryset = queryset.filter(title__icontains=search)

        page = self.paginate_queryset(queryset)
        if page is not None:
            return self.get_paginated_response(IssueListSerializer(page, many=True).data)
        return Response(IssueListSerializer(queryset, many=True).data)

    def create(self, request, project_id=None):
        repo = self._get_repo()
        serializer = IssueCreateSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        issue = serializer.save(repository=repo, author=request.user)
        return Response(IssueDetailSerializer(issue).data, status=status.HTTP_201_CREATED)

    def retrieve(self, request, project_id=None, number=None):
        issue = get_object_or_404(self.get_queryset(), number=number)
        return Response(IssueDetailSerializer(issue).data)

    def partial_update(self, request, project_id=None, number=None):
        issue = get_object_or_404(self.get_queryset(), number=number)
        data = request.data.copy()

        if "state" in data and data["state"] == "closed" and issue.state == "open":
            data["closed_at"] = timezone.now()
        elif "state" in data and data["state"] == "open" and issue.state == "closed":
            data["closed_at"] = None

        serializer = IssueDetailSerializer(issue, data=data, partial=True)
        serializer.is_valid(raise_exception=True)
        serializer.save()
        return Response(IssueDetailSerializer(issue).data)

    @action(methods=["get", "post"], detail=True)
    def comments(self, request, project_id=None, number=None):
        issue = get_object_or_404(self.get_queryset(), number=number)

        if request.method == "GET":
            comments = IssueComment.objects.filter(issue=issue)
            page = self.paginate_queryset(comments)
            if page is not None:
                return self.get_paginated_response(IssueCommentSerializer(page, many=True).data)
            return Response(IssueCommentSerializer(comments, many=True).data)

        serializer = IssueCommentSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        comment = serializer.save(issue=issue, author=request.user)
        return Response(IssueCommentSerializer(comment).data, status=status.HTTP_201_CREATED)

    @action(methods=["get"], detail=False)
    def labels(self, request, project_id=None):
        repo = self._get_repo()
        labels = Label.objects.filter(repository=repo)
        return Response(LabelSerializer(labels, many=True).data)

    @action(methods=["post"], detail=False)
    def create_label(self, request, project_id=None):
        repo = self._get_repo()
        serializer = LabelSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        label = serializer.save(repository=repo)
        return Response(LabelSerializer(label).data, status=status.HTTP_201_CREATED)

    @action(methods=["get"], detail=False)
    def milestones(self, request, project_id=None):
        repo = self._get_repo()
        milestones = Milestone.objects.filter(repository=repo)
        return Response(MilestoneSerializer(milestones, many=True).data)

    @action(methods=["post"], detail=False)
    def create_milestone(self, request, project_id=None):
        repo = self._get_repo()
        serializer = MilestoneSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        milestone = serializer.save(repository=repo)
        return Response(MilestoneSerializer(milestone).data, status=status.HTTP_201_CREATED)
