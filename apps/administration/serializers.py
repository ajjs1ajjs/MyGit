from rest_framework import serializers

from .models import (
    AuditEvent,
    BackupJob,
    BackupSchedule,
    MirrorTarget,
    RepositoryImportJob,
    TwoFactorDevice,
)


class AuditEventSerializer(serializers.ModelSerializer):
    actor_username = serializers.CharField(source="actor.username", read_only=True, allow_null=True)

    class Meta:
        model = AuditEvent
        fields = "__all__"
        read_only_fields = ["id", "created_at", "updated_at"]


class BackupScheduleSerializer(serializers.ModelSerializer):
    class Meta:
        model = BackupSchedule
        fields = "__all__"
        read_only_fields = ["id", "last_run_at", "next_run_at", "created_at", "updated_at"]


class BackupJobSerializer(serializers.ModelSerializer):
    schedule_name = serializers.CharField(source="schedule.name", read_only=True, allow_null=True)

    class Meta:
        model = BackupJob
        fields = "__all__"
        read_only_fields = ["id", "created_at", "updated_at"]


class MirrorTargetSerializer(serializers.ModelSerializer):
    class Meta:
        model = MirrorTarget
        fields = "__all__"
        read_only_fields = [
            "id",
            "last_run_at",
            "last_status",
            "last_error",
            "created_at",
            "updated_at",
        ]


class RepositoryImportJobSerializer(serializers.ModelSerializer):
    actor_username = serializers.CharField(source="actor.username", read_only=True)

    class Meta:
        model = RepositoryImportJob
        fields = "__all__"
        read_only_fields = [
            "id",
            "actor",
            "status",
            "error",
            "repository",
            "created_at",
            "updated_at",
        ]

    def validate_target_path(self, value):
        from apps.repositories.models import validate_repository_path

        try:
            validate_repository_path(value)
        except Exception as e:
            raise serializers.ValidationError(str(e)) from None
        return value

    def validate_source(self, value):
        from apps.core.security import validate_public_http_url

        value = (value or "").strip()
        provider = self.initial_data.get("provider", "")
        if provider in ("github", "gitlab"):
            if not value or "://" in value:
                raise serializers.ValidationError(
                    "source must be in 'owner/repo' format for this provider."
                )
            return value
        if not value:
            raise serializers.ValidationError("source is required.")
        try:
            validate_public_http_url(value)
        except Exception as e:
            raise serializers.ValidationError(str(e)) from None
        return value


class TwoFactorDeviceSerializer(serializers.ModelSerializer):
    username = serializers.CharField(source="user.username", read_only=True)
    secret = serializers.SerializerMethodField()
    otpauth_url = serializers.SerializerMethodField()

    class Meta:
        model = TwoFactorDevice
        fields = [
            "id",
            "user",
            "username",
            "method",
            "name",
            "confirmed",
            "secret",
            "otpauth_url",
            "last_used_at",
            "created_at",
        ]
        read_only_fields = ["id", "user", "confirmed", "last_used_at", "created_at"]

    def get_secret(self, obj):
        # The secret is only shown during provisioning, before confirmation.
        if obj.method == "totp" and obj.secret and not obj.confirmed:
            return obj.secret
        return None

    def get_otpauth_url(self, obj):
        if obj.method == "totp" and obj.secret and not obj.confirmed:
            from .totp import otpauth_uri

            return otpauth_uri(obj.secret, obj.user.username)
        return None
