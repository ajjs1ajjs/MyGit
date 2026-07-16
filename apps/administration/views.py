import subprocess
from pathlib import Path

from django.conf import settings
from django.utils import timezone
from rest_framework import status, viewsets
from rest_framework.decorators import action
from rest_framework.permissions import IsAdminUser, IsAuthenticated
from rest_framework.response import Response

from .models import (
    AuditEvent,
    BackupJob,
    BackupSchedule,
    MirrorTarget,
    RepositoryImportJob,
    TwoFactorDevice,
)
from .serializers import (
    AuditEventSerializer,
    BackupJobSerializer,
    BackupScheduleSerializer,
    MirrorTargetSerializer,
    RepositoryImportJobSerializer,
    TwoFactorDeviceSerializer,
)


def audit(
    actor,
    action: str,
    *,
    target_type: str = "",
    target_id: str = "",
    message: str = "",
    **metadata,
):
    AuditEvent.objects.create(
        actor=actor if getattr(actor, "is_authenticated", False) else None,
        action=action,
        target_type=target_type,
        target_id=str(target_id or ""),
        message=message,
        metadata=metadata,
    )


class AuditEventViewSet(viewsets.ReadOnlyModelViewSet):
    permission_classes = [IsAdminUser]
    serializer_class = AuditEventSerializer
    queryset = AuditEvent.objects.select_related("actor").all()

    def get_queryset(self):
        queryset = super().get_queryset()
        action_name = self.request.query_params.get("action")
        if action_name:
            queryset = queryset.filter(action=action_name)
        return queryset


class BackupScheduleViewSet(viewsets.ModelViewSet):
    permission_classes = [IsAdminUser]
    serializer_class = BackupScheduleSerializer
    queryset = BackupSchedule.objects.all()

    @action(methods=["post"], detail=True)
    def run_now(self, request, pk=None):
        schedule = self.get_object()
        job = BackupJob.objects.create(
            schedule=schedule,
            kind=BackupJob.Kind.CREATE,
            status=BackupJob.Status.RUNNING,
            started_at=timezone.now(),
        )
        script = Path(settings.BASE_DIR) / "scripts" / "mygit-backup"
        command = [str(script), "create", "--keep-local", str(schedule.keep_local)]
        if schedule.encrypt:
            command.append("--encrypt")
        if schedule.upload:
            command.append("--upload")
        try:
            result = subprocess.run(
                command,
                cwd=settings.BASE_DIR,
                capture_output=True,
                text=True,
                timeout=3600,
            )
            job.log = (result.stdout or "") + (result.stderr or "")
            job.status = (
                BackupJob.Status.SUCCESS if result.returncode == 0 else BackupJob.Status.FAILED
            )
            job.finished_at = timezone.now()
            job.archive_path = _archive_from_output(result.stdout)
            job.save()
            schedule.last_run_at = timezone.now()
            schedule.save(update_fields=["last_run_at", "updated_at"])
            audit(
                request.user,
                "backup.run",
                target_type="backup_schedule",
                target_id=schedule.id,
                message=job.status,
            )
            return Response(BackupJobSerializer(job).data, status=status.HTTP_201_CREATED)
        except Exception as exc:
            job.status = BackupJob.Status.FAILED
            job.log = str(exc)
            job.finished_at = timezone.now()
            job.save()
            return Response(
                BackupJobSerializer(job).data,
                status=status.HTTP_500_INTERNAL_SERVER_ERROR,
            )


def _archive_from_output(output: str) -> str:
    for line in output.splitlines():
        if line.startswith("Backup created:"):
            return line.split(":", 1)[1].strip()
    return ""


class BackupJobViewSet(viewsets.ReadOnlyModelViewSet):
    permission_classes = [IsAdminUser]
    serializer_class = BackupJobSerializer
    queryset = BackupJob.objects.select_related("schedule").all()


class MirrorTargetViewSet(viewsets.ModelViewSet):
    permission_classes = [IsAdminUser]
    serializer_class = MirrorTargetSerializer
    queryset = MirrorTarget.objects.all()

    @action(methods=["post"], detail=True)
    def sync(self, request, pk=None):
        mirror = self.get_object()
        script = Path(settings.BASE_DIR) / "scripts" / "mygit-backup"
        command = [str(script), "replicate-repos"]
        if mirror.delete_remote_missing:
            command.append("--delete")
        try:
            result = subprocess.run(
                command,
                cwd=settings.BASE_DIR,
                capture_output=True,
                text=True,
                timeout=3600,
            )
            mirror.last_run_at = timezone.now()
            mirror.last_status = "success" if result.returncode == 0 else "failed"
            mirror.last_error = "" if result.returncode == 0 else result.stderr
            mirror.save()
            audit(
                request.user,
                "mirror.sync",
                target_type="mirror_target",
                target_id=mirror.id,
                message=mirror.last_status,
            )
            return Response(MirrorTargetSerializer(mirror).data)
        except Exception as exc:
            mirror.last_status = "failed"
            mirror.last_error = str(exc)
            mirror.save()
            return Response(
                MirrorTargetSerializer(mirror).data,
                status=status.HTTP_500_INTERNAL_SERVER_ERROR,
            )


class RepositoryImportJobViewSet(viewsets.ModelViewSet):
    permission_classes = [IsAuthenticated]
    serializer_class = RepositoryImportJobSerializer

    def get_queryset(self):
        if self.request.user.is_superuser:
            return RepositoryImportJob.objects.select_related("actor", "repository")
        return RepositoryImportJob.objects.filter(actor=self.request.user).select_related(
            "actor",
            "repository",
        )

    def perform_create(self, serializer):
        job = serializer.save(actor=self.request.user)
        audit(
            self.request.user,
            "repository_import.queued",
            target_type="repository_import_job",
            target_id=job.id,
            message=job.target_path,
        )


class TwoFactorDeviceViewSet(viewsets.ModelViewSet):
    permission_classes = [IsAuthenticated]
    serializer_class = TwoFactorDeviceSerializer

    def get_queryset(self):
        if self.request.user.is_superuser and self.request.query_params.get("all") == "true":
            return TwoFactorDevice.objects.select_related("user")
        return TwoFactorDevice.objects.filter(user=self.request.user)

    def perform_create(self, serializer):
        device = serializer.save(user=self.request.user)
        audit(
            self.request.user,
            "2fa.device_created",
            target_type="two_factor_device",
            target_id=device.id,
            message=device.name,
        )
