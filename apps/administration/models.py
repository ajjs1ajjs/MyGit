from django.conf import settings
from django.db import models

from apps.core.models import BaseModel


class AuditEvent(BaseModel):
    class Severity(models.TextChoices):
        INFO = "info", "Info"
        WARNING = "warning", "Warning"
        CRITICAL = "critical", "Critical"

    actor = models.ForeignKey(
        settings.AUTH_USER_MODEL,
        null=True,
        blank=True,
        on_delete=models.SET_NULL,
        related_name="audit_events",
    )
    action = models.CharField(max_length=128, db_index=True)
    target_type = models.CharField(max_length=128, blank=True, db_index=True)
    target_id = models.CharField(max_length=128, blank=True)
    message = models.TextField(blank=True)
    severity = models.CharField(max_length=20, choices=Severity.choices, default=Severity.INFO)
    ip_address = models.GenericIPAddressField(null=True, blank=True)
    metadata = models.JSONField(default=dict, blank=True)

    class Meta:
        db_table = "administration_audit_event"
        ordering = ["-created_at"]

    def __str__(self):
        return f"{self.action} {self.target_type}:{self.target_id}".strip()


class BackupSchedule(BaseModel):
    class Frequency(models.TextChoices):
        HOURLY = "hourly", "Hourly"
        DAILY = "daily", "Daily"
        WEEKLY = "weekly", "Weekly"

    name = models.CharField(max_length=255)
    frequency = models.CharField(max_length=20, choices=Frequency.choices, default=Frequency.DAILY)
    time_of_day = models.TimeField(default="02:15")
    enabled = models.BooleanField(default=True)
    encrypt = models.BooleanField(default=True)
    upload = models.BooleanField(default=True)
    keep_local = models.PositiveIntegerField(default=14)
    last_run_at = models.DateTimeField(null=True, blank=True)
    next_run_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "administration_backup_schedule"
        ordering = ["name"]

    def __str__(self):
        return self.name


class BackupJob(BaseModel):
    class Status(models.TextChoices):
        QUEUED = "queued", "Queued"
        RUNNING = "running", "Running"
        SUCCESS = "success", "Success"
        FAILED = "failed", "Failed"

    class Kind(models.TextChoices):
        CREATE = "create", "Create"
        VERIFY = "verify", "Verify"
        RESTORE = "restore", "Restore"
        TEST_RESTORE = "test_restore", "Test restore"
        REPLICATE = "replicate", "Replicate"

    schedule = models.ForeignKey(
        BackupSchedule,
        null=True,
        blank=True,
        on_delete=models.SET_NULL,
        related_name="jobs",
    )
    kind = models.CharField(max_length=30, choices=Kind.choices, default=Kind.CREATE)
    status = models.CharField(max_length=20, choices=Status.choices, default=Status.QUEUED)
    archive_path = models.CharField(max_length=1024, blank=True)
    started_at = models.DateTimeField(null=True, blank=True)
    finished_at = models.DateTimeField(null=True, blank=True)
    log = models.TextField(blank=True)
    metadata = models.JSONField(default=dict, blank=True)

    class Meta:
        db_table = "administration_backup_job"
        ordering = ["-created_at"]

    def __str__(self):
        return f"{self.kind} {self.status}"


class MirrorTarget(BaseModel):
    name = models.CharField(max_length=255)
    target = models.CharField(max_length=1024)
    enabled = models.BooleanField(default=True)
    delete_remote_missing = models.BooleanField(default=False)
    last_run_at = models.DateTimeField(null=True, blank=True)
    last_status = models.CharField(max_length=20, blank=True)
    last_error = models.TextField(blank=True)

    class Meta:
        db_table = "administration_mirror_target"
        ordering = ["name"]

    def __str__(self):
        return self.name


class RepositoryImportJob(BaseModel):
    class Provider(models.TextChoices):
        GITHUB = "github", "GitHub"
        GITLAB = "gitlab", "GitLab"
        GITEA = "gitea", "Gitea"
        CUSTOM = "custom", "Custom"

    class Status(models.TextChoices):
        QUEUED = "queued", "Queued"
        RUNNING = "running", "Running"
        SUCCESS = "success", "Success"
        FAILED = "failed", "Failed"

    actor = models.ForeignKey(
        settings.AUTH_USER_MODEL,
        on_delete=models.CASCADE,
        related_name="repository_import_jobs",
    )
    provider = models.CharField(max_length=20, choices=Provider.choices)
    source = models.CharField(max_length=1024)
    target_path = models.CharField(max_length=512)
    status = models.CharField(max_length=20, choices=Status.choices, default=Status.QUEUED)
    error = models.TextField(blank=True)
    repository = models.ForeignKey(
        "repositories.Repository",
        null=True,
        blank=True,
        on_delete=models.SET_NULL,
        related_name="import_jobs",
    )

    class Meta:
        db_table = "administration_repository_import_job"
        ordering = ["-created_at"]

    def __str__(self):
        return f"{self.provider}: {self.target_path}"


class TwoFactorDevice(BaseModel):
    class Method(models.TextChoices):
        TOTP = "totp", "TOTP"
        WEBAUTHN = "webauthn", "WebAuthn"

    user = models.ForeignKey(
        settings.AUTH_USER_MODEL,
        on_delete=models.CASCADE,
        related_name="two_factor_devices",
    )
    method = models.CharField(max_length=20, choices=Method.choices)
    name = models.CharField(max_length=255)
    confirmed = models.BooleanField(default=False)
    secret = models.TextField(blank=True)
    credential_id = models.CharField(max_length=512, blank=True)
    public_key = models.TextField(blank=True)
    sign_count = models.BigIntegerField(default=0)
    last_used_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "administration_two_factor_device"
        unique_together = [("user", "method", "name")]
        ordering = ["user__username", "name"]

    def __str__(self):
        return f"{self.user.username}: {self.name}"
