from rest_framework import serializers

from .models import WikiPage


class WikiPageSerializer(serializers.ModelSerializer):
    author_username = serializers.CharField(source="author.username", read_only=True)

    class Meta:
        model = WikiPage
        fields = [
            "id",
            "slug",
            "title",
            "content",
            "author",
            "author_username",
            "created_at",
            "updated_at",
        ]
        read_only_fields = ["id", "author", "created_at", "updated_at"]
