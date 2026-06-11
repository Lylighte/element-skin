"""Helpers for externally visible site/API URLs."""

from urllib.parse import urlsplit, urlunsplit


def normalize_public_url(url: str) -> str:
    """Return a clean absolute public URL without a trailing slash."""
    value = str(url or "").strip()
    if not value:
        return ""
    if value.startswith("/"):
        return ""
    if not value.startswith(("http://", "https://")):
        value = f"https://{value}"

    parts = urlsplit(value)
    if not parts.scheme or not parts.netloc:
        return ""
    path = parts.path.rstrip("/")
    return urlunsplit((parts.scheme, parts.netloc, path, "", ""))


def public_api_url(config) -> str:
    """Resolve the configured public Yggdrasil API base URL."""
    configured = config.get("server.api_url", "") if config else ""
    return normalize_public_url(configured)


def public_site_url(config) -> str:
    """Resolve the configured public frontend URL."""
    configured = config.get("server.site_url", "") if config else ""
    return normalize_public_url(configured)
