import logging

from django.conf import settings
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
    CodeOwnerRule,
    ProtectedBranch,
    Release,
    Repository,
    RepositoryAccess,
    validate_repository_component,
)

logger = logging.getLogger("mygit")


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
            "custom_disk_path",
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

    def validate_custom_disk_path(self, value):
        if not value or not value.strip():
            return None
        return value.strip()

    def create(self, validated_data):
        request = self.context["request"]
        owner_type = validated_data.get("owner_type", "user")
        owner_id = validated_data.get("owner_id", request.user.id)

        if owner_type == "organization":
            from apps.organizations.models import Group, GroupMember

            group = get_object_or_404(Group, id=owner_id)
            if (
                not request.user.is_superuser
                and not GroupMember.objects.filter(group=group, user=request.user).exists()
            ):
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


class ProtectedBranchSerializer(serializers.ModelSerializer):
    class Meta:
        model = ProtectedBranch
        fields = "__all__"
        read_only_fields = ["id", "repository", "created_at", "updated_at"]


class CodeOwnerRuleSerializer(serializers.ModelSerializer):
    owner_usernames = serializers.SerializerMethodField()

    class Meta:
        model = CodeOwnerRule
        fields = [
            "id",
            "repository",
            "pattern",
            "owners",
            "owner_usernames",
            "raw_owners",
            "created_at",
            "updated_at",
        ]
        read_only_fields = ["id", "repository", "owner_usernames", "created_at", "updated_at"]

    def get_owner_usernames(self, obj):
        return list(obj.owners.values_list("username", flat=True))


