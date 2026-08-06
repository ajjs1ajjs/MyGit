from urllib.parse import parse_qs

from channels.db import database_sync_to_async
from rest_framework_simplejwt.authentication import JWTAuthentication


class JWTAuthMiddleware:
    """Authenticate WebSocket connections using a JWT access token.

    The token is read from the ``Authorization: Bearer <jwt>`` header or, as a
    fallback for browser clients, from the ``?token=<jwt>`` query parameter.
    """

    def __init__(self, app):
        self.app = app

    async def __call__(self, scope, receive, send):
        scope = dict(scope)
        token = None
        for name, value in scope.get("headers", []):
            if name == b"authorization":
                decoded = value.decode("utf-8", errors="ignore")
                if decoded.startswith("Bearer "):
                    token = decoded[7:]
                    break
        if not token:
            query = parse_qs((scope.get("query_string") or b"").decode("utf-8", errors="ignore"))
            candidate = query.get("token")
            if candidate:
                token = candidate[0]

        scope["user"] = await self._resolve_user(token) if token else None
        return await self.app(scope, receive, send)

    @database_sync_to_async
    def _resolve_user(self, token: str):
        try:
            jwt_auth = JWTAuthentication()
            validated = jwt_auth.get_validated_token(token)
            return jwt_auth.get_user(validated)
        except Exception:
            return None
