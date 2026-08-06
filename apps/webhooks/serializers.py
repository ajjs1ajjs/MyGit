from rest_framework import serializers

from apps.core.security import validate_public_http_url

from .models import Webhook, WebhookDelivery


def validate_webhook_url(url_str: str) -> bool:
    try:
        validate_public_http_url(url_str)
        return True
    except Exception:
        return False


class WebhookSerializer(serializers.ModelSerializer):
    secret = serializers.CharField(
        write_only=True, required=False, allow_blank=True, trim_whitespace=False
    )

    class Meta:
        model = Webhook
        fields = [
            "id",
            "url",
            "secret",
            "events",
            "is_active",
            "repository",
            "created_at",
            "updated_at",
        ]
        read_only_fields = ["id", "repository", "created_at", "updated_at"]

    def validate_url(self, value):
        if not validate_webhook_url(value):
            raise serializers.ValidationError("URL points to a blocked or internal network.")
        return value


class WebhookDeliverySerializer(serializers.ModelSerializer):
    class Meta:
        model = WebhookDelivery
        fields = [
            "id",
            "webhook",
            "event",
            "payload",
            "status",
            "response_code",
            "response_body",
            "retry_count",
            "delivered_at",
            "created_at",
        ]
        read_only_fields = fields
