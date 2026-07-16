import logging
from functools import wraps

from django.conf import settings
from django.contrib.auth import get_user_model
from django.http import JsonResponse
from django.utils.crypto import constant_time_compare
from django.views.decorators.csrf import csrf_exempt
from django.views.decorators.http import require_GET, require_POST

from apps.accounts.models import SSHKey
from apps.repositories.models import ProtectedBranch, Repository, RepositoryAccess

logger = logging.getLogger("mygit")
User = get_user_model()


def require_internal_token(view_func):
    @wraps(view_func)
    def wrapper(request, *args, **kwargs):
        token = request.META.get("HTTP_AUTHORIZATION", "")
        expected = getattr(settings, "MYGIT_INTERNAL_API_TOKEN", "")
        if not expected:
            logger.critical("MYGIT_INTERNAL_API_TOKEN is not configured! SSH hooks will not work.")
            return JsonResponse({"detail": "Internal API token is not configured."}, status=503)
        if not constant_time_compare(token, f"Bearer {expected}"):
            return JsonResponse({"detail": "Forbidden."}, status=403)
        return view_func(request, *args, **kwargs)

    return wrapper


@require_GET
@require_internal_token
def authorized_keys(request):
    username = request.GET.get("username", "")
    try:
        user = User.objects.get(username=username, is_active=True)
    except User.DoesNotExist:
        return JsonResponse({"keys": []})

    keys = SSHKey.objects.filter(user=user).values("id", "public_key")
    return JsonResponse({"keys": list(keys)})


@csrf_exempt
@require_POST
@require_internal_token
def check_access(request):
    import json

    try:
        data = json.loads(request.body)
    except json.JSONDecodeError:
        data = request.POST.dict()

    key_id = data.get("key_id", "")
    repo_path = data.get("repo_path", "")
    action = data.get("action", "")

    if action not in {"pull", "push"}:
        return JsonResponse({"detail": "Invalid action."}, status=400)

    try:
        key = SSHKey.objects.get(id=key_id)
    except (SSHKey.DoesNotExist, ValueError):
        return JsonResponse({"detail": "Invalid key."}, status=403)

    if repo_path.endswith(".git"):
        repo_path = repo_path[:-4]

    try:
        repo = Repository.objects.get(path=repo_path, is_archived=False)
    except Repository.DoesNotExist:
        return JsonResponse({"detail": "Repository not found."}, status=404)

    if action == "pull" and repo.visibility == Repository.Visibility.PUBLIC:
        return JsonResponse({"detail": "Access granted."})

    if str(repo.owner_id) == str(key.user.id):
        return JsonResponse({"detail": "Access granted."})

    access = RepositoryAccess.objects.filter(repository=repo, user=key.user).first()

    if not access:
        return JsonResponse({"detail": "Access denied."}, status=403)

    required_role = (
        RepositoryAccess.Role.DEVELOPER if action == "push" else RepositoryAccess.Role.GUEST
    )
    if access.role < required_role:
        return JsonResponse({"detail": "Access denied."}, status=403)

    return JsonResponse({"detail": "Access granted."})


@csrf_exempt
@require_POST
@require_internal_token
def pre_receive(request):
    import json

    try:
        data = json.loads(request.body)
    except json.JSONDecodeError:
        return JsonResponse({"detail": "Invalid JSON."}, status=400)

    repo_path = data.get("repo_path", "")
    ref = data.get("ref", "")
    old_rev = data.get("old_rev", "0" * 40)
    new_rev = data.get("new_rev", "0" * 40)

    if repo_path.endswith(".git"):
        repo_path = repo_path[:-4]

    if ref.startswith("refs/heads/"):
        branch = ref[11:]
        try:
            repo = Repository.objects.get(path=repo_path, is_archived=False)
        except Repository.DoesNotExist:
            return JsonResponse({"detail": "Repository not found."}, status=404)

        if repo.is_archived:
            msg = "ERROR: Cannot push to an archived repository."
            return JsonResponse({"detail": msg}, status=403)

        if branch == repo.default_branch and new_rev == "0" * 40:
            msg = f"ERROR: Cannot delete the default branch '{branch}'."
            return JsonResponse({"detail": msg}, status=403)

        protected = _matching_protected_branch(repo, branch)
        if protected:
            is_delete = new_rev == "0" * 40
            is_force_push = old_rev != "0" * 40 and new_rev != "0" * 40
            if is_delete and not protected.allow_delete:
                return JsonResponse({"detail": "ERROR: Protected branch cannot be deleted."}, status=403)
            if is_force_push and not protected.allow_force_push:
                return JsonResponse({"detail": "ERROR: Force push is disabled for this branch."}, status=403)
            if not protected.allow_direct_push:
                return JsonResponse(
                    {"detail": "ERROR: Direct push is disabled for this protected branch."},
                    status=403,
                )

    logger.info(
        "pre-receive: repo=%s ref=%s old=%s new=%s",
        repo_path,
        ref,
        old_rev[:8],
        new_rev[:8],
    )
    return JsonResponse({"detail": "OK"})


@csrf_exempt
@require_POST
@require_internal_token
def post_receive(request):
    import json

    try:
        data = json.loads(request.body)
    except json.JSONDecodeError:
        return JsonResponse({"detail": "Invalid JSON."}, status=400)

    repo_path = data.get("repo_path", "")
    ref = data.get("ref", "")
    old_rev = data.get("old_rev", "0" * 40)
    new_rev = data.get("new_rev", "0" * 40)

    if repo_path.endswith(".git"):
        repo_path = repo_path[:-4]

    try:
        repo = Repository.objects.get(path=repo_path)
        from apps.git_service.backend import GitBackend

        repo.size_kb = GitBackend.get_repo_size_kb(repo.disk_path)
        repo.save(update_fields=["size_kb", "updated_at"])
    except Repository.DoesNotExist:
        pass

    logger.info(
        "post-receive: repo=%s ref=%s old=%s new=%s",
        repo_path,
        ref,
        old_rev[:8],
        new_rev[:8],
    )
    return JsonResponse({"detail": "OK"})


def _matching_protected_branch(repo: Repository, branch: str) -> ProtectedBranch | None:
    for rule in repo.protected_branches.all():
        if rule.matches(branch):
            return rule
    return None
