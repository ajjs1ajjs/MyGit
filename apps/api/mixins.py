from django.http import Http404
from rest_framework.exceptions import PermissionDenied

from apps.repositories.models import Repository, RepositoryAccess


class RepoAccessMixin:
    def _get_repo(self):
        repo_id = self.kwargs.get("project_id", "")
        try:
            repo = Repository.objects.get(id=repo_id)
        except (Repository.DoesNotExist, ValueError):
            raise Http404("Repository not found.") from None

        user = self.request.user
        if repo.visibility == Repository.Visibility.PUBLIC:
            return repo
        if str(repo.owner_id) == str(user.id):
            return repo
        if not RepositoryAccess.objects.filter(
            repository=repo,
            user=user,
            role__gte=RepositoryAccess.Role.GUEST,
        ).exists():
            raise PermissionDenied("You do not have access to this repository.")

        return repo
