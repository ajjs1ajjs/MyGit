from django.conf import settings
from django.db import models
from django.utils.translation import gettext_lazy as _

from apps.core.models import BaseModel


class MergeRequest(BaseModel):
    class State(models.TextChoices):
        OPEN = "open", _("Open")
        DRAFT = "draft", _("Draft")
        MERGED = "merged", _("Merged")
        CLOSED = "closed", _("Closed")

    repository = models.ForeignKey(
        "repositories.Repository", on_delete=models.CASCADE, related_name="merge_requests"
    )
    author = models.ForeignKey(
        settings.AUTH_USER_MODEL, on_delete=models.CASCADE, related_name="authored_mrs"
    )
    assignee = models.ForeignKey(
        settings.AUTH_USER_MODEL,
        null=True,
        blank=True,
        on_delete=models.SET_NULL,
        related_name="assigned_mrs",
    )
    source_branch = models.CharField(max_length=255)
    target_branch = models.CharField(max_length=255)
    title = models.CharField(max_length=512)
    description = models.TextField(blank=True)
    state = models.CharField(max_length=20, choices=State.choices, default=State.OPEN)
    number = models.IntegerField()
    merge_commit_sha = models.CharField(max_length=40, null=True, blank=True)
    merged_at = models.DateTimeField(null=True, blank=True)
    merged_by = models.ForeignKey(
        settings.AUTH_USER_MODEL,
        null=True,
        blank=True,
        on_delete=models.SET_NULL,
        related_name="merged_mrs",
    )
    closes_issues = models.ManyToManyField("issues.Issue", blank=True, related_name="closing_mrs")

    class Meta:
        db_table = "merge_requests_mergerequest"
        unique_together = [("repository", "number")]
        ordering = ["-created_at"]

    def __str__(self):
        return f"{self.repository.path}!{self.number}: {self.title}"

    def save(self, *args, **kwargs):
        if not self.number:
            last = MergeRequest.objects.filter(repository=self.repository).aggregate(
                last_num=models.Max("number")
            )["last_num"]
            self.number = (last or 0) + 1
        super().save(*args, **kwargs)


class MergeRequestComment(BaseModel):
    merge_request = models.ForeignKey(
        MergeRequest, on_delete=models.CASCADE, related_name="comments"
    )
    author = models.ForeignKey(
        settings.AUTH_USER_MODEL, on_delete=models.CASCADE, related_name="mr_comments"
    )
    body = models.TextField()

    class Meta:
        db_table = "merge_requests_comment"
        ordering = ["created_at"]

    def __str__(self):
        return f"{self.merge_request} - {self.author.username}"


class MergeRequestReview(BaseModel):
    merge_request = models.ForeignKey(
        MergeRequest, on_delete=models.CASCADE, related_name="reviews"
    )
    author = models.ForeignKey(
        settings.AUTH_USER_MODEL, on_delete=models.CASCADE, related_name="mr_reviews"
    )
    body = models.TextField(blank=True)
    approved = models.BooleanField(default=False)

    class Meta:
        db_table = "merge_requests_review"
        unique_together = [("merge_request", "author")]

    def __str__(self):
        status = "approved" if self.approved else "commented"
        return f"{self.merge_request} - {self.author.username} ({status})"
