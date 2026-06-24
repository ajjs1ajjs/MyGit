import re

from django.conf import settings
from django.core.exceptions import SuspiciousFileOperation, ValidationError
from django.db import models

from apps.core.models import BaseModel

REPOSITORY_COMPONENT_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,254}$")


def validate_repository_component(value: str) -> str:
    value = (value or "").strip()
    if (
        not REPOSITORY_COMPONENT_RE.fullmatch(value)
        or value in {".", ".."}
        or value.endswith(".git")
    ):
        raise ValidationError(
            "Use only letters, numbers, dots, underscores and hyphens; "
            "the name must not end with .git."
        )
    return value


def validate_repository_path(value: str) -> str:
    value = (value or "").strip()
    parts = value.split("/")
    if len(parts) != 2:
        raise ValidationError("Repository path must be in owner/name format.")
    for part in parts:
        validate_repository_component(part)
    return value


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
    custom_disk_path = models.CharField(max_length=1024, blank=True, null=True, default=None)

    class Meta:
        db_table = "repositories_repository"
        ordering = ["-updated_at"]

    def __str__(self):
        return self.path

    def clean(self):
        super().clean()
        validate_repository_component(self.name)
        validate_repository_path(self.path)

    @property
    def disk_path(self):
        if self.custom_disk_path:
            from pathlib import Path
            return Path(self.custom_disk_path).resolve(strict=False)

        repo_root = getattr(settings, "MYGIT_REPOS_ROOT", None)
        if repo_root is None:
            import os

            repo_root = os.path.join(settings.BASE_DIR, "repos")
        from pathlib import Path

        root = Path(repo_root).resolve(strict=False)
        candidate = (root / f"{self.path}.git").resolve(strict=False)
        if candidate == root or not candidate.is_relative_to(root):
            raise SuspiciousFileOperation("Repository path escapes the configured repository root.")
        return candidate


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
