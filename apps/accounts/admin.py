from django.contrib import admin
from django.contrib.auth.admin import UserAdmin as DjangoUserAdmin
from django.utils.translation import gettext_lazy as _

from .models import PersonalAccessToken, SSHKey, User


@admin.register(User)
class UserAdmin(DjangoUserAdmin):
    list_display = ("username", "email", "full_name", "is_active", "is_superuser", "date_joined")
    list_filter = ("is_active", "is_superuser", "date_joined")
    search_fields = ("username", "email", "full_name")
    ordering = ("-date_joined",)

    fieldsets = (
        (None, {"fields": ("email", "username", "password")}),
        (_("Personal info"), {"fields": ("full_name", "bio")}),
        (None, {"fields": ("is_active", "is_staff", "is_superuser", "groups", "user_permissions")}),
        (_("Important dates"), {"fields": ("last_login", "date_joined")}),
    )
    add_fieldsets = (
        (
            None,
            {
                "classes": ("wide",),
                "fields": ("email", "username", "password1", "password2"),
            },
        ),
    )


@admin.register(SSHKey)
class SSHKeyAdmin(admin.ModelAdmin):
    list_display = ("user", "title", "fingerprint", "created_at")
    search_fields = ("user__username", "title", "fingerprint")
    readonly_fields = ("fingerprint",)


@admin.register(PersonalAccessToken)
class PersonalAccessTokenAdmin(admin.ModelAdmin):
    list_display = ("user", "name", "last_used_at", "expires_at", "created_at")
    search_fields = ("user__username", "name")
    readonly_fields = ("token_hash",)
