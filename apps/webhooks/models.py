import hashlib
import hmac

from django.db import models

from apps.core.models import BaseModel


class Webhook(BaseModel):
    repository = models.ForeignKey(
        "repositories.Repository",
        null=True,
        blank=True,
        on_delete=models.CASCADE,
        related_name="webhooks",
    )
    url = models.URLField(max_length=1024)
    secret = models.CharField(max_length=255, blank=True)
    events = models.JSONField(default=list)
    is_active = models.BooleanField(default=True)

    class Meta:
        db_table = "webhooks_webhook"
        ordering = ["-created_at"]

    def __str__(self):
        repo = self.repository.path if self.repository else "system"
        return f"{repo}: {self.url}"

    def sign_payload(self, payload: bytes) -> str:
        if not self.secret:
            return ""
        return hmac.new(self.secret.encode(), payload, hashlib.sha256).hexdigest()


class WebhookDelivery(BaseModel):
    class Status(models.TextChoices):
        PENDING = "pending", "Pending"
        SUCCESS = "success", "Success"
        FAILED = "failed", "Failed"

    webhook = models.ForeignKey(Webhook, on_delete=models.CASCADE, related_name="deliveries")
    event = models.CharField(max_length=255)
    payload = models.JSONField(default=dict)
    status = models.CharField(max_length=20, choices=Status.choices, default=Status.PENDING)
    response_code = models.IntegerField(null=True, blank=True)
    response_body = models.TextField(blank=True)
    retry_count = models.IntegerField(default=0)
    delivered_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "webhooks_delivery"
        ordering = ["-created_at"]

    def __str__(self):
        return f"{self.webhook} - {self.event} ({self.status})"
