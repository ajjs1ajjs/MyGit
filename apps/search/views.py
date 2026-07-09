from django.contrib.auth import get_user_model
from django.db.models import Q
from rest_framework.decorators import api_view, permission_classes
from rest_framework.permissions import IsAuthenticated
from rest_framework.response import Response

from apps.issues.models import Issue
from apps.merge_requests.models import MergeRequest
from apps.repositories.models import Repository
from apps.wiki.models import WikiPage

User = get_user_model()


@api_view(["GET"])
@permission_classes([IsAuthenticated])
def global_search(request):
    q = request.query_params.get("q", "").strip()
    if not q or len(q) < 2:
        return Response({"detail": "Query must be at least 2 characters."}, status=400)

    results = {}
    user = request.user

    accessible_repos = Repository.objects.filter(
        Q(visibility="public") | Q(owner_id=user.id) | Q(access_list__user=user),
    ).distinct()

    repos = accessible_repos.filter(
        Q(path__icontains=q) | Q(name__icontains=q) | Q(description__icontains=q),
    )[:10]
    if repos:
        results["repositories"] = [
            {"id": str(r.id), "path": r.path, "type": "repository"} for r in repos
        ]

    issues = (
        Issue.objects.filter(
            Q(title__icontains=q) | Q(description__icontains=q),
            repository__in=accessible_repos,
        )
        .select_related("repository")
        .distinct()[:10]
    )
    if issues:
        results["issues"] = [
            {
                "id": str(i.id),
                "number": i.number,
                "title": i.title,
                "state": i.state,
                "repo": i.repository.path,
                "type": "issue",
            }
            for i in issues
        ]

    mrs = (
        MergeRequest.objects.filter(
            Q(title__icontains=q) | Q(description__icontains=q),
            repository__in=accessible_repos,
        )
        .select_related("repository")
        .distinct()[:10]
    )
    if mrs:
        results["merge_requests"] = [
            {
                "id": str(m.id),
                "number": m.number,
                "title": m.title,
                "state": m.state,
                "repo": m.repository.path,
                "type": "merge_request",
            }
            for m in mrs
        ]

    wiki = (
        WikiPage.objects.filter(
            Q(title__icontains=q) | Q(content__icontains=q),
            repository__in=accessible_repos,
        )
        .select_related("repository")
        .distinct()[:10]
    )
    if wiki:
        results["wiki"] = [
            {
                "id": str(w.id),
                "slug": w.slug,
                "title": w.title,
                "repo": w.repository.path,
                "type": "wiki",
            }
            for w in wiki
        ]

    return Response(results)
