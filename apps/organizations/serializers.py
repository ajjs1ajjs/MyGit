import re

from rest_framework import serializers

from .models import Group, GroupMember, Team, TeamMembership

_GROUP_PATH_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,254}$")


def validate_group_path(value):
    value = (value or "").strip()
    if not _GROUP_PATH_RE.fullmatch(value) or value in {".", ".."}:
        raise serializers.ValidationError(
            "Use only letters, numbers, dots, underscores and hyphens."
        )
    return value


class GroupListSerializer(serializers.ModelSerializer):
    member_count = serializers.SerializerMethodField()
    project_count = serializers.SerializerMethodField()

    class Meta:
        model = Group
        fields = [
            "id",
            "name",
            "path",
            "description",
            "avatar",
            "parent",
            "member_count",
            "project_count",
            "created_at",
            "updated_at",
        ]
        read_only_fields = ["id", "created_at", "updated_at"]

    def validate_path(self, value):
        return validate_group_path(value)

    def get_member_count(self, obj):
        return obj.members.count()

    def get_project_count(self, obj):
        from apps.repositories.models import Repository

        return Repository.objects.filter(owner_type="organization", owner_id=obj.id).count()


class GroupDetailSerializer(serializers.ModelSerializer):
    class Meta:
        model = Group
        fields = "__all__"
        read_only_fields = ["id", "created_at", "updated_at"]

    def validate_path(self, value):
        return validate_group_path(value)


class GroupMemberSerializer(serializers.ModelSerializer):
    username = serializers.CharField(source="user.username", read_only=True)
    email = serializers.EmailField(source="user.email", read_only=True)

    class Meta:
        model = GroupMember
        fields = ["id", "user", "username", "email", "role", "created_at"]
        read_only_fields = ["id", "created_at"]

    def to_representation(self, instance):
        data = super().to_representation(instance)
        request = self.context.get("request")
        viewer = getattr(request, "user", None)
        if not viewer or not getattr(viewer, "is_authenticated", False):
            data.pop("email", None)
            return data
        is_member = GroupMember.objects.filter(group=instance.group, user=viewer).exists()
        if not (viewer.is_superuser or is_member):
            data.pop("email", None)
        return data


class TeamSerializer(serializers.ModelSerializer):
    class Meta:
        model = Team
        fields = ["id", "name", "members"]
        read_only_fields = ["id"]


class TeamMembershipSerializer(serializers.ModelSerializer):
    username = serializers.CharField(source="user.username", read_only=True)

    class Meta:
        model = TeamMembership
        fields = ["id", "user", "username", "team", "created_at"]
        read_only_fields = ["id", "created_at"]
