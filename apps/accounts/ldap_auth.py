import logging

from django.conf import settings
from django.contrib.auth import get_user_model
from django.db import IntegrityError

logger = logging.getLogger("mygit")


def authenticate_ldap_user(username: str, password: str):
    if not getattr(settings, "MYGIT_LDAP_ENABLED", False):
        return None
    if not username or not password or not getattr(settings, "MYGIT_LDAP_SERVER_URI", ""):
        return None
    if not getattr(settings, "MYGIT_LDAP_USER_SEARCH_BASE", ""):
        return None

    try:
        from ldap3 import ALL, Connection, Server
        from ldap3.core.exceptions import LDAPException
        from ldap3.utils.conv import escape_filter_chars
    except ImportError:
        logger.warning("LDAP login requested but ldap3 is not installed.")
        return None

    try:
        server = Server(settings.MYGIT_LDAP_SERVER_URI, get_info=ALL)
        bind_user = settings.MYGIT_LDAP_BIND_DN or None
        bind_password = settings.MYGIT_LDAP_BIND_PASSWORD or None
        conn = Connection(server, user=bind_user, password=bind_password, auto_bind=False)
        try:
            if settings.MYGIT_LDAP_START_TLS:
                conn.open()
                conn.start_tls()
            if not conn.bind():
                return None

            safe_username = escape_filter_chars(username)
            search_filter = settings.MYGIT_LDAP_USER_SEARCH_FILTER.format(
                username=safe_username
            )
            attrs = [
                settings.MYGIT_LDAP_USERNAME_ATTR,
                settings.MYGIT_LDAP_EMAIL_ATTR,
                settings.MYGIT_LDAP_FULL_NAME_ATTR,
            ]
            if not conn.search(
                settings.MYGIT_LDAP_USER_SEARCH_BASE,
                search_filter,
                attributes=list({attr for attr in attrs if attr}),
            ):
                return None
            if not conn.entries:
                return None

            entry = conn.entries[0]
            user_dn = entry.entry_dn
            profile = _extract_profile(entry, username)
        finally:
            conn.unbind()

        user_conn = Connection(server, user=user_dn, password=password, auto_bind=False)
        if settings.MYGIT_LDAP_START_TLS:
            user_conn.open()
            user_conn.start_tls()
        if not user_conn.bind():
            return None
        user_conn.unbind()
        return _get_or_create_ldap_user(profile)
    except LDAPException as e:
        logger.info("LDAP authentication failed for %s: %s", username, e)
        return None


def _extract_profile(entry, fallback_username: str) -> dict:
    def attr_value(name: str) -> str:
        if not name or not hasattr(entry, name):
            return ""
        value = getattr(entry, name).value
        if isinstance(value, list):
            return str(value[0]) if value else ""
        return str(value or "")

    username = attr_value(settings.MYGIT_LDAP_USERNAME_ATTR) or fallback_username
    email = attr_value(settings.MYGIT_LDAP_EMAIL_ATTR)
    full_name = attr_value(settings.MYGIT_LDAP_FULL_NAME_ATTR)
    return {"username": username, "email": email, "full_name": full_name}


def _get_or_create_ldap_user(profile: dict):
    user_model = get_user_model()
    username = profile["username"]
    email = profile["email"] or user_model.objects.default_email(username)
    defaults = {
        "email": email,
        "full_name": profile["full_name"],
        "is_active": True,
    }

    try:
        user, created = user_model.objects.get_or_create(username=username, defaults=defaults)
    except IntegrityError:
        defaults["email"] = user_model.objects.default_email(username)
        user, created = user_model.objects.get_or_create(username=username, defaults=defaults)

    if created:
        user.set_unusable_password()
        user.save(update_fields=["password"])
    else:
        changed_fields = []
        for field in ("email", "full_name"):
            value = defaults[field]
            if value and getattr(user, field) != value:
                setattr(user, field, value)
                changed_fields.append(field)
        if changed_fields:
            user.save(update_fields=changed_fields)

    return user if user.is_active else None
