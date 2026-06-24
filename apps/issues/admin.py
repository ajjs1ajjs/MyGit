from django.contrib import admin

from .models import Issue, IssueComment, Label, Milestone


@admin.register(Label)
class LabelAdmin(admin.ModelAdmin):
    list_display = ("repository", "name", "color")
    search_fields = ("name", "repository__path")


@admin.register(Milestone)
class MilestoneAdmin(admin.ModelAdmin):
    list_display = ("repository", "title", "due_date", "is_closed")
    list_filter = ("is_closed",)


@admin.register(Issue)
class IssueAdmin(admin.ModelAdmin):
    list_display = ("repository", "number", "title", "state", "author", "assignee", "created_at")
    list_filter = ("state", "repository")
    search_fields = ("title", "description", "repository__path")


@admin.register(IssueComment)
class IssueCommentAdmin(admin.ModelAdmin):
    list_display = ("issue", "author", "created_at")
    search_fields = ("body",)
