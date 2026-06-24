from django.db.models import Q
from django.http import HttpResponse
from django.shortcuts import get_object_or_404
from rest_framework import serializers, status, viewsets
from rest_framework.decorators import action
from rest_framework.permissions import IsAuthenticated
from rest_framework.response import Response

from apps.api.mixins import ensure_repo_access
from apps.git_service.backend import GitBackend, GitServiceError
from apps.repositories.models import (
    Repository,
    RepositoryAccess,
    validate_repository_component,
)


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
            "owner_type",
            "owner_id",
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
        owner_type = validated_data.get("owner_type", "user")
        owner_id = validated_data.get("owner_id", request.user.id)

        if owner_type == "organization":
            from apps.organizations.models import Group, GroupMember
            group = get_object_or_404(Group, id=owner_id)
            if not request.user.is_superuser and not GroupMember.objects.filter(group=group, user=request.user).exists():
                raise serializers.ValidationError("You do not have access to this group.")
            owner_path = group.path
        else:
            owner_type = "user"
            owner_id = request.user.id
            owner_path = request.user.username

        validated_data["owner_type"] = owner_type
        validated_data["owner_id"] = owner_id
        validated_data["path"] = f"{owner_path}/{validated_data['name']}"

        repo = Repository.objects.create(**validated_data)

        GitBackend(repo.disk_path).init_bare()
        repo.size_kb = GitBackend.get_repo_size_kb(repo.disk_path)
        repo.save(update_fields=["size_kb"])

        RepositoryAccess.objects.create(
            user=request.user, repository=repo, role=RepositoryAccess.Role.OWNER
        )

        return repo

    def validate_name(self, value):
        try:
            return validate_repository_component(value)
        except Exception as e:
            raise serializers.ValidationError(str(e)) from e


