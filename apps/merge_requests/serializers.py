from rest_framework import serializers

from .models import MergeRequest, MergeRequestComment, MergeRequestReview


class MergeRequestListSerializer(serializers.ModelSerializer):
    author_username = serializers.CharField(source="author.username", read_only=True)
    assignee_username = serializers.CharField(
        source="assignee.username", read_only=True, allow_null=True
    )
    latest_pipeline_status = serializers.SerializerMethodField()

    class Meta:
        model = MergeRequest
        fields = [
            "id",
            "number",
            "title",
            "state",
            "source_branch",
            "target_branch",
            "author",
            "author_username",
            "assignee",
            "assignee_username",
            "latest_pipeline_status",
            "created_at",
            "updated_at",
            "merged_at",
        ]

    def get_latest_pipeline_status(self, obj):
        pipeline = obj.repository.pipelines.filter(ref=obj.source_branch).order_by("-created_at").first()
        if not pipeline:
            return None
        return {"id": str(pipeline.id), "status": pipeline.status}


class MergeRequestDetailSerializer(serializers.ModelSerializer):
    author_username = serializers.CharField(source="author.username", read_only=True)
    assignee_username = serializers.CharField(
        source="assignee.username", read_only=True, allow_null=True
    )
    merged_by_username = serializers.CharField(
        source="merged_by.username", read_only=True, allow_null=True
    )
    latest_pipeline_status = serializers.SerializerMethodField()

    class Meta:
        model = MergeRequest
        fields = [
            "id",
            "number",
            "title",
            "description",
            "state",
            "source_branch",
            "target_branch",
            "author",
            "author_username",
            "assignee",
            "assignee_username",
            "merged_by",
            "merged_by_username",
            "merge_commit_sha",
            "merged_at",
            "closes_issues",
            "latest_pipeline_status",
            "created_at",
            "updated_at",
        ]
        read_only_fields = [
            "id",
            "number",
            "author",
            "state",
            "merge_commit_sha",
            "merged_at",
            "merged_by",
            "created_at",
            "updated_at",
        ]

    def get_latest_pipeline_status(self, obj):
        pipeline = obj.repository.pipelines.filter(ref=obj.source_branch).order_by("-created_at").first()
        if not pipeline:
            return None
        return {"id": str(pipeline.id), "status": pipeline.status}


class MergeRequestCreateSerializer(serializers.ModelSerializer):
    closes_issues = serializers.ListField(
        child=serializers.IntegerField(), required=False, write_only=True
    )

    class Meta:
        model = MergeRequest
        fields = [
            "title",
            "description",
            "source_branch",
            "target_branch",
            "assignee",
            "closes_issues",
        ]

    def create(self, validated_data):
        close_issue_numbers = validated_data.pop("closes_issues", [])
        mr = MergeRequest.objects.create(**validated_data)
        if close_issue_numbers:
            from apps.issues.models import Issue

            issues = Issue.objects.filter(repository=mr.repository, number__in=close_issue_numbers)
            mr.closes_issues.add(*issues)
        return mr


class MergeRequestCommentSerializer(serializers.ModelSerializer):
    author_username = serializers.CharField(source="author.username", read_only=True)

    class Meta:
        model = MergeRequestComment
        fields = ["id", "author", "author_username", "body", "created_at", "updated_at"]
        read_only_fields = ["id", "author", "created_at", "updated_at"]


class MergeRequestReviewSerializer(serializers.ModelSerializer):
    author_username = serializers.CharField(source="author.username", read_only=True)

    class Meta:
        model = MergeRequestReview
        fields = ["id", "author", "author_username", "body", "approved", "created_at"]
        read_only_fields = ["id", "author", "created_at"]
