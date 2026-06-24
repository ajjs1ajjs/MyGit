import hashlib

from django.contrib.auth.models import AbstractBaseUser, PermissionsMixin
from django.contrib.auth.validators import UnicodeUsernameValidator
from django.db import models
from django.utils import timezone
from django.utils.translation import gettext_lazy as _

from apps.core.models import BaseModel

from .managers import UserManager


class User(AbstractBaseUser, PermissionsMixin, BaseModel):
    username_validator = UnicodeUsernameValidator()

    username = models.CharField(
        _("username"),
        max_length=150,
        unique=True,
        validators=[username_validator],
    )
    email = models.EmailField(_("email address"), unique=True)
    full_name = models.CharField(_("full name"), max_length=255, blank=True)
    bio = models.TextField(_("bio"), blank=True)
    avatar = models.URLField(max_length=512, blank=True)

    is_active = models.BooleanField(default=True)
    is_superuser = models.BooleanField(default=False)
    is_staff = models.BooleanField(default=False)
    must_change_password = models.BooleanField(default=False)

    date_joined = models.DateTimeField(default=timezone.now)
    last_login = models.DateTimeField(null=True, blank=True)

    objects = UserManager()

    USERNAME_FIELD = "username"
    REQUIRED_FIELDS = []

    class Meta:
        db_table = "accounts_user"
        verbose_name = _("user")
        verbose_name_plural = _("users")
        ordering = ["-date_joined"]

    def __str__(self):
        return self.username

    def clean(self):
        super().clean()
        self.email = self.__class__.objects.normalize_email(self.email)


class SSHKey(BaseModel):
    user = models.ForeignKey(User, on_delete=models.CASCADE, related_name="ssh_keys")
    title = models.CharField(max_length=255)
    public_key = models.TextField(unique=True)
    fingerprint = models.CharField(max_length=64, unique=True, editable=False)

    class Meta:
        db_table = "accounts_sshkey"
        verbose_name = _("SSH key")
        verbose_name_plural = _("SSH keys")
        ordering = ["-created_at"]

    def __str__(self):
        return f"{self.user.username}: {self.title}"

    def save(self, *args, **kwargs):
        if not self.fingerprint:
            self.fingerprint = self._compute_fingerprint()
        super().save(*args, **kwargs)

    def _compute_fingerprint(self) -> str:
        key_parts = self.public_key.strip().split()
        if len(key_parts) >= 2:
            import base64

            try:
                key_bytes = base64.b64decode(key_parts[1])
                return hashlib.sha256(key_bytes).hexdigest()
            except Exception:
                pass
        return hashlib.sha256(self.public_key.encode()).hexdigest()


class PersonalAccessToken(BaseModel):
    user = models.ForeignKey(User, on_delete=models.CASCADE, related_name="tokens")
    name = models.CharField(max_length=255)
    token_hash = models.CharField(max_length=128, unique=True)
    scopes = models.JSONField(default=list)
    last_used_at = models.DateTimeField(null=True, blank=True)
    expires_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "accounts_token"
        verbose_name = _("personal access token")
        verbose_name_plural = _("personal access tokens")
        ordering = ["-created_at"]

    def __str__(self):
        return f"{self.user.username}: {self.name}"

    @property
    def is_expired(self):
        if self.expires_at is None:
            return False
        return timezone.now() > self.expires_at


class IntegrationToken(BaseModel):
    class Provider(models.TextChoices):
        GITHUB = "github", "GitHub"
        GITLAB = "gitlab", "GitLab"

    user = models.ForeignKey(User, on_delete=models.CASCADE, related_name="integration_tokens")
    provider = models.CharField(max_length=50, choices=Provider.choices)
    token = models.CharField(max_length=255)

    class Meta:
        db_table = "accounts_integrationtoken"
        unique_together = (("user", "provider"),)
        verbose_name = _("integration token")
        verbose_name_plural = _("integration tokens")
        ordering = ["-created_at"]

    def __str__(self):
        return f"{self.user.username} - {self.provider}"

