from django.db.models import Q
from django.http import HttpResponse
from rest_framework import serializers, status, viewsets
from rest_framework.decorators import action
from rest_framework.permissions import IsAuthenticated
from rest_framework.response import Response

from apps.git_service.backend import GitBackend, GitServiceError
from apps.repositories.models import Repository, RepositoryAccess


class RepositorySerializer(serializers.ModelSerializer):
    class Meta:
        model = Repository
        fields = [
            "id",
            "name",
            "path",
            "description",
            "visibility",
            "default_branch",
            "is_archived",
            "is_fork",
            "forked_from",
            "size_kb",
            "created_at",
            "updated_at",
        ]
        read_only_fields = [
            "id",
            "path",
            "is_fork",
            "forked_from",
            "size_kb",
            "created_at",
            "updated_at",
        ]

    def create(self, validated_data):
        request = self.context["request"]
        validated_data["owner_type"] = "user"
        validated_data["owner_id"] = request.user.id
        validated_data["path"] = f"{request.user.username}/{validated_data['name']}"
        repo = Repository.objects.create(**validated_data)

        GitBackend(repo.disk_path).init_bare()
        repo.size_kb = GitBackend.get_repo_size_kb(repo.disk_path)
        repo.save(update_fields=["size_kb"])

        RepositoryAccess.objects.create(
            user=request.user, repository=repo, role=RepositoryAccess.Role.OWNER
        )

        return repo


