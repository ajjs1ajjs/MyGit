from django.http import Http404
from rest_framework.exceptions import PermissionDenied

from apps.repositories.models import Repository, RepositoryAccess


def ensure_repo_access(
    repo: Repository,
    user,
    min_role: int = RepositoryAccess.Role.GUEST,
    *,
    allow_public_read: bool = True,
) -> Repository:
    if (
        allow_public_read
        and min_role <= RepositoryAccess.Role.GUEST
        and repo.visibility == Repository.Visibility.PUBLIC
    ):
        return repo

    if user and getattr(user, "is_superuser", False):
        return repo

    if user and str(repo.owner_id) == str(user.id):
        return repo

    if not RepositoryAccess.objects.filter(
        repository=repo,
        user=user,
        role__gte=min_role,
    ).exists():
        raise PermissionDenied("You do not have access to this repository.")

    return repo


class RepoAccessMixin:
    def _get_repo(self):
        repo_id = self.kwargs.get("project_id", "")
        try:
            repo = Repository.objects.get(id=repo_id)
        except (Repository.DoesNotExist, ValueError):
            raise Http404("Repository not found.") from None

        return ensure_repo_access(repo, self.request.user)
