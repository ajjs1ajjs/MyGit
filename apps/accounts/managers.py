import re

from django.contrib.auth.base_user import BaseUserManager
from django.utils.translation import gettext_lazy as _


class UserManager(BaseUserManager):
    def create_user(self, username, email="", password=None, **extra_fields):
        if not username:
            raise ValueError(_("The Username field must be set"))

        email = self.normalize_email(email or self.default_email(username))
        user = self.model(email=email, username=username, **extra_fields)
        user.set_password(password)
        user.save(using=self._db)
        return user

    def create_superuser(self, username, email="", password=None, **extra_fields):
        extra_fields.setdefault("is_superuser", True)
        extra_fields.setdefault("is_staff", True)
        extra_fields.setdefault("is_active", True)

        if extra_fields.get("is_superuser") is not True:
            raise ValueError(_("Superuser must have is_superuser=True."))
        if extra_fields.get("is_staff") is not True:
            raise ValueError(_("Superuser must have is_staff=True."))

        return self.create_user(username=username, email=email, password=password, **extra_fields)

    @staticmethod
    def default_email(username: str) -> str:
        local_part = re.sub(r"[^A-Za-z0-9._-]", "_", username).strip("._-") or "user"
        return f"{local_part[:180]}@users.mygit.local"
