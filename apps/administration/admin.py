from django.contrib import admin

from .models import (
    AuditEvent,
    BackupJob,
    BackupSchedule,
    MirrorTarget,
    RepositoryImportJob,
    TwoFactorDevice,
)


@admin.register(AuditEvent)
class AuditEventAdmin(admin.ModelAdmin):
    list_display = ("action", "actor", "target_type", "target_id", "severity", "created_at")
    list_filter = ("action", "severity", "target_type")
    search_fields = ("action", "message", "target_id", "actor__username")


admin.site.register(BackupSchedule)
admin.site.register(BackupJob)
admin.site.register(MirrorTarget)
admin.site.register(RepositoryImportJob)
admin.site.register(TwoFactorDevice)