class ProjectViewSet(viewsets.ModelViewSet):
    serializer_class = RepositorySerializer
    permission_classes = [IsAuthenticated]
    lookup_field = "id"

    def get_queryset(self):
        user = self.request.user
        if user and getattr(user, "is_superuser", False):
            return Repository.objects.all().order_by("-updated_at")
        return (
            Repository.objects.filter(
                Q(visibility=Repository.Visibility.PUBLIC)
                | Q(owner_id=user.id)
                | Q(access_list__user=user, access_list__role__gte=RepositoryAccess.Role.GUEST)
            )
            .distinct()
            .order_by("-updated_at")
        )

    def _ensure_repo_role(self, repo: Repository, role: int) -> None:
        ensure_repo_access(repo, self.request.user, min_role=role, allow_public_read=False)

    def perform_update(self, serializer):
        self._ensure_repo_role(self.get_object(), RepositoryAccess.Role.MAINTAINER)
        serializer.save()

    @action(methods=["post"], detail=True)
    def fork(self, request, id=None):
        repo = self.get_object()
        new_name = request.data.get("name", f"{repo.name}-fork")
        try:
            validate_repository_component(new_name)
        except Exception as e:
            return Response({"detail": str(e)}, status=status.HTTP_400_BAD_REQUEST)
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
        import shutil

        if repo.disk_path.exists():
            shutil.copytree(str(repo.disk_path), str(new_repo.disk_path))
            new_repo.size_kb = GitBackend.get_repo_size_kb(new_repo.disk_path)
            new_repo.save(update_fields=["size_kb"])

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

        if request.method == "GET":
            try:
                branches = self._get_backend(repo).get_branches()
                return Response({"branches": branches})
            except GitServiceError as e:
                return Response({"detail": str(e)}, status=404)

        if request.method == "POST":
            self._ensure_repo_role(repo, RepositoryAccess.Role.DEVELOPER)
            name = request.data.get("name", "")
            ref = request.data.get("ref", repo.default_branch)
            if not name:
                return Response({"detail": "Branch name is required."}, status=400)
            try:
                backend = self._get_backend(repo)
                result = backend.create_branch(name, ref)
                return Response(result, status=status.HTTP_201_CREATED)
            except Exception as e:
                return Response({"detail": str(e)}, status=400)

        return Response({"detail": "Method not allowed."}, status=405)

    @action(methods=["delete"], detail=True, url_path="branches/(?P<name>[^/]+)")
    def delete_branch(self, request, id=None, name=None):
        repo = self.get_object()
        self._ensure_repo_role(repo, RepositoryAccess.Role.DEVELOPER)
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

        if request.method == "GET":
            try:
                tag_list = self._get_backend(repo).get_tags()
                return Response({"tags": tag_list})
            except GitServiceError as e:
                return Response({"detail": str(e)}, status=404)

        if request.method == "POST":
            self._ensure_repo_role(repo, RepositoryAccess.Role.DEVELOPER)
            name = request.data.get("name", "")
            ref = request.data.get("ref", repo.default_branch)
            msg = request.data.get("message", "")
            if not name:
                return Response({"detail": "Tag name is required."}, status=400)
            try:
                backend = self._get_backend(repo)
                result = backend.create_tag(name, ref, message=msg)
                return Response(result, status=status.HTTP_201_CREATED)
            except Exception as e:
                return Response({"detail": str(e)}, status=400)

        return Response({"detail": "Method not allowed."}, status=405)

    @action(methods=["delete"], detail=True, url_path="tags/(?P<name>[^/]+)")
    def delete_tag(self, request, id=None, name=None):
        repo = self.get_object()
        self._ensure_repo_role(repo, RepositoryAccess.Role.DEVELOPER)
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
            import re

            safe_name = re.sub(r"[^\w\-.]", "_", repo.name)
            safe_ref = re.sub(r"[^\w\-.]", "_", ref.replace("/", "-"))
            filename = f"{safe_name}-{safe_ref}.{fmt}"
            response["Content-Disposition"] = f'attachment; filename="{filename}"'
            return response
        except GitServiceError as e:
            return Response({"detail": str(e)}, status=404)

    @action(methods=["get"], detail=False, url_path="by-path/(?P<repo_path>[^/]+/[^/]+)")
    def by_path(self, request, repo_path=None):
        repo = get_object_or_404(self.get_queryset(), path=repo_path)
        return Response(RepositorySerializer(repo).data)

    @action(methods=["get"], detail=False, url_path="import/github/repos")
    def github_repos(self, request):
        from apps.accounts.models import IntegrationToken
        import requests

        token = request.query_params.get("token")
        if not token:
            integration_token = IntegrationToken.objects.filter(user=request.user, provider="github").first()
            if not integration_token:
                return Response({"detail": "GitHub token not configured."}, status=status.HTTP_400_BAD_REQUEST)
            token = integration_token.token

        headers = {
            "Authorization": f"Bearer {token}",
            "Accept": "application/vnd.github+json",
        }
        try:
            resp = requests.get("https://api.github.com/user/repos?per_page=100&sort=updated", headers=headers, timeout=10)
            if not resp.ok:
                return Response({"detail": f"GitHub API error: {resp.text}"}, status=resp.status_code)

            repos = []
            for item in resp.json():
                repos.append({
                    "name": item["name"],
                    "full_name": item["full_name"],
                    "description": item.get("description") or "",
                    "private": item["private"],
                    "clone_url": item["clone_url"],
                    "default_branch": item.get("default_branch") or "main",
                })
            return Response(repos)
        except Exception as e:
            return Response({"detail": str(e)}, status=status.HTTP_500_INTERNAL_SERVER_ERROR)

    @action(methods=["get"], detail=False, url_path="import/gitlab/repos")
    def gitlab_repos(self, request):
        from apps.accounts.models import IntegrationToken
        import requests

        token = request.query_params.get("token")
        if not token:
            integration_token = IntegrationToken.objects.filter(user=request.user, provider="gitlab").first()
            if not integration_token:
                return Response({"detail": "GitLab token not configured."}, status=status.HTTP_400_BAD_REQUEST)
            token = integration_token.token

        headers = {
            "PRIVATE-TOKEN": token,
        }
        try:
            resp = requests.get("https://gitlab.com/api/v4/projects?membership=true&simple=true&per_page=100&order_by=updated_at", headers=headers, timeout=10)
            if not resp.ok:
                return Response({"detail": f"GitLab API error: {resp.text}"}, status=resp.status_code)

            repos = []
            for item in resp.json():
                repos.append({
                    "name": item["name"],
                    "full_name": item["path_with_namespace"],
                    "description": item.get("description") or "",
                    "private": item["visibility"] == "private",
                    "clone_url": item["http_url_to_repo"],
                    "default_branch": item.get("default_branch") or "main",
                })
            return Response(repos)
        except Exception as e:
            return Response({"detail": str(e)}, status=status.HTTP_500_INTERNAL_SERVER_ERROR)

    @action(methods=["post"], detail=False, url_path="import")
    def import_project(self, request):
        from apps.accounts.models import IntegrationToken
        import subprocess
        import shutil
        from apps.git_service.hooks import install_hooks

        provider = request.data.get("provider")  # "github", "gitlab", or "custom"
        repo_name = request.data.get("repo_name")
        clone_url = request.data.get("clone_url")
        name = request.data.get("name")
        visibility = request.data.get("visibility", "private")
        description = request.data.get("description", "")
        token = request.data.get("token")

        if not name:
            return Response({"detail": "Project name is required."}, status=status.HTTP_400_BAD_REQUEST)

        try:
            validate_repository_component(name)
        except Exception as e:
            return Response({"detail": str(e)}, status=status.HTTP_400_BAD_REQUEST)

        owner_type = request.data.get("owner_type", "user")
        owner_id = request.data.get("owner_id")

        if owner_type == "organization" and owner_id:
            from apps.organizations.models import Group, GroupMember
            group = get_object_or_404(Group, id=owner_id)
            if not request.user.is_superuser and not GroupMember.objects.filter(group=group, user=request.user).exists():
                return Response({"detail": "You do not have access to this group."}, status=status.HTTP_403_FORBIDDEN)
            owner_path = group.path
        else:
            owner_type = "user"
            owner_id = request.user.id
            owner_path = request.user.username

        path = f"{owner_path}/{name}"
        if Repository.objects.filter(path=path).exists():
            return Response({"detail": "Repository already exists."}, status=status.HTTP_409_CONFLICT)

        if provider in ["github", "gitlab"] and not token:
            integration_token = IntegrationToken.objects.filter(user=request.user, provider=provider).first()
            if integration_token:
                token = integration_token.token

        if provider == "github":
            if not repo_name:
                return Response({"detail": "repo_name is required for github provider."}, status=status.HTTP_400_BAD_REQUEST)
            if token:
                actual_clone_url = f"https://{token}@github.com/{repo_name}.git"
            else:
                actual_clone_url = f"https://github.com/{repo_name}.git"
        elif provider == "gitlab":
            if not repo_name:
                return Response({"detail": "repo_name is required for gitlab provider."}, status=status.HTTP_400_BAD_REQUEST)
            if token:
                actual_clone_url = f"https://oauth2:{token}@gitlab.com/{repo_name}.git"
            else:
                actual_clone_url = f"https://gitlab.com/{repo_name}.git"
        elif provider == "custom":
            if not clone_url:
                return Response({"detail": "clone_url is required for custom provider."}, status=status.HTTP_400_BAD_REQUEST)
            actual_clone_url = clone_url
        else:
            return Response({"detail": "Invalid provider."}, status=status.HTTP_400_BAD_REQUEST)

        repo = Repository.objects.create(
            owner_type=owner_type,
            owner_id=owner_id,
            name=name,
            path=path,
            description=description,
            visibility=visibility,
            default_branch="main",
        )

        disk_path = repo.disk_path
        disk_path.parent.mkdir(parents=True, exist_ok=True)

        cmd = ["git", "clone", "--bare", actual_clone_url, str(disk_path)]
        try:
            res = subprocess.run(cmd, capture_output=True, text=True, timeout=90)
            if res.returncode != 0:
                if disk_path.exists():
                    shutil.rmtree(disk_path)
                repo.delete()
                err_msg = res.stderr
                if token:
                    err_msg = err_msg.replace(token, "********")
                return Response({"detail": f"Clone failed: {err_msg}"}, status=status.HTTP_400_BAD_REQUEST)
        except subprocess.TimeoutExpired:
            if disk_path.exists():
                shutil.rmtree(disk_path)
            repo.delete()
            return Response({"detail": "Clone timed out after 90 seconds."}, status=status.HTTP_408_REQUEST_TIMEOUT)
        except Exception as e:
            if disk_path.exists():
                shutil.rmtree(disk_path)
            repo.delete()
            return Response({"detail": str(e)}, status=status.HTTP_500_INTERNAL_SERVER_ERROR)

        try:
            install_hooks(disk_path)
            backend = GitBackend(disk_path)
            if backend.exists():
                repo.default_branch = backend.get_default_branch()
                repo.size_kb = GitBackend.get_repo_size_kb(disk_path)
                repo.save(update_fields=["default_branch", "size_kb"])
        except Exception:
            pass

        RepositoryAccess.objects.get_or_create(
            user=request.user, repository=repo, role=RepositoryAccess.Role.OWNER
        )

        return Response(RepositorySerializer(repo).data, status=status.HTTP_201_CREATED)

    def perform_destroy(self, instance):
        self._ensure_repo_role(instance, RepositoryAccess.Role.OWNER)
        backend = GitBackend(instance.disk_path)
        if backend.exists():
            backend.delete()
        instance.delete()
