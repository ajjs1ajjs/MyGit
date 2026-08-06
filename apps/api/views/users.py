from django.contrib.auth import get_user_model
from django.contrib.auth.password_validation import validate_password
from django.core.exceptions import ValidationError as DjangoValidationError
from django.shortcuts import get_object_or_404
from rest_framework import serializers, status, viewsets
from rest_framework.decorators import action
from rest_framework.permissions import IsAuthenticated
from rest_framework.response import Response

from apps.accounts.models import IntegrationToken, PersonalAccessToken, SSHKey

User = get_user_model()


class UserSerializer(serializers.ModelSerializer):
    class Meta:
        model = User
        fields = [
            "id", "username", "email", "full_name", "bio", "avatar",
            "is_superuser", "is_active", "must_change_password", "date_joined",
        ]
        read_only_fields = ["id", "date_joined"]

    def to_representation(self, instance):
        data = super().to_representation(instance)
        request = self.context.get("request")
        viewer = getattr(request, "user", None)
        viewer_ok = bool(viewer and getattr(viewer, "is_authenticated", False))
        is_self = viewer_ok and str(viewer.id) == str(instance.id)
        if not (viewer_ok and (getattr(viewer, "is_superuser", False) or is_self)):
            for field in ("email", "is_superuser", "is_active", "must_change_password"):
                data.pop(field, None)
        return data


class SSHKeySerializer(serializers.ModelSerializer):
    class Meta:
        model = SSHKey
        fields = ["id", "title", "public_key", "fingerprint", "created_at"]
        read_only_fields = ["id", "fingerprint", "created_at"]

    def validate_public_key(self, value):
        import base64

        value = (value or "").strip()
        if not value or "\n" in value or "\r" in value:
            raise serializers.ValidationError("Public key must be a single line.")
        parts = value.split()
        if len(parts) < 2 or not parts[0].startswith(
            ("ssh-", "ecdsa-", "sk-", "rsa-sha2-")
        ):
            raise serializers.ValidationError("Invalid SSH public key format.")
        try:
            base64.b64decode(parts[1], validate=True)
        except Exception:
            raise serializers.ValidationError("Invalid SSH key body.") from None
        return value


class PatCreateSerializer(serializers.Serializer):
    name = serializers.CharField(max_length=255)
    scopes = serializers.ListField(
        child=serializers.ChoiceField(choices=["read_repo", "write_repo", "api", "admin"]),
        required=False,
        default=list,
    )
    expires_in_days = serializers.IntegerField(required=False, allow_null=True)


class PatResponseSerializer(serializers.ModelSerializer):
    token = serializers.CharField(read_only=True)

    class Meta:
        model = PersonalAccessToken
        fields = ["id", "name", "scopes", "expires_at", "created_at", "token"]
        read_only_fields = ["id", "expires_at", "created_at", "token"]


