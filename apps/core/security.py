"""Shared security helpers (SSRF protection for user-supplied URLs)."""
import ipaddress
import socket
from urllib.parse import urlparse

from rest_framework import serializers

BLOCKED_NETWORKS = [
    ipaddress.ip_network("127.0.0.0/8"),
    ipaddress.ip_network("10.0.0.0/8"),
    ipaddress.ip_network("172.16.0.0/12"),
    ipaddress.ip_network("192.168.0.0/16"),
    ipaddress.ip_network("169.254.0.0/16"),
    ipaddress.ip_network("0.0.0.0/8"),
    ipaddress.ip_network("100.64.0.0/10"),
    ipaddress.ip_network("::1/128"),
    ipaddress.ip_network("fc00::/7"),
    ipaddress.ip_network("fe80::/10"),
]


def validate_public_http_url(url_str: str, allow_credentials: bool = False) -> str:
    """Validate an http(s) URL and reject hosts that resolve to private networks.

    Raises ``rest_framework.exceptions.ValidationError`` on invalid input.
    """
    parsed = urlparse(url_str)
    if parsed.scheme not in ("https", "http"):
        raise serializers.ValidationError(
            "Only http(s) URLs are allowed (file://, ssh:// and local paths are blocked)."
        )
    if not allow_credentials and (parsed.username or parsed.password):
        raise serializers.ValidationError("Credentials must not be embedded in the URL.")
    host = parsed.hostname or ""
    if not host:
        raise serializers.ValidationError("URL host is required.")
    try:
        for addrinfo in socket.getaddrinfo(host, 80):
            ip = ipaddress.ip_address(addrinfo[4][0])
            if any(ip in net for net in BLOCKED_NETWORKS):
                raise serializers.ValidationError(
                    "URL points to a private or internal network."
                )
    except serializers.ValidationError:
        raise
    except Exception:
        raise serializers.ValidationError("Could not resolve URL host.") from None
    return url_str
