"""Permission helpers and constants."""

from .catalog import PermissionCatalog
from .scopes import (
    AccountScopes,
    MinecraftScopes,
    NoticeScopes,
    OAuthScopes,
    ProfileScopes,
    TextureScopes,
    WardrobeScopes,
)
from .validator import PermissionValidator

__all__ = [
    "AccountScopes",
    "MinecraftScopes",
    "NoticeScopes",
    "OAuthScopes",
    "PermissionCatalog",
    "PermissionValidator",
    "ProfileScopes",
    "TextureScopes",
    "WardrobeScopes",
]
