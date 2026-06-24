from django.contrib.auth import get_user_model
from django.shortcuts import get_object_or_404
from rest_framework import serializers, status, viewsets
from rest_framework.decorators import action
from rest_framework.permissions import IsAuthenticated
from rest_framework.response import Response

from apps.accounts.models import PersonalAccessToken, SSHKey

User = get_user_model()


class UserSerializer(serializers.ModelSerializer):
    class Meta:
        model = User
        fields = ["id", "username", "email", "full_name", "bio", "avatar", "date_joined"]
        read_only_fields = ["id", "date_joined"]


class SSHKeySerializer(serializers.ModelSerializer):
    class Meta:
        model = SSHKey
        fields = ["id", "title", "public_key", "fingerprint", "created_at"]
        read_only_fields = ["id", "fingerprint", "created_at"]


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

    def get_serializer_class(self):
        if self.action in ("ssh_keys",):
            return SSHKeySerializer
        if self.action in ("create_pat", "pat_tokens"):
            return PatResponseSerializer
        return UserSerializer

    def retrieve(self, request, username=None):
        user = get_object_or_404(User, username=username)
        return Response(UserSerializer(user).data)

    def list(self, request):
        page = self.paginate_queryset(self.get_queryset())
        if page is not None:
            return self.get_paginated_response(UserSerializer(page, many=True).data)
        return Response(UserSerializer(self.get_queryset(), many=True).data)

    @action(methods=["get", "patch"], detail=False)
    def me(self, request):
        if request.method == "GET":
            return Response(UserSerializer(request.user).data)
        serializer = UserSerializer(request.user, data=request.data, partial=True)
        serializer.is_valid(raise_exception=True)
        serializer.save()
        return Response(serializer.data)

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
