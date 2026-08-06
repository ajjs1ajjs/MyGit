from django.db.models import Q
from rest_framework import viewsets
from rest_framework.decorators import action
from rest_framework.permissions import IsAuthenticated
from rest_framework.response import Response

from .models import Snippet
from .serializers import SnippetSerializer


class SnippetViewSet(viewsets.ModelViewSet):
    serializer_class = SnippetSerializer
    permission_classes = [IsAuthenticated]

    def get_queryset(self):
        user = self.request.user
        if self.action in ("update", "partial_update", "destroy"):
            return Snippet.objects.filter(author=user)
        return Snippet.objects.filter(Q(visibility="public") | Q(author=user))

    def perform_create(self, serializer):
        serializer.save(author=self.request.user)

    @action(methods=["get"], detail=False)
    def mine(self, request):
        snippets = Snippet.objects.filter(author=request.user)
        return Response(SnippetSerializer(snippets, many=True).data)
