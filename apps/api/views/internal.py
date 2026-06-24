import logging
from functools import wraps

from django.conf import settings
from django.contrib.auth import get_user_model
from django.http import JsonResponse
from django.views.decorators.csrf import csrf_exempt
from django.views.decorators.http import require_GET, require_POST

from apps.accounts.models import SSHKey
from apps.repositories.models import Repository, RepositoryAccess

logger = logging.getLogger("mygit")
User = get_user_model()


def require_internal_token(view_func):
    @wraps(view_func)
    def wrapper(request, *args, **kwargs):
        token = request.META.get("HTTP_AUTHORIZATION", "")
        expected = getattr(settings, "MYGIT_INTERNAL_API_TOKEN", "")
        if not expected:
            return view_func(request, *args, **kwargs)
        if token != f"Bearer {expected}":
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
