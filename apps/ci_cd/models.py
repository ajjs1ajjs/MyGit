from django.conf import settings
from django.db import models

from apps.core.models import BaseModel, TimeStampedModel


class Pipeline(BaseModel):
    class Status(models.TextChoices):
        PENDING = "pending", "Pending"
        RUNNING = "running", "Running"
        SUCCESS = "success", "Success"
        FAILED = "failed", "Failed"
        CANCELED = "canceled", "Canceled"

    repository = models.ForeignKey(
        "repositories.Repository", on_delete=models.CASCADE, related_name="pipelines"
    )
    author = models.ForeignKey(
        settings.AUTH_USER_MODEL, on_delete=models.CASCADE, related_name="pipelines"
    )
    ref = models.CharField(max_length=255)
    sha = models.CharField(max_length=40)
    status = models.CharField(max_length=20, choices=Status.choices, default=Status.PENDING)
    stages = models.JSONField(default=list)
    started_at = models.DateTimeField(null=True, blank=True)
    finished_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "cicd_pipeline"
        ordering = ["-created_at"]

    def __str__(self):
        return f"{self.repository.path} ({self.ref}) #{self.short_sha}"

    @property
    def short_sha(self):
        return self.sha[:8] if self.sha else ""


class Job(BaseModel, TimeStampedModel):
    class Status(models.TextChoices):
        PENDING = "pending", "Pending"
        RUNNING = "running", "Running"
        SUCCESS = "success", "Success"
        FAILED = "failed", "Failed"
        CANCELED = "canceled", "Canceled"

    pipeline = models.ForeignKey(Pipeline, on_delete=models.CASCADE, related_name="jobs")
    name = models.CharField(max_length=255)
    stage = models.CharField(max_length=255)
    status = models.CharField(max_length=20, choices=Status.choices, default=Status.PENDING)
    log = models.TextField(blank=True)
    artifacts = models.JSONField(default=list, blank=True)
    runner_id = models.CharField(max_length=255, blank=True)
    started_at = models.DateTimeField(null=True, blank=True)
    finished_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "cicd_job"
        ordering = ["stage", "created_at"]

    def __str__(self):
        return f"{self.pipeline} / {self.name}"
