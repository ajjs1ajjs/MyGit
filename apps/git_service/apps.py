from django.apps import AppConfig


class GitServiceConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"
    name = "apps.git_service"
    label = "git_service"
    verbose_name = "Git Service"
