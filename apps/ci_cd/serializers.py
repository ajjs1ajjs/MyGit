from rest_framework import serializers

from .models import Job, Pipeline


class JobSerializer(serializers.ModelSerializer):
    class Meta:
        model = Job
        fields = [
            "id",
            "pipeline",
            "name",
            "stage",
            "status",
            "log",
            "artifacts",
            "runner_id",
            "started_at",
            "finished_at",
            "created_at",
            "updated_at",
        ]
        read_only_fields = [
            "id",
            "pipeline",
            "log",
            "artifacts",
            "started_at",
            "finished_at",
            "created_at",
            "updated_at",
        ]


class JobLogUpdateSerializer(serializers.Serializer):
    log = serializers.CharField(required=False, allow_blank=True)
    status = serializers.ChoiceField(choices=["running", "success", "failed"], required=False)


class PipelineSerializer(serializers.ModelSerializer):
    jobs = JobSerializer(many=True, read_only=True)
    author_username = serializers.CharField(source="author.username", read_only=True)

    class Meta:
        model = Pipeline
        fields = [
            "id",
            "repository",
            "ref",
            "sha",
            "status",
            "stages",
            "jobs",
            "author",
            "author_username",
            "started_at",
            "finished_at",
            "created_at",
            "updated_at",
        ]
        read_only_fields = [
            "id",
            "status",
            "stages",
            "jobs",
            "author",
            "started_at",
            "finished_at",
            "created_at",
            "updated_at",
        ]


class PipelineCreateSerializer(serializers.Serializer):
    ref = serializers.CharField()
    sha = serializers.CharField()
