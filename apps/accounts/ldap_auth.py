import logging

from django.conf import settings
from django.contrib.auth import get_user_model
from django.db import IntegrityError

logger = logging.getLogger("mygit")


def get_local_domain():
    import os
    import socket

    # 1. Check environment variables
    domain = os.environ.get("USERDOMAIN") or os.environ.get("LOGONSERVER")
    if domain:
        domain = domain.lstrip("\\").strip().lower()
        if domain:
            return domain

    # 2. Check FQDN
    fqdn = socket.getfqdn()
    if "." in fqdn:
        parts = fqdn.split(".", 1)
        if parts[1] and parts[1].strip() not in ("localdomain", "localhost"):
            return parts[1].lower().strip()

    # 3. Read /etc/resolv.conf
    if os.path.exists("/etc/resolv.conf"):
        try:
            with open("/etc/resolv.conf", "r") as f:
                for line in f:
                    line = line.strip()
                    if line.startswith("search ") or line.startswith("domain "):
                        parts = line.split()
                        if len(parts) > 1 and parts[1].strip() not in ("localdomain", "localhost"):
                            return parts[1].lower().strip()
        except Exception:
            pass
    return None


def find_ldap_server(domain: str):
    import socket
    for host in (domain, f"ldap.{domain}", f"dc.{domain}", f"ad.{domain}"):
        try:
            ip = socket.gethostbyname(host)
            s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
            s.settimeout(1.5)
            s.connect((ip, 389))
            s.close()
            return host
        except Exception:
            pass
    return None


def authenticate_ldap_user(username: str, password: str):
    if not username or not password:
        return None

    # LDAP is strictly opt-in. This also prevents the auto-discovery network
    # scans (DNS + TCP connect to port 389) that used to run on every failed
    # local login when LDAP was not configured at all.
    if not getattr(settings, "MYGIT_LDAP_ENABLED", False):
        return None

    try:
        from ldap3 import ALL, Connection, Server
        from ldap3.core.exceptions import LDAPException
        from ldap3.utils.conv import escape_filter_chars
    except ImportError:
        logger.warning("LDAP login requested but ldap3 is not installed.")
        return None

    # Option A: Explicitly configured
    if getattr(settings, "MYGIT_LDAP_SERVER_URI", "") and getattr(settings, "MYGIT_LDAP_USER_SEARCH_BASE", ""):
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

    # Option B: Auto-discovery and Direct Bind (fallback)
    domain_name = get_local_domain()
    if not domain_name:
        return None

    ldap_server = find_ldap_server(domain_name)
    if not ldap_server:
        return None

    search_base = ",".join(f"dc={part}" for part in domain_name.split("."))

    user_dns = [
        f"{username}@{domain_name}",
        f"{domain_name.split('.')[0].upper()}\\{username}"
    ]

    server_obj = Server(f"ldap://{ldap_server}:389", get_info=ALL)
    bound_conn = None
    for user_dn in user_dns:
        try:
            conn = Connection(server_obj, user=user_dn, password=password, auto_bind=False)
            if conn.bind():
                bound_conn = conn
                break
        except Exception:
            pass

    if not bound_conn:
        return None

    email = ""
    full_name = username
    try:
        safe_username = escape_filter_chars(username)
        search_filter = f"(|(sAMAccountName={safe_username})(uid={safe_username}))"
        attrs = ["mail", "cn", "displayName", "givenName", "sn"]
        if bound_conn.search(search_base, search_filter, attributes=attrs) and bound_conn.entries:
            entry = bound_conn.entries[0]
            def get_attr(name):
                if hasattr(entry, name):
                    val = getattr(entry, name).value
                    if isinstance(val, list):
                        return str(val[0]) if val else ""
                    return str(val or "")
                return ""

            email = get_attr("mail")
            full_name = get_attr("displayName") or get_attr("cn")
            if not full_name:
                gn = get_attr("givenName")
                sn = get_attr("sn")
                if gn or sn:
                    full_name = f"{gn} {sn}".strip()
    except Exception:
        pass
    finally:
        bound_conn.unbind()

    if not full_name:
        full_name = username

    profile = {
        "username": username,
        "email": email,
        "full_name": full_name
    }
    return _get_or_create_ldap_user(profile)


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