class UserViewSet(viewsets.GenericViewSet):
    queryset = User.objects.all()
    permission_classes = [IsAuthenticated]
    lookup_field = "username"
    lookup_value_regex = r"[^/]+"

    def get_serializer_class(self):
        if self.action in ("ssh_keys",):
            return SSHKeySerializer
        if self.action in ("create_pat", "pat_tokens"):
            return PatResponseSerializer
        return UserSerializer

    def retrieve(self, request, username=None):
        user = get_object_or_404(User, username=username)
        return Response(
            UserSerializer(user, context={"request": request}).data
        )

    def partial_update(self, request, username=None):
        user = get_object_or_404(User, username=username)
        if not request.user.is_superuser and user != request.user:
            return Response({"detail": "Permission denied."}, status=status.HTTP_403_FORBIDDEN)
        data = request.data.copy()
        if not request.user.is_superuser:
            data.pop("is_superuser", None)
            data.pop("is_active", None)
        password = data.pop("password", None)
        if isinstance(password, list):
            password = password[0] if password else None
        serializer = UserSerializer(user, data=data, partial=True, context={"request": request})
        serializer.is_valid(raise_exception=True)
        serializer.save()
        if password:
            try:
                validate_password(password, user)
            except DjangoValidationError as e:
                return Response({"detail": list(e.messages)}, status=status.HTTP_400_BAD_REQUEST)
            user.set_password(password)
            user.save(update_fields=["password"])
        return Response(serializer.data)

    def list(self, request):
        page = self.paginate_queryset(self.get_queryset())
        if page is not None:
            return self.get_paginated_response(
                UserSerializer(page, many=True, context={"request": request}).data
            )
        return Response(
            UserSerializer(self.get_queryset(), many=True, context={"request": request}).data
        )

    @action(methods=["get", "patch"], detail=False)
    def me(self, request):
        if request.method == "GET":
            data = UserSerializer(request.user, context={"request": request}).data
            data["must_change_password"] = request.user.must_change_password
            return Response(data)
        data = request.data.copy()
        for field in ("is_superuser", "is_staff", "is_active", "must_change_password", "password"):
            data.pop(field, None)
        serializer = UserSerializer(
            request.user, data=data, partial=True, context={"request": request}
        )
        serializer.is_valid(raise_exception=True)
        serializer.save()
        return Response(serializer.data)

    @action(methods=["post"], detail=False)
    def change_password(self, request):
        current = request.data.get("current_password", "")
        new_password = request.data.get("new_password", "")
        if not request.user.check_password(current):
            return Response(
                {"detail": "Current password is incorrect."},
                status=status.HTTP_400_BAD_REQUEST,
            )
        if len(new_password) < 8:
            return Response(
                {"detail": "Password must be at least 8 characters."},
                status=status.HTTP_400_BAD_REQUEST,
            )
        try:
            validate_password(new_password, request.user)
        except DjangoValidationError as e:
            return Response({"detail": list(e.messages)}, status=status.HTTP_400_BAD_REQUEST)
        request.user.set_password(new_password)
        request.user.must_change_password = False
        request.user.save(update_fields=["password", "must_change_password"])
        return Response({"detail": "Password changed."})

    @action(methods=["get", "post"], detail=True)
    def keys(self, request, username=None):
        user = get_object_or_404(User, username=username)
        if request.method == "GET":
            keys = SSHKey.objects.filter(user=user)
            return Response(SSHKeySerializer(keys, many=True).data)

        if user != request.user and not request.user.is_superuser:
            return Response({"detail": "Permission denied."}, status=status.HTTP_403_FORBIDDEN)

        serializer = SSHKeySerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        serializer.save(user=user)
        return Response(serializer.data, status=status.HTTP_201_CREATED)

    @action(methods=["delete"], detail=True, url_path="keys/(?P<key_id>[^/.]+)")
    def delete_key(self, request, username=None, key_id=None):
        user = get_object_or_404(User, username=username)
        if user != request.user and not request.user.is_superuser:
            return Response({"detail": "Permission denied."}, status=status.HTTP_403_FORBIDDEN)
        key = get_object_or_404(SSHKey, id=key_id, user=user)
        key.delete()
        return Response(status=status.HTTP_204_NO_CONTENT)

    @action(methods=["get", "post"], detail=True, url_path="tokens")
    def pat_tokens(self, request, username=None):
        user = get_object_or_404(User, username=username)
        if user != request.user and not request.user.is_superuser:
            return Response({"detail": "Permission denied."}, status=status.HTTP_403_FORBIDDEN)

        if request.method == "GET":
            tokens = PersonalAccessToken.objects.filter(user=user)
            return Response(PatResponseSerializer(tokens, many=True).data)

        serializer = PatCreateSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)

        import hashlib
        import secrets
        from datetime import timedelta

        from django.utils import timezone

        raw_token = f"mygit_pat_{secrets.token_urlsafe(32)}"
        token_hash = hashlib.sha256(raw_token.encode()).hexdigest()

        expires_at = None
        if serializer.validated_data.get("expires_in_days"):
            expires_at = timezone.now() + timedelta(
                days=serializer.validated_data["expires_in_days"]
            )

        pat = PersonalAccessToken.objects.create(
            user=user,
            name=serializer.validated_data["name"],
            token_hash=token_hash,
            scopes=serializer.validated_data.get("scopes", []),
            expires_at=expires_at,
        )

        result = PatResponseSerializer(pat).data
        result["token"] = raw_token
        return Response(result, status=status.HTTP_201_CREATED)

    @action(methods=["delete"], detail=True, url_path="tokens/(?P<token_id>[^/.]+)")
    def delete_token(self, request, username=None, token_id=None):
        user = get_object_or_404(User, username=username)
        if user != request.user and not request.user.is_superuser:
            return Response({"detail": "Permission denied."}, status=status.HTTP_403_FORBIDDEN)
        token = get_object_or_404(PersonalAccessToken, id=token_id, user=user)
        token.delete()
        return Response(status=status.HTTP_204_NO_CONTENT)

    @action(methods=["get", "post"], detail=True, url_path="integration-tokens")
    def integration_tokens(self, request, username=None):
        user = get_object_or_404(User, username=username)
        if user != request.user and not request.user.is_superuser:
            return Response({"detail": "Permission denied."}, status=status.HTTP_403_FORBIDDEN)

        if request.method == "GET":
            tokens = IntegrationToken.objects.filter(user=user)
            data = []
            for t in tokens:
                raw = t.get_token()
                masked = raw[:4] + "****" + raw[-4:] if len(raw) > 8 else "****"
                data.append({
                    "id": t.id,
                    "provider": t.provider,
                    "masked_token": masked,
                    "created_at": t.created_at,
                })
            return Response(data)

        # POST: create or update integration token
        provider = request.data.get("provider")
        token = request.data.get("token")
        if not provider or not token:
            return Response({"detail": "provider and token are required."}, status=status.HTTP_400_BAD_REQUEST)
        if provider not in ["github", "gitlab"]:
            return Response({"detail": "Invalid provider."}, status=status.HTTP_400_BAD_REQUEST)

        obj, created = IntegrationToken.objects.update_or_create(
            user=user, provider=provider, defaults={"token": token}
        )
        raw = obj.get_token()
        masked = raw[:4] + "****" + raw[-4:] if len(raw) > 8 else "****"
        return Response({
            "id": obj.id,
            "provider": obj.provider,
            "masked_token": masked,
            "created_at": obj.created_at,
        }, status=status.HTTP_201_CREATED if created else status.HTTP_200_OK)

    @action(methods=["delete"], detail=True, url_path="integration-tokens/(?P<provider>[^/.]+)")
    def delete_integration_token(self, request, username=None, provider=None):
        user = get_object_or_404(User, username=username)
        if user != request.user and not request.user.is_superuser:
            return Response({"detail": "Permission denied."}, status=status.HTTP_403_FORBIDDEN)

        token = get_object_or_404(IntegrationToken, user=user, provider=provider)
        token.delete()
        return Response(status=status.HTTP_204_NO_CONTENT)

