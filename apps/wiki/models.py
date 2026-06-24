from django.conf import settings
from django.db import models

from apps.core.models import BaseModel


class WikiPage(BaseModel):
    repository = models.ForeignKey(
        "repositories.Repository", on_delete=models.CASCADE, related_name="wiki_pages"
    )
    author = models.ForeignKey(
        settings.AUTH_USER_MODEL, on_delete=models.CASCADE, related_name="wiki_pages"
    )
    slug = models.SlugField(max_length=255)
    title = models.CharField(max_length=255)
    content = models.TextField(blank=True)

    class Meta:
        db_table = "wiki_page"
        unique_together = [("repository", "slug")]
        ordering = ["slug"]

    def __str__(self):
        return f"{self.repository.path}/wiki/{self.slug}"
