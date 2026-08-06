"""Minimal RFC 6238 TOTP implementation (SHA1, 6 digits, 30s window)."""
import base64
import hashlib
import hmac
import secrets
import struct
import time


def generate_secret() -> str:
    """Return a 20-byte random base32 secret (160-bit, Google Authenticator compatible)."""
    return base64.b32encode(secrets.token_bytes(20)).decode("ascii").rstrip("=")


def _code_for_counter(secret_b32: str, counter: int) -> str:
    key = base64.b32decode(secret_b32.encode("ascii"), casefold=True)
    msg = struct.pack(">Q", counter)
    digest = hmac.new(key, msg, hashlib.sha1).digest()
    offset = digest[-1] & 0x0F
    truncated = struct.unpack(">I", digest[offset : offset + 4])[0] & 0x7FFFFFFF
    return f"{truncated % 1000000:06d}"


def verify_totp(secret_b32: str, code: str, drift: int = 1) -> bool:
    code = str(code).strip()
    if not code or not code.isdigit() or len(code) != 6:
        return False
    counter = int(time.time()) // 30
    for offset in range(-drift, drift + 1):
        if _code_for_counter(secret_b32, counter + offset) == code:
            return True
    return False


def otpauth_uri(secret_b32: str, username: str, issuer: str = "MyGit") -> str:
    import urllib.parse

    label = urllib.parse.quote(f"{issuer}:{username}", safe="")
    return (
        f"otpauth://totp/{label}?secret={secret_b32}&issuer={urllib.parse.quote(issuer)}"
    )
