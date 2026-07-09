import base64
import logging
import time

from django.contrib.auth import get_user_model
from django.core.cache import cache
from django.http import (
    HttpResponse,
    HttpResponseBadRequest,
    HttpResponseForbidden,
)
from django.shortcuts import get_object_or_404
from django.views.decorators.csrf import csrf_exempt
from django.views.decorators.http import require_http_methods

from apps.git_service.backend import GitBackend, GitServiceError
from apps.repositories.models import Repository, RepositoryAccess

logger = logging.getLogger("mygit")
User = get_user_model()


def _rate_limit(request, key_prefix: str, max_requests: int = 30, window: int = 60) -> bool:
    ip = request.META.get("REMOTE_ADDR", "unknown")
    cache_key = f"git_rate:{key_prefix}:{ip}"
    now = time.time()
    hits = cache.get(cache_key, [])
    hits = [t for t in hits if t > now - window]
    if len(hits) >= max_requests:
        return False
    hits.append(now)
    cache.set(cache_key, hits, window)
    return True


def _authenticate_user_with_password(login: str, password: str):
    try:
        user = User.objects.get(username=login)
    except User.DoesNotExist:
        try:
            user = User.objects.get(email=login)
        except User.DoesNotExist:
            return None
    if user.check_password(password):
        return user
    if _check_pat_token(user, password):
        return user
    return None


def _check_pat_token(user, token_string: str) -> bool:
    import hashlib

    from django.utils import timezone

    from apps.accounts.models import PersonalAccessToken

    token_hash = hashlib.sha256(token_string.encode()).hexdigest()
    try:
        pat = PersonalAccessToken.objects.get(user=user, token_hash=token_hash)
        if pat.is_expired:
            return False
        pat.last_used_at = timezone.now()
        pat.save(update_fields=["last_used_at"])
        return True
    except PersonalAccessToken.DoesNotExist:
        return False


def _authenticate_git_http(request):
    auth_header = request.META.get("HTTP_AUTHORIZATION", "")
    if auth_header.startswith("Basic "):
        try:
            decoded = base64.b64decode(auth_header[6:]).decode("utf-8")
            username, password = decoded.split(":", 1)
            user = _authenticate_user_with_password(username, password)
            if user and user.is_active:
                return user
        except Exception:
            pass

    if auth_header.startswith("Bearer "):
        from rest_framework_simplejwt.authentication import JWTAuthentication

        jwt_auth = JWTAuthentication()
        try:
            validated_token = jwt_auth.get_validated_token(auth_header[7:])
            user = jwt_auth.get_user(validated_token)
            if user and user.is_active:
                return user
        except Exception:
            pass

    return None


def _check_repo_access(repo: Repository, user, service: str) -> bool:
    is_read = service == "git-upload-pack"
    is_write = service == "git-receive-pack"

    if repo.visibility == Repository.Visibility.PUBLIC and is_read:
        return True

    if not user or not user.is_active:
        return False

    if str(repo.owner_id) == str(user.id):
        return True

    access = RepositoryAccess.objects.filter(repository=repo, user=user).first()

    if not access:
        return False

    if is_read and access.role >= RepositoryAccess.Role.GUEST:
        return True
    return is_write and access.role >= RepositoryAccess.Role.DEVELOPER


@csrf_exempt
@require_http_methods(["GET", "POST"])
def git_info_refs(request, owner: str, repo_name: str):
    if not _rate_limit(request, "info_refs", max_requests=60):
        return HttpResponse("Rate limit exceeded.", status=429)

    repo = get_object_or_404(Repository, path=f"{owner}/{repo_name}", is_archived=False)
    service = request.GET.get("service", "")

    if service not in ("git-upload-pack", "git-receive-pack"):
        return HttpResponseBadRequest("Invalid git service.")

    user = _authenticate_git_http(request)

    if not _check_repo_access(repo, user, service):
        if not user:
            response = HttpResponse("Authentication required", status=401)
            response["WWW-Authenticate"] = 'Basic realm="MyGit"'
            return response
        return HttpResponseForbidden("Access denied.")

    backend = GitBackend(repo.disk_path)
    if not backend.exists():
        return HttpResponse("Repository not found.", status=404)

    try:
        output = backend.handle_smart_http(service)
        pkt_line = f"# service={service}\n"
        pkt_len = format(len(pkt_line) + 4, "04x")
        header = (pkt_len + pkt_line + "0000").encode()
        content_type = f"application/x-{service}-advertisement"
        response = HttpResponse(header + output, content_type=content_type)
        response["Cache-Control"] = "no-cache"
        return response
    except GitServiceError as e:
        logger.error(f"Git info_refs error for {repo.path}: {e}")
        return HttpResponse(str(e), status=500)


@csrf_exempt
@require_http_methods(["POST"])
def git_rpc(request, owner: str, repo_name: str):
    if not _rate_limit(request, "rpc", max_requests=120):
        return HttpResponse("Rate limit exceeded.", status=429)

    path_info = request.path
    service = None

    if "/git-upload-pack" in path_info:
        service = "git-upload-pack"
    elif "/git-receive-pack" in path_info:
        service = "git-receive-pack"
    else:
        return HttpResponseBadRequest("Invalid git service.")

    repo = get_object_or_404(Repository, path=f"{owner}/{repo_name}", is_archived=False)

    user = _authenticate_git_http(request)

    if not _check_repo_access(repo, user, service):
        if not user:
            response = HttpResponse("Authentication required", status=401)
            response["WWW-Authenticate"] = 'Basic realm="MyGit"'
            return response
        return HttpResponseForbidden("Access denied.")

    backend = GitBackend(repo.disk_path)
    if not backend.exists():
        return HttpResponse("Repository not found.", status=404)

    try:
        input_data = request.body
        output = backend.handle_smart_http(service, input_stream=input_data)
        content_type = f"application/x-{service}-result"
        response = HttpResponse(output, content_type=content_type)
        response["Cache-Control"] = "no-cache"
        return response
    except GitServiceError as e:
        logger.error(f"Git RPC error for {repo.path}: {e}")
        return HttpResponse(str(e), status=500)