class ProjectViewSet(viewsets.ModelViewSet):
    serializer_class = RepositorySerializer
    permission_classes = [IsAuthenticated]
    lookup_field = "id"

    def get_queryset(self):
        user = self.request.user
        return (
            Repository.objects.filter(
                Q(visibility=Repository.Visibility.PUBLIC)
                | Q(owner_id=user.id)
                | Q(access_list__user=user, access_list__role__gte=RepositoryAccess.Role.GUEST)
            )
            .distinct()
            .order_by("-updated_at")
        )

    @action(methods=["post"], detail=True)
    def fork(self, request, id=None):
        repo = self.get_object()
        new_name = request.data.get("name", f"{repo.name}-fork")
        new_path = f"{request.user.username}/{new_name}"

        if Repository.objects.filter(path=new_path).exists():
            return Response(
                {"detail": "Repository already exists."}, status=status.HTTP_409_CONFLICT
            )

        new_repo = Repository.objects.create(
            owner_type="user",
            owner_id=request.user.id,
            name=new_name,
            path=new_path,
            description=f"Fork of {repo.path}",
            visibility=repo.visibility,
            default_branch=repo.default_branch,
            is_fork=True,
            forked_from=repo,
        )
        RepositoryAccess.objects.create(
            user=request.user, repository=new_repo, role=RepositoryAccess.Role.OWNER
        )
        return Response(RepositorySerializer(new_repo).data, status=status.HTTP_201_CREATED)

    def _get_backend(self, repo) -> GitBackend:
        backend = GitBackend(repo.disk_path)
        if not backend.exists():
            raise GitServiceError("Repository not found on disk.")
        return backend

    @action(methods=["get"], detail=True)
    def tree(self, request, id=None):
        repo = self.get_object()
        ref = request.query_params.get("ref", repo.default_branch)
        path = request.query_params.get("path", "")
        try:
            backend = self._get_backend(repo)
            entries = backend.get_tree(ref, path)
            return Response({"ref": ref, "path": path, "entries": entries})
        except GitServiceError as e:
            return Response({"detail": str(e)}, status=404)

    @action(methods=["get"], detail=True, url_path="blobs/(?P<sha>[^/]+)")
    def blob(self, request, id=None, sha=None):
        repo = self.get_object()
        ref = request.query_params.get("ref", repo.default_branch)
        path = request.query_params.get("path", "")
        try:
            backend = self._get_backend(repo)
            if path:
                content, blob_sha = backend.get_blob_by_path(ref, path)
            else:
                content, blob_sha = backend.get_blob(sha)
            return Response({"sha": blob_sha, "content": content})
        except (GitServiceError, KeyError) as e:
            return Response({"detail": str(e)}, status=404)

    @action(methods=["get"], detail=True, url_path="blame")
    def blame(self, request, id=None):
        repo = self.get_object()
        ref = request.query_params.get("ref", repo.default_branch)
        path = request.query_params.get("path", "")
        if not path:
            return Response({"detail": "path parameter is required."}, status=400)
        try:
            backend = self._get_backend(repo)
            lines = backend.get_blame(ref, path)
            return Response({"ref": ref, "path": path, "lines": lines})
        except GitServiceError as e:
            return Response({"detail": str(e)}, status=404)

    @action(methods=["get"], detail=True)
    def commits(self, request, id=None):
        repo = self.get_object()
        ref = request.query_params.get("ref", repo.default_branch)
        page = int(request.query_params.get("page", 1))
        try:
            backend = self._get_backend(repo)
            result = backend.get_commits(ref, page=page)
            return Response({"ref": ref, "commits": result})
        except GitServiceError as e:
            return Response({"detail": str(e)}, status=404)

    @action(methods=["get"], detail=True, url_path="commits/(?P<sha>[^/]+)")
    def commit_detail(self, request, id=None, sha=None):
        repo = self.get_object()
        try:
            backend = self._get_backend(repo)
            result = backend.get_commit(sha)
            if not result:
                return Response({"detail": "Commit not found."}, status=404)
            return Response(result)
        except GitServiceError as e:
            return Response({"detail": str(e)}, status=404)

    @action(methods=["get"], detail=True, url_path="commits/(?P<sha>[^/]+)/diff")
    def commit_diff(self, request, id=None, sha=None):
        repo = self.get_object()
        try:
            backend = self._get_backend(repo)
            diffs = backend.get_commit_diff(sha)
            return Response({"sha": sha, "diffs": diffs})
        except GitServiceError as e:
            return Response({"detail": str(e)}, status=404)

    @action(methods=["get", "post", "delete"], detail=True, url_path="branches")
    def branches(self, request, id=None):
        repo = self.get_object()
        backend = self._get_backend(repo)

        if request.method == "GET":
            branches = backend.get_branches()
            return Response({"branches": branches})

        if request.method == "POST":
            name = request.data.get("name", "")
            ref = request.data.get("ref", repo.default_branch)
            if not name:
                return Response({"detail": "Branch name is required."}, status=400)
            try:
                result = backend.create_branch(name, ref)
                return Response(result, status=status.HTTP_201_CREATED)
            except Exception as e:
                return Response({"detail": str(e)}, status=400)

        return Response({"detail": "Method not allowed."}, status=405)

    @action(methods=["delete"], detail=True, url_path="branches/(?P<name>[^/]+)")
    def delete_branch(self, request, id=None, name=None):
        repo = self.get_object()
        try:
            backend = self._get_backend(repo)
            backend.delete_branch(name)
            return Response(status=status.HTTP_204_NO_CONTENT)
        except GitServiceError as e:
            return Response({"detail": str(e)}, status=404)
        except Exception as e:
            return Response({"detail": str(e)}, status=400)

    @action(methods=["get", "post", "delete"], detail=True, url_path="tags")
    def tags(self, request, id=None):
        repo = self.get_object()
        backend = self._get_backend(repo)

        if request.method == "GET":
            tag_list = backend.get_tags()
            return Response({"tags": tag_list})

        if request.method == "POST":
            name = request.data.get("name", "")
            ref = request.data.get("ref", repo.default_branch)
            msg = request.data.get("message", "")
            if not name:
                return Response({"detail": "Tag name is required."}, status=400)
            try:
                result = backend.create_tag(name, ref, message=msg)
                return Response(result, status=status.HTTP_201_CREATED)
            except Exception as e:
                return Response({"detail": str(e)}, status=400)

        return Response({"detail": "Method not allowed."}, status=405)

    @action(methods=["delete"], detail=True, url_path="tags/(?P<name>[^/]+)")
    def delete_tag(self, request, id=None, name=None):
        repo = self.get_object()
        try:
            backend = self._get_backend(repo)
            backend.delete_tag(name)
            return Response(status=status.HTTP_204_NO_CONTENT)
        except GitServiceError as e:
            return Response({"detail": str(e)}, status=404)
        except Exception as e:
            return Response({"detail": str(e)}, status=400)

    @action(methods=["get"], detail=True)
    def archive(self, request, id=None):
        repo = self.get_object()
        ref = request.query_params.get("ref", repo.default_branch)
        fmt = request.query_params.get("format", "tar.gz")
        try:
            backend = self._get_backend(repo)
            data = backend.get_archive(ref, fmt)
            content_type = "application/gzip" if fmt == "tar.gz" else f"application/{fmt}"
            response = HttpResponse(data, content_type=content_type)
            safe_ref = ref.replace("/", "-")
            response["Content-Disposition"] = f'attachment; filename="{repo.name}-{safe_ref}.{fmt}"'
            return response
        except GitServiceError as e:
            return Response({"detail": str(e)}, status=404)

    def perform_destroy(self, instance):
        backend = GitBackend(instance.disk_path)
        if backend.exists():
            backend.delete()
        instance.delete()
