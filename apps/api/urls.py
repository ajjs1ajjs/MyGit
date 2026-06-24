from django.urls import include, path
from rest_framework.routers import DefaultRouter

from apps.ci_cd.views import JobViewSet, PipelineViewSet
from apps.issues.views import IssueViewSet
from apps.merge_requests.views import MergeRequestViewSet
from apps.notifications.views import NotificationViewSet
from apps.organizations.views import GroupViewSet
from apps.search.views import global_search
from apps.snippets.views import SnippetViewSet
from apps.webhooks.views import WebhookViewSet
from apps.wiki.views import WikiPageViewSet

from .views.auth import AuthViewSet
from .views.internal import authorized_keys, check_access, post_receive, pre_receive
from .views.projects import ProjectViewSet
from .views.users import UserViewSet

router = DefaultRouter()
router.register(
    r"projects/(?P<project_id>[^/.]+)/pipelines",
    PipelineViewSet,
    basename="pipelines",
)
router.register(
    r"projects/(?P<project_id>[^/.]+)/pipelines/(?P<pipeline_id>[^/.]+)/jobs",
    JobViewSet,
    basename="pipeline-jobs",
)
router.register(
    r"projects/(?P<project_id>[^/.]+)/hooks",
    WebhookViewSet,
    basename="webhooks",
)
router.register(r"notifications", NotificationViewSet, basename="notifications")
router.register(r"groups", GroupViewSet, basename="groups")
router.register(r"snippets", SnippetViewSet, basename="snippets")
router.register(
    r"projects/(?P<project_id>[^/.]+)/wiki",
    WikiPageViewSet,
    basename="wiki",
)
router.register(r"auth", AuthViewSet, basename="auth")
router.register(r"users", UserViewSet, basename="users")
router.register(r"projects", ProjectViewSet, basename="projects")
router.register(r"projects/(?P<project_id>[^/.]+)/issues", IssueViewSet, basename="issues")
router.register(
    r"projects/(?P<project_id>[^/.]+)/merge_requests",
    MergeRequestViewSet,
    basename="merge-requests",
)

urlpatterns = [
    path("internal/authorized_keys", authorized_keys, name="authorized-keys"),
    path("internal/check_access", check_access, name="check-access"),
    path("internal/pre-receive", pre_receive, name="pre-receive"),
    path("internal/post-receive", post_receive, name="post-receive"),
    path("search", global_search, name="global-search"),
    path("", include(router.urls)),
]
