from django.contrib import admin

from .models import Repository, RepositoryAccess


@admin.register(Repository)
class RepositoryAdmin(admin.ModelAdmin):
    list_display = (
        "path",
        "owner_type",
        "owner_id",
        "visibility",
        "default_branch",
        "is_archived",
        "created_at",
    )
    list_filter = ("visibility", "is_archived", "is_fork", "owner_type")
    search_fields = ("path", "name", "description")
    readonly_fields = ("size_kb",)


@admin.register(RepositoryAccess)
class RepositoryAccessAdmin(admin.ModelAdmin):
    list_display = ("user", "repository", "role", "created_at")
    list_filter = ("role",)
    search_fields = ("user__username", "repository__path")
