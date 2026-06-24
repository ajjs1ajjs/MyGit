from django.shortcuts import get_object_or_404
from rest_framework import status, viewsets
from rest_framework.permissions import IsAuthenticated
from rest_framework.response import Response

from apps.repositories.models import Repository

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
        return get_object_or_404(Repository, id=repo_id)

    def list(self, request, project_id=None):
        pages = self.get_queryset()
        return Response(WikiPageSerializer(pages, many=True).data)

    def retrieve(self, request, project_id=None, slug=None):
        page = get_object_or_404(self.get_queryset(), slug=slug)
        return Response(WikiPageSerializer(page).data)

    def create(self, request, project_id=None):
        repo = self._get_repo()
        serializer = WikiPageSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        page = serializer.save(repository=repo, author=request.user)
        return Response(WikiPageSerializer(page).data, status=status.HTTP_201_CREATED)

    def update(self, request, project_id=None, slug=None):
        page = get_object_or_404(self.get_queryset(), slug=slug)
        serializer = WikiPageSerializer(page, data=request.data)
        serializer.is_valid(raise_exception=True)
        serializer.save()
        return Response(WikiPageSerializer(page).data)

    def destroy(self, request, project_id=None, slug=None):
        page = get_object_or_404(self.get_queryset(), slug=slug)
        page.delete()
        return Response(status=status.HTTP_204_NO_CONTENT)
