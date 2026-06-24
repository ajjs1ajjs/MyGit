from django.conf import settings
from django.db import models

from apps.core.models import BaseModel


class Repository(BaseModel):
    class Visibility(models.TextChoices):
        PUBLIC = "public", "Public"
        PRIVATE = "private", "Private"
        INTERNAL = "internal", "Internal"

    owner_type = models.CharField(max_length=50, default="user")
    owner_id = models.UUIDField()
    name = models.CharField(max_length=255)
    path = models.CharField(max_length=512, unique=True)
    description = models.TextField(blank=True)
    visibility = models.CharField(
        max_length=20, choices=Visibility.choices, default=Visibility.PRIVATE
    )
    default_branch = models.CharField(max_length=255, default="main")
    is_archived = models.BooleanField(default=False)
    is_fork = models.BooleanField(default=False)
    forked_from = models.ForeignKey(
        "self", null=True, blank=True, on_delete=models.SET_NULL, related_name="forks"
    )
    size_kb = models.BigIntegerField(default=0)

    class Meta:
        db_table = "repositories_repository"
        ordering = ["-updated_at"]

    def __str__(self):
        return self.path

    @property
    def disk_path(self):
        repo_root = getattr(settings, "MYGIT_REPOS_ROOT", None)
        if repo_root is None:
            import os

            repo_root = os.path.join(settings.BASE_DIR, "repos")
        from pathlib import Path

        return Path(repo_root) / f"{self.path}.git"


class RepositoryAccess(BaseModel):
    class Role(models.IntegerChoices):
        NONE = 0, "No access"
        GUEST = 10, "Guest"
        REPORTER = 20, "Reporter"
        DEVELOPER = 30, "Developer"
        MAINTAINER = 40, "Maintainer"
        OWNER = 50, "Owner"

    user = models.ForeignKey("accounts.User", on_delete=models.CASCADE, related_name="repo_access")
    repository = models.ForeignKey(Repository, on_delete=models.CASCADE, related_name="access_list")
    role = models.IntegerField(choices=Role.choices, default=Role.DEVELOPER)

    class Meta:
        db_table = "repositories_access"
        unique_together = [("user", "repository")]

    def __str__(self):
        return f"{self.user.username} -> {self.repository.path} ({self.get_role_display()})"
