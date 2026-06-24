from rest_framework import serializers

from .models import Webhook, WebhookDelivery


class WebhookSerializer(serializers.ModelSerializer):
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
        read_only_fields = ["id", "created_at", "updated_at"]


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
