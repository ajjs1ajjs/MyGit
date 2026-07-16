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


class TwoFactorDeviceSerializer(serializers.ModelSerializer):
    username = serializers.CharField(source="user.username", read_only=True)

    class Meta:
        model = TwoFactorDevice
        fields = [
            "id",
            "user",
            "username",
            "method",
            "name",
            "confirmed",
            "last_used_at",
            "created_at",
        ]
        read_only_fields = ["id", "user", "confirmed", "last_used_at", "created_at"]
