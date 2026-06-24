from django.shortcuts import get_object_or_404
from rest_framework import viewsets
from rest_framework.decorators import action
from rest_framework.permissions import IsAuthenticated
from rest_framework.response import Response

from apps.repositories.models import Repository

from .models import Webhook, WebhookDelivery
from .serializers import WebhookDeliverySerializer, WebhookSerializer


class WebhookViewSet(viewsets.ModelViewSet):
    permission_classes = [IsAuthenticated]
    serializer_class = WebhookSerializer
    lookup_field = "id"

    def get_queryset(self):
        repo = self._get_repo()
        return Webhook.objects.filter(repository=repo)

    def _get_repo(self):
        repo_id = self.kwargs.get("project_id", "")
        return get_object_or_404(Repository, id=repo_id)

    def perform_create(self, serializer):
        serializer.save(repository=self._get_repo())

    @action(methods=["get"], detail=True)
    def deliveries(self, request, project_id=None, id=None):
        webhook = self.get_object()
        deliveries = WebhookDelivery.objects.filter(webhook=webhook)
        page = self.paginate_queryset(deliveries)
        if page is not None:
            return self.get_paginated_response(WebhookDeliverySerializer(page, many=True).data)
        return Response(WebhookDeliverySerializer(deliveries, many=True).data)
