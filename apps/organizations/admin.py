from django.contrib import admin

from .models import Group, GroupMember, Team, TeamMembership


@admin.register(Group)
class GroupAdmin(admin.ModelAdmin):
    list_display = ("name", "path", "parent", "created_at")
    search_fields = ("name", "path")
    prepopulated_fields = {"path": ("name",)}


@admin.register(GroupMember)
class GroupMemberAdmin(admin.ModelAdmin):
    list_display = ("user", "group", "role", "created_at")
    list_filter = ("role",)
    search_fields = ("user__username", "group__path")


@admin.register(Team)
class TeamAdmin(admin.ModelAdmin):
    list_display = ("name", "group")
    search_fields = ("name", "group__path")


@admin.register(TeamMembership)
class TeamMembershipAdmin(admin.ModelAdmin):
    list_display = ("user", "team")
    search_fields = ("user__username", "team__name")
