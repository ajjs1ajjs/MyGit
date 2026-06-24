from django.contrib import admin

from .models import Job, Pipeline


@admin.register(Pipeline)
class PipelineAdmin(admin.ModelAdmin):
    list_display = ("repository", "ref", "sha", "status", "created_at")
    list_filter = ("status",)


@admin.register(Job)
class JobAdmin(admin.ModelAdmin):
    list_display = ("pipeline", "name", "stage", "status", "started_at", "finished_at")
    list_filter = ("status", "stage")