class ReleaseSerializer(serializers.ModelSerializer):
    created_by_username = serializers.CharField(
        source="created_by.username", read_only=True, allow_null=True
    )

    class Meta:
        model = Release
        fields = "__all__"
        read_only_fields = [
            "id",
            "repository",
            "changelog",
            "is_signed",
            "signature",
            "created_by",
            "created_at",
            "updated_at",
        ]


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

    @action(methods=["get"], detail=True, url_path="raw/(?P<sha>[^/]+)")
    def raw_blob_by_sha(self, request, id=None, sha=None):
        repo = self.get_object()
        try:
            backend = self._get_backend(repo)
            content, blob_sha = backend.get_blob(sha)
            from django.http import HttpResponse
            response = HttpResponse(content, content_type="text/plain; charset=utf-8")
            return response
        except (GitServiceError, KeyError) as e:
            return Response({"detail": str(e)}, status=404)

    @action(methods=["get"], detail=True, url_path="raw/(?P<ref>[^/]+)/(?P<path>.+)")
    def raw_blob_by_path(self, request, id=None, ref=None, path=None):
        repo = self.get_object()
        try:
            backend = self._get_backend(repo)
            content, blob_sha = backend.get_blob_by_path(ref, path)
            from django.http import HttpResponse
            response = HttpResponse(content, content_type="text/plain; charset=utf-8")
            return response
        except (GitServiceError, KeyError) as e:
            return Response({"detail": str(e)}, status=404)

    @action(methods=["get"], detail=True, url_path="raw")
    def raw_blob_by_path(self, request, id=None):
        repo = self.get_object()
        ref = request.query_params.get("ref", repo.default_branch)
        path = request.query_params.get("path", "")
        if not path:
            return Response({"detail": "path parameter is required."}, status=400)
        try:
            backend = self._get_backend(repo)
            content, blob_sha = backend.get_blob_by_path(ref, path)
            from django.http import HttpResponse
            response = HttpResponse(content, content_type="text/plain; charset=utf-8")
            return response
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
        protected = _matching_protected_branch(repo, name)
        if protected and not protected.allow_delete:
            return Response({"detail": "This branch is protected from deletion."}, status=403)
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

    @action(methods=["get", "post"], detail=True, url_path="protected-branches")
    def protected_branches(self, request, id=None):
        repo = self.get_object()
        if request.method == "GET":
            rules = repo.protected_branches.all()
            return Response(ProtectedBranchSerializer(rules, many=True).data)
        self._ensure_repo_role(repo, RepositoryAccess.Role.MAINTAINER)
        serializer = ProtectedBranchSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        rule = serializer.save(repository=repo)
        return Response(ProtectedBranchSerializer(rule).data, status=status.HTTP_201_CREATED)

    @action(
        methods=["patch", "delete"], detail=True, url_path="protected-branches/(?P<rule_id>[^/]+)"
    )
    def protected_branch_detail(self, request, id=None, rule_id=None):
        repo = self.get_object()
        self._ensure_repo_role(repo, RepositoryAccess.Role.MAINTAINER)
        rule = get_object_or_404(ProtectedBranch, repository=repo, id=rule_id)
        if request.method == "DELETE":
            rule.delete()
            return Response(status=status.HTTP_204_NO_CONTENT)
        serializer = ProtectedBranchSerializer(rule, data=request.data, partial=True)
        serializer.is_valid(raise_exception=True)
        serializer.save()
        return Response(ProtectedBranchSerializer(rule).data)

    @action(methods=["get", "post"], detail=True, url_path="codeowners")
    def codeowners(self, request, id=None):
        repo = self.get_object()
        if request.method == "GET":
            rules = repo.codeowner_rules.prefetch_related("owners")
            return Response(CodeOwnerRuleSerializer(rules, many=True).data)
        self._ensure_repo_role(repo, RepositoryAccess.Role.MAINTAINER)
        serializer = CodeOwnerRuleSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        rule = serializer.save(repository=repo)
        return Response(CodeOwnerRuleSerializer(rule).data, status=status.HTTP_201_CREATED)

    @action(methods=["get", "post"], detail=True, url_path="releases")
    def releases(self, request, id=None):
        repo = self.get_object()
        if request.method == "GET":
            releases = repo.releases.select_related("created_by")
            return Response(ReleaseSerializer(releases, many=True).data)
        self._ensure_repo_role(repo, RepositoryAccess.Role.MAINTAINER)
        serializer = ReleaseSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        data = serializer.validated_data
        changelog = _generate_changelog(repo, data["tag_name"])
        release = serializer.save(repository=repo, created_by=request.user, changelog=changelog)
        return Response(ReleaseSerializer(release).data, status=status.HTTP_201_CREATED)

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
            stream = backend.get_archive_stream(ref, fmt)
            content_type = "application/gzip" if fmt == "tar.gz" else f"application/{fmt}"
            response = HttpResponse(stream, content_type=content_type)
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
            integration_token = IntegrationToken.objects.filter(
                user=request.user, provider="github"
            ).first()
            if not integration_token:
                return Response(
                    {"detail": "GitHub token not configured."}, status=status.HTTP_400_BAD_REQUEST
                )
            token = integration_token.get_token()

        headers = {
            "Authorization": f"Bearer {token}",
            "Accept": "application/vnd.github+json",
        }
        try:
            resp = requests.get(
                "https://api.github.com/user/repos?per_page=100&sort=updated",
                headers=headers,
                timeout=10,
            )
            if not resp.ok:
                logger.error(
                    "GitHub API error for user %s: %s", request.user.username, resp.text[:200]
                )
                return Response({"detail": "Failed to fetch repositories from GitHub."}, status=502)

            repos = []
            for item in resp.json():
                repos.append(
                    {
                        "name": item["name"],
                        "full_name": item["full_name"],
                        "description": item.get("description") or "",
                        "private": item["private"],
                        "clone_url": item["clone_url"],
                        "default_branch": item.get("default_branch") or "main",
                    }
                )
            return Response(repos)
        except Exception as e:
            return Response({"detail": str(e)}, status=status.HTTP_500_INTERNAL_SERVER_ERROR)

    @action(methods=["get"], detail=False, url_path="import/gitlab/repos")
    def gitlab_repos(self, request):
        from apps.accounts.models import IntegrationToken
        import requests

        token = request.query_params.get("token")
        if not token:
            integration_token = IntegrationToken.objects.filter(
                user=request.user, provider="gitlab"
            ).first()
            if not integration_token:
                return Response(
                    {"detail": "GitLab token not configured."}, status=status.HTTP_400_BAD_REQUEST
                )
            token = integration_token.get_token()

        headers = {
            "PRIVATE-TOKEN": token,
        }
        try:
            resp = requests.get(
                "https://gitlab.com/api/v4/projects?membership=true&simple=true&per_page=100&order_by=updated_at",
                headers=headers,
                timeout=10,
            )
            if not resp.ok:
                logger.error(
                    "GitLab API error for user %s: %s", request.user.username, resp.text[:200]
                )
                return Response({"detail": "Failed to fetch repositories from GitLab."}, status=502)

            repos = []
            for item in resp.json():
                repos.append(
                    {
                        "name": item["name"],
                        "full_name": item["path_with_namespace"],
                        "description": item.get("description") or "",
                        "private": item["visibility"] == "private",
                        "clone_url": item["http_url_to_repo"],
                        "default_branch": item.get("default_branch") or "main",
                    }
                )
            return Response(repos)
        except Exception as e:
            return Response({"detail": str(e)}, status=status.HTTP_500_INTERNAL_SERVER_ERROR)

    @action(methods=["post"], detail=False, url_path="import")
    def import_project(self, request):
        from apps.accounts.models import IntegrationToken
        import os
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
            return Response(
                {"detail": "Project name is required."}, status=status.HTTP_400_BAD_REQUEST
            )

        try:
            validate_repository_component(name)
        except Exception as e:
            return Response({"detail": str(e)}, status=status.HTTP_400_BAD_REQUEST)

        owner_type = request.data.get("owner_type", "user")
        owner_id = request.data.get("owner_id")

        if owner_type == "organization" and owner_id:
            from apps.organizations.models import Group, GroupMember

            group = get_object_or_404(Group, id=owner_id)
            if (
                not request.user.is_superuser
                and not GroupMember.objects.filter(group=group, user=request.user).exists()
            ):
                return Response(
                    {"detail": "You do not have access to this group."},
                    status=status.HTTP_403_FORBIDDEN,
                )
            owner_path = group.path
        else:
            owner_type = "user"
            owner_id = request.user.id
            owner_path = request.user.username

        path = f"{owner_path}/{name}"
        if Repository.objects.filter(path=path).exists():
            return Response(
                {"detail": "Repository already exists."}, status=status.HTTP_409_CONFLICT
            )

        if provider in ["github", "gitlab"] and not token:
            integration_token = IntegrationToken.objects.filter(
                user=request.user, provider=provider
            ).first()
            if integration_token:
                token = integration_token.get_token()

        if provider == "github":
            if not repo_name:
                return Response(
                    {"detail": "repo_name is required for github provider."},
                    status=status.HTTP_400_BAD_REQUEST,
                )
            if token:
                actual_clone_url = f"https://{token}@github.com/{repo_name}.git"
            else:
                actual_clone_url = f"https://github.com/{repo_name}.git"
        elif provider == "gitlab":
            if not repo_name:
                return Response(
                    {"detail": "repo_name is required for gitlab provider."},
                    status=status.HTTP_400_BAD_REQUEST,
                )
            if token:
                actual_clone_url = f"https://oauth2:{token}@gitlab.com/{repo_name}.git"
            else:
                actual_clone_url = f"https://gitlab.com/{repo_name}.git"
        elif provider == "custom":
            if not clone_url:
                return Response(
                    {"detail": "clone_url is required for custom provider."},
                    status=status.HTTP_400_BAD_REQUEST,
                )
            actual_clone_url = clone_url
        else:
            return Response({"detail": "Invalid provider."}, status=status.HTTP_400_BAD_REQUEST)

        custom_disk_path = request.data.get("custom_disk_path")
        if custom_disk_path and custom_disk_path.strip():
            custom_disk_path = custom_disk_path.strip()
        else:
            custom_disk_path = None

        repo = Repository.objects.create(
            owner_type=owner_type,
            owner_id=owner_id,
            name=name,
            path=path,
            description=description,
            visibility=visibility,
            default_branch="main",
            custom_disk_path=custom_disk_path,
        )

        disk_path = repo.disk_path
        disk_path.parent.mkdir(parents=True, exist_ok=True)

        cmd = ["git", "clone", "--bare", actual_clone_url, str(disk_path)]
        try:
            env = os.environ.copy()
            if token and provider == "github":
                env["GIT_ASKPASS"] = ""
                env["GIT_USERNAME"] = token
            elif token and provider == "gitlab":
                env["GIT_ASKPASS"] = ""
                env["GIT_USERNAME"] = "oauth2"
                env["GIT_PASSWORD"] = token
            res = subprocess.run(cmd, capture_output=True, text=True, timeout=90, env=env)
            if res.returncode != 0:
                if disk_path.exists():
                    shutil.rmtree(disk_path)
                repo.delete()
                err_msg = res.stderr
                if token:
                    err_msg = err_msg.replace(token, "********")
                return Response(
                    {"detail": f"Clone failed: {err_msg}"}, status=status.HTTP_400_BAD_REQUEST
                )
        except subprocess.TimeoutExpired:
            if disk_path.exists():
                shutil.rmtree(disk_path)
            repo.delete()
            return Response(
                {"detail": "Clone timed out after 90 seconds."},
                status=status.HTTP_408_REQUEST_TIMEOUT,
            )
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

    @action(methods=["get"], detail=False, url_path="browse-disk")
    def browse_disk(self, request):
        if not request.user.is_superuser:
            return Response({"detail": "Permission denied."}, status=403)

        target_path = request.query_params.get("path", "")
        import os
        from pathlib import Path

        allowed_root = os.path.realpath(getattr(settings, "MYGIT_REPOS_ROOT", ""))
        directories = []
        parent_path = None
        current_path = ""

        try:
            if os.name == "nt":
                if not target_path:
                    import string
                    import ctypes

                    bitmask = ctypes.windll.kernel32.GetLogicalDrives()
                    for letter in string.ascii_uppercase:
                        if bitmask & 1:
                            directories.append(f"{letter}:\\")
                        bitmask >>= 1
                    current_path = ""
                    parent_path = None
                else:
                    path_obj = Path(target_path).resolve(strict=True)
                    if not str(path_obj).startswith(allowed_root):
                        return Response({"detail": "Path outside allowed root."}, status=403)
                    current_path = str(path_obj)
                    if path_obj.parent != path_obj:
                        parent_path = str(path_obj.parent)

                    for entry in os.scandir(path_obj):
                        try:
                            if entry.is_dir():
                                directories.append(entry.name)
                        except OSError:
                            pass
                    directories.sort(key=str.lower)
            else:
                if not target_path:
                    path_obj = Path(allowed_root) if allowed_root else Path("/")
                else:
                    path_obj = Path(target_path).resolve(strict=True)
                    if allowed_root and not str(path_obj).startswith(allowed_root):
                        return Response({"detail": "Path outside allowed root."}, status=403)

                current_path = str(path_obj)
                if path_obj != Path("/"):
                    parent_path = str(path_obj.parent)

                for entry in os.scandir(path_obj):
                    try:
                        if entry.is_dir():
                            directories.append(entry.name)
                    except OSError:
                        pass
                directories.sort(key=str.lower)

            return Response(
                {
                    "current_path": current_path,
                    "parent_path": parent_path,
                    "directories": directories,
                }
            )
        except Exception as e:
            return Response({"detail": str(e)}, status=status.HTTP_400_BAD_REQUEST)

    @action(methods=["post"], detail=False, url_path="create-disk-folder")
    def create_disk_folder(self, request):
        if not request.user.is_superuser:
            return Response({"detail": "Permission denied."}, status=403)

        parent_path = request.data.get("parent_path")
        name = request.data.get("name")
        if not parent_path or not name:
            return Response(
                {"detail": "parent_path and name are required."}, status=status.HTTP_400_BAD_REQUEST
            )

        from pathlib import Path
        import os

        allowed_root = os.path.realpath(getattr(settings, "MYGIT_REPOS_ROOT", ""))
        try:
            target = (Path(parent_path) / name).resolve(strict=False)
            if allowed_root and not str(target).startswith(allowed_root):
                return Response({"detail": "Path outside allowed root."}, status=403)
            target.mkdir(parents=True, exist_ok=True)
            return Response({"path": str(target)})
        except Exception as e:
            return Response({"detail": str(e)}, status=status.HTTP_400_BAD_REQUEST)


def _matching_protected_branch(repo: Repository, branch: str) -> ProtectedBranch | None:
    for rule in repo.protected_branches.all():
        if rule.matches(branch):
            return rule
    return None


def _generate_changelog(repo: Repository, tag_name: str) -> str:
    try:
        backend = GitBackend(repo.disk_path)
        commits = backend.get_commits(repo.default_branch, page=1, per_page=50)
    except Exception:
        return ""
    lines = [f"Changes for {tag_name}", ""]
    for commit in commits:
        lines.append(f"- {commit['short_sha']} {commit['message'].splitlines()[0]}")
    return "\n".join(lines)
