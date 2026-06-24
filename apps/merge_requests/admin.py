from django.contrib import admin

from .models import MergeRequest, MergeRequestComment, MergeRequestReview


@admin.register(MergeRequest)
class MergeRequestAdmin(admin.ModelAdmin):
    list_display = (
        "repository",
        "number",
        "title",
        "state",
        "author",
        "assignee",
        "source_branch",
        "target_branch",
    )
    list_filter = ("state",)
    search_fields = ("title", "description", "repository__path")


@admin.register(MergeRequestComment)
class MergeRequestCommentAdmin(admin.ModelAdmin):
    list_display = ("merge_request", "author", "created_at")


@admin.register(MergeRequestReview)
class MergeRequestReviewAdmin(admin.ModelAdmin):
    list_display = ("merge_request", "author", "approved", "created_at")
