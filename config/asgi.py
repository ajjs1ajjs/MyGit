import os

from channels.routing import ProtocolTypeRouter, URLRouter
from django.core.asgi import get_asgi_application

from apps.ci_cd.routing import websocket_urlpatterns
from apps.ci_cd.websocket_auth import JWTAuthMiddleware

# Production entrypoint: never silently fall back to DEBUG=True local settings.
os.environ.setdefault("DJANGO_SETTINGS_MODULE", "config.settings.production")

application = ProtocolTypeRouter({
    "http": get_asgi_application(),
    "websocket": JWTAuthMiddleware(
        URLRouter(websocket_urlpatterns)
    ),
})
