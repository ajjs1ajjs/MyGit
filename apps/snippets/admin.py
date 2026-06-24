from django.contrib import admin

from .models import Snippet


@admin.register(Snippet)
class SnippetAdmin(admin.ModelAdmin):
    list_display = ("title", "author", "language", "visibility", "created_at")
    list_filter = ("visibility", "language")
    search_fields = ("title", "description", "code")
