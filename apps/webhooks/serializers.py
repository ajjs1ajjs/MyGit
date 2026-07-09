import ipaddress
import socket
from urllib.parse import urlparse

from rest_framework import serializers

from .models import Webhook, WebhookDelivery

BLOCKED_NETWORKS = [
    ipaddress.ip_network("127.0.0.0/8"),
    ipaddress.ip_network("10.0.0.0/8"),
    ipaddress.ip_network("172.16.0.0/12"),
    ipaddress.ip_network("192.168.0.0/16"),
    ipaddress.ip_network("169.254.0.0/16"),
    ipaddress.ip_network("::1/128"),
    ipaddress.ip_network("fc00::/7"),
]


def validate_webhook_url(url_str: str) -> bool:
    parsed = urlparse(url_str)
    if parsed.scheme not in ("https", "http"):
        return False
    try:
        host = parsed.hostname or ""
        port = parsed.port or 80
        for addrinfo in socket.getaddrinfo(host, port):
            ip = addrinfo[4][0]
            addr = ipaddress.ip_address(ip)
            if any(addr in net for net in BLOCKED_NETWORKS):
                return False
        return True
    except Exception:
        return False


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
