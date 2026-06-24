from django.shortcuts import get_object_or_404
from rest_framework import status, viewsets
from rest_framework.exceptions import PermissionDenied
from rest_framework.permissions import IsAuthenticated
from rest_framework.response import Response

from apps.api.mixins import ensure_repo_access
from apps.repositories.models import Repository, RepositoryAccess

from .models import WikiPage
from .serializers import WikiPageSerializer


class WikiPageViewSet(viewsets.GenericViewSet):
    permission_classes = [IsAuthenticated]
    lookup_field = "slug"
    lookup_value_regex = "[^/]+"

    def get_serializer_class(self):
        return WikiPageSerializer

    def get_queryset(self):
        return WikiPage.objects.filter(repository=self._get_repo())

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

    def list(self, request, project_id=None):
        pages = self.get_queryset()
        return Response(WikiPageSerializer(pages, many=True).data)

    def retrieve(self, request, project_id=None, slug=None):
        page = get_object_or_404(self.get_queryset(), slug=slug)
        return Response(WikiPageSerializer(page).data)

    def create(self, request, project_id=None):
        repo = self._get_repo()
        ensure_repo_access(
            repo,
            request.user,
            min_role=RepositoryAccess.Role.DEVELOPER,
            allow_public_read=False,
        )
        serializer = WikiPageSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        page = serializer.save(repository=repo, author=request.user)
        return Response(WikiPageSerializer(page).data, status=status.HTTP_201_CREATED)

    def update(self, request, project_id=None, slug=None):
        page = get_object_or_404(self.get_queryset(), slug=slug)
        ensure_repo_access(
            page.repository,
            request.user,
            min_role=RepositoryAccess.Role.DEVELOPER,
            allow_public_read=False,
        )
        serializer = WikiPageSerializer(page, data=request.data)
        serializer.is_valid(raise_exception=True)
        serializer.save()
        return Response(WikiPageSerializer(page).data)

    def destroy(self, request, project_id=None, slug=None):
        page = get_object_or_404(self.get_queryset(), slug=slug)
        ensure_repo_access(
            page.repository,
            request.user,
            min_role=RepositoryAccess.Role.DEVELOPER,
            allow_public_read=False,
        )
        page.delete()
        return Response(status=status.HTTP_204_NO_CONTENT)
