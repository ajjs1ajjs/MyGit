from django.conf import settings
from django.db import models

from apps.core.models import BaseModel


class Snippet(BaseModel):
    author = models.ForeignKey(
        settings.AUTH_USER_MODEL, on_delete=models.CASCADE, related_name="snippets"
    )
    title = models.CharField(max_length=255)
    description = models.TextField(blank=True)
    code = models.TextField()
    language = models.CharField(max_length=50, blank=True)
    visibility = models.CharField(
        max_length=20,
        choices=[("public", "Public"), ("private", "Private")],
        default="public",
    )

    class Meta:
        db_table = "snippets_snippet"
        ordering = ["-created_at"]

    def __str__(self):
        return self.title
