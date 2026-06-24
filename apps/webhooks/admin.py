from django.contrib import admin

from .models import Webhook, WebhookDelivery


@admin.register(Webhook)
class WebhookAdmin(admin.ModelAdmin):
    list_display = ("repository", "url", "is_active", "created_at")
    list_filter = ("is_active",)
    search_fields = ("url",)


@admin.register(WebhookDelivery)
class WebhookDeliveryAdmin(admin.ModelAdmin):
    list_display = ("webhook", "event", "status", "response_code", "retry_count", "created_at")
    list_filter = ("status",)
