from django.contrib import admin

from .models import WikiPage


@admin.register(WikiPage)
class WikiPageAdmin(admin.ModelAdmin):
    list_display = ("repository", "slug", "title", "author", "created_at", "updated_at")
    search_fields = ("title", "slug", "content")
