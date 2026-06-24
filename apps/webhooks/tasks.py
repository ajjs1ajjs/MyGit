import json
import logging

import requests
from celery import shared_task
from django.utils import timezone

from .models import WebhookDelivery

logger = logging.getLogger("mygit")


@shared_task(
    bind=True,
    max_retries=3,
    default_retry_delay=30,
    autoretry_for=(requests.RequestException,),
)
def deliver_webhook(self, delivery_id: str):
    try:
        delivery = WebhookDelivery.objects.get(id=delivery_id)
    except WebhookDelivery.DoesNotExist:
        return

    webhook = delivery.webhook
    payload_bytes = json.dumps(delivery.payload).encode()
    signature = webhook.sign_payload(payload_bytes)

    headers = {
        "Content-Type": "application/json",
        "User-Agent": "MyGit-Webhook/1.0",
        "X-MyGit-Event": delivery.event,
    }
    if signature:
        headers["X-MyGit-Signature"] = f"sha256={signature}"

    try:
        resp = requests.post(
            webhook.url,
            data=payload_bytes,
            headers=headers,
            timeout=30,
        )
        delivery.response_code = resp.status_code
        delivery.response_body = resp.text[:4096]
        delivery.status = (
            WebhookDelivery.Status.SUCCESS if resp.ok else WebhookDelivery.Status.FAILED
        )
    except requests.RequestException as e:
        delivery.retry_count += 1
        delivery.response_body = str(e)[:4096]
        delivery.status = WebhookDelivery.Status.FAILED
        raise

    delivery.delivered_at = timezone.now()
    delivery.save(
        update_fields=[
            "status",
            "response_code",
            "response_body",
            "retry_count",
            "delivered_at",
        ]
    )



