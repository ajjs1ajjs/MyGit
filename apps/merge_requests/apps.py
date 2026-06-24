from django.apps import AppConfig
from django.utils.translation import gettext_lazy as _


class MergeRequestsConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"
    name = "apps.merge_requests"
    verbose_name = _("Merge Requests")
