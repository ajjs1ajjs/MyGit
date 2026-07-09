import logging

from celery import shared_task
from django.conf import settings
from django.contrib.auth import get_user_model
from django.core.mail import send_mail
from django.template.loader import render_to_string

logger = logging.getLogger("mygit")
User = get_user_model()


@shared_task
def send_password_reset_email(context: dict):
    user_id = context.get("user_id")
    if not user_id:
        return
    try:
        user = User.objects.get(id=user_id)
    except User.DoesNotExist:
        return

    subject = render_to_string("email/password_reset_subject.txt", context)
    body = render_to_string("email/password_reset_body.txt", context)

    send_mail(
        subject=subject.strip(),
        message=body,
        from_email=None,
        recipient_list=[user.email],
        fail_silently=True,
    )
