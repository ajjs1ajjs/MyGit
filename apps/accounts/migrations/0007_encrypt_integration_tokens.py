"""Encrypt integration tokens that were stored in plaintext before the
token-encryption-on-save change."""

import base64
import hashlib

from django.db import migrations


def encrypt_plaintext(apps, schema_editor):
    IntegrationToken = apps.get_model("accounts", "IntegrationToken")
    from cryptography.fernet import Fernet
    from django.conf import settings

    raw = getattr(settings, "ENCRYPTION_KEY", "") or settings.SECRET_KEY
    key = base64.urlsafe_b64encode(hashlib.sha256(raw.encode()).digest())
    fernet = Fernet(key)

    for token in IntegrationToken.objects.exclude(token__startswith="gAAAAA").iterator():
        if token.token:
            token.token = fernet.encrypt(token.token.encode()).decode()
            token.save(update_fields=["token"])


def decrypt_plaintext(apps, schema_editor):
    IntegrationToken = apps.get_model("accounts", "IntegrationToken")
    from cryptography.fernet import Fernet
    from django.conf import settings

    raw = getattr(settings, "ENCRYPTION_KEY", "") or settings.SECRET_KEY
    key = base64.urlsafe_b64encode(hashlib.sha256(raw.encode()).digest())
    fernet = Fernet(key)

    for token in IntegrationToken.objects.filter(token__startswith="gAAAAA").iterator():
        try:
            token.token = fernet.decrypt(token.token.encode()).decode()
            token.save(update_fields=["token"])
        except Exception:
            pass


class Migration(migrations.Migration):

    dependencies = [
        ("accounts", "0006_sshkey_is_active"),
    ]

    operations = [
        migrations.RunPython(encrypt_plaintext, decrypt_plaintext),
    ]
