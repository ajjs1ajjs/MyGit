from django.conf import settings
from django.db import models

from apps.core.models import BaseModel


class Notification(BaseModel):
    class Type(models.TextChoices):
        ISSUE = "issue", "Issue"
        MERGE_REQUEST = "merge_request", "Merge Request"
        PIPELINE = "pipeline", "Pipeline"
        WIKI = "wiki", "Wiki"
        SYSTEM = "system", "System"

    recipient = models.ForeignKey(
        settings.AUTH_USER_MODEL, on_delete=models.CASCADE, related_name="notifications"
    )
    type = models.CharField(max_length=20, choices=Type.choices)
    title = models.CharField(max_length=512)
    message = models.TextField(blank=True)
    link = models.CharField(max_length=1024, blank=True)
    is_read = models.BooleanField(default=False)

    class Meta:
        db_table = "notifications_notification"
        ordering = ["-created_at"]

    def __str__(self):
        return f"{self.recipient.username}: {self.title}"
