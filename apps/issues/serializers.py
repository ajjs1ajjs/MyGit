from rest_framework import serializers

from .models import Issue, IssueComment, Label, Milestone


class LabelSerializer(serializers.ModelSerializer):
    class Meta:
        model = Label
        fields = ["id", "name", "color", "created_at", "updated_at"]
        read_only_fields = ["id", "created_at", "updated_at"]


class MilestoneSerializer(serializers.ModelSerializer):
    progress = serializers.IntegerField(read_only=True)

    class Meta:
        model = Milestone
        fields = [
            "id",
            "title",
            "description",
            "due_date",
            "is_closed",
            "progress",
            "created_at",
            "updated_at",
        ]
        read_only_fields = ["id", "progress", "created_at", "updated_at"]


class IssueListSerializer(serializers.ModelSerializer):
    author_username = serializers.CharField(source="author.username", read_only=True)
    assignee_username = serializers.CharField(
        source="assignee.username", read_only=True, allow_null=True
    )
    labels = LabelSerializer(many=True, read_only=True)

    class Meta:
        model = Issue
        fields = [
            "id",
            "number",
            "title",
            "state",
            "author",
            "author_username",
            "assignee",
            "assignee_username",
            "milestone",
            "labels",
            "created_at",
            "updated_at",
            "closed_at",
        ]


class IssueDetailSerializer(serializers.ModelSerializer):
    author_username = serializers.CharField(source="author.username", read_only=True)
    assignee_username = serializers.CharField(
        source="assignee.username", read_only=True, allow_null=True
    )
    labels = LabelSerializer(many=True, read_only=True)

    class Meta:
        model = Issue
        fields = [
            "id",
            "number",
            "title",
            "description",
            "state",
            "author",
            "author_username",
            "assignee",
            "assignee_username",
            "milestone",
            "labels",
            "created_at",
            "updated_at",
            "closed_at",
        ]
        read_only_fields = ["id", "number", "author", "created_at", "updated_at", "closed_at"]


class IssueCreateSerializer(serializers.ModelSerializer):
    labels = serializers.ListField(child=serializers.UUIDField(), required=False, write_only=True)

    class Meta:
        model = Issue
        fields = [
            "title",
            "description",
            "assignee",
            "milestone",
            "labels",
        ]

    def create(self, validated_data):
        label_ids = validated_data.pop("labels", [])
        issue = Issue.objects.create(**validated_data)
        if label_ids:
            issue.labels.set(label_ids)
        return issue


class IssueCommentSerializer(serializers.ModelSerializer):
    author_username = serializers.CharField(source="author.username", read_only=True)

    class Meta:
        model = IssueComment
        fields = ["id", "author", "author_username", "body", "created_at", "updated_at"]
        read_only_fields = ["id", "author", "created_at", "updated_at"]
