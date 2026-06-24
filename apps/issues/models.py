from django.conf import settings
from django.db import models
from django.utils.translation import gettext_lazy as _

from apps.core.models import BaseModel


class Label(BaseModel):
    repository = models.ForeignKey(
        "repositories.Repository", on_delete=models.CASCADE, related_name="labels"
    )
    name = models.CharField(max_length=255)
    color = models.CharField(max_length=7, default="#000000")

    class Meta:
        db_table = "issues_label"
        unique_together = [("repository", "name")]
        ordering = ["name"]

    def __str__(self):
        return f"{self.repository.path}: {self.name}"


class Milestone(BaseModel):
    repository = models.ForeignKey(
        "repositories.Repository", on_delete=models.CASCADE, related_name="milestones"
    )
    title = models.CharField(max_length=255)
    description = models.TextField(blank=True)
    due_date = models.DateTimeField(null=True, blank=True)
    is_closed = models.BooleanField(default=False)

    class Meta:
        db_table = "issues_milestone"
        unique_together = [("repository", "title")]
        ordering = ["-created_at"]

    def __str__(self):
        return f"{self.repository.path}: {self.title}"

    @property
    def progress(self):
        total = self.issues.count()
        if total == 0:
            return 0
        closed = self.issues.filter(state="closed").count()
        return int(closed / total * 100)


class Issue(BaseModel):
    class State(models.TextChoices):
        OPEN = "open", _("Open")
        CLOSED = "closed", _("Closed")

    repository = models.ForeignKey(
        "repositories.Repository", on_delete=models.CASCADE, related_name="issues"
    )
    author = models.ForeignKey(
        settings.AUTH_USER_MODEL, on_delete=models.CASCADE, related_name="authored_issues"
    )
    assignee = models.ForeignKey(
        settings.AUTH_USER_MODEL,
        null=True,
        blank=True,
        on_delete=models.SET_NULL,
        related_name="assigned_issues",
    )
    milestone = models.ForeignKey(
        Milestone, null=True, blank=True, on_delete=models.SET_NULL, related_name="issues"
    )
    title = models.CharField(max_length=512)
    description = models.TextField(blank=True)
    state = models.CharField(max_length=20, choices=State.choices, default=State.OPEN)
    number = models.IntegerField()
    labels = models.ManyToManyField(Label, related_name="issues", blank=True)
    closed_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "issues_issue"
        unique_together = [("repository", "number")]
        ordering = ["-created_at"]

    def __str__(self):
        return f"{self.repository.path}#{self.number}: {self.title}"

    def save(self, *args, **kwargs):
        if not self.number:
            last = Issue.objects.filter(repository=self.repository).aggregate(
                last_num=models.Max("number")
            )["last_num"]
            self.number = (last or 0) + 1
        super().save(*args, **kwargs)


class IssueComment(BaseModel):
    issue = models.ForeignKey(Issue, on_delete=models.CASCADE, related_name="comments")
    author = models.ForeignKey(
        settings.AUTH_USER_MODEL, on_delete=models.CASCADE, related_name="issue_comments"
    )
    body = models.TextField()

    class Meta:
        db_table = "issues_comment"
        ordering = ["created_at"]

    def __str__(self):
        return f"{self.issue} - {self.author.username}"
