from django.conf import settings
from django.db import models
from django.utils.translation import gettext_lazy as _

from apps.core.models import BaseModel


class Group(BaseModel):
    name = models.CharField(max_length=255)
    path = models.CharField(max_length=255, unique=True)
    description = models.TextField(blank=True)
    avatar = models.URLField(max_length=512, blank=True)
    parent = models.ForeignKey(
        "self", null=True, blank=True, on_delete=models.CASCADE, related_name="children"
    )

    class Meta:
        db_table = "organizations_group"
        ordering = ["path"]

    def __str__(self):
        return self.path


class GroupMember(BaseModel):
    class Role(models.IntegerChoices):
        GUEST = 10, _("Guest")
        REPORTER = 20, _("Reporter")
        DEVELOPER = 30, _("Developer")
        MAINTAINER = 40, _("Maintainer")
        OWNER = 50, _("Owner")

    group = models.ForeignKey(Group, on_delete=models.CASCADE, related_name="members")
    user = models.ForeignKey(
        settings.AUTH_USER_MODEL, on_delete=models.CASCADE, related_name="group_memberships"
    )
    role = models.IntegerField(choices=Role.choices, default=Role.DEVELOPER)

    class Meta:
        db_table = "organizations_groupmember"
        unique_together = [("group", "user")]

    def __str__(self):
        return f"{self.user.username} @ {self.group.path} ({self.get_role_display()})"


class Team(BaseModel):
    group = models.ForeignKey(Group, on_delete=models.CASCADE, related_name="teams")
    name = models.CharField(max_length=255)
    members = models.ManyToManyField(
        settings.AUTH_USER_MODEL, through="TeamMembership", related_name="teams"
    )

    class Meta:
        db_table = "organizations_team"
        unique_together = [("group", "name")]

    def __str__(self):
        return f"{self.group.path}/{self.name}"


class TeamMembership(BaseModel):
    team = models.ForeignKey(Team, on_delete=models.CASCADE)
    user = models.ForeignKey(settings.AUTH_USER_MODEL, on_delete=models.CASCADE)

    class Meta:
        db_table = "organizations_teammembership"
        unique_together = [("team", "user")]

    def __str__(self):
        return f"{self.user.username} in {self.team}"
