from rest_framework import serializers

from .models import Group, GroupMember, Team, TeamMembership


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


class GroupMemberSerializer(serializers.ModelSerializer):
    username = serializers.CharField(source="user.username", read_only=True)
    email = serializers.EmailField(source="user.email", read_only=True)

    class Meta:
        model = GroupMember
        fields = ["id", "user", "username", "email", "role", "created_at"]
        read_only_fields = ["id", "created_at"]


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
