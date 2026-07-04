"""Permission helpers and constants."""

from .catalog import PermissionCatalog
from .scopes import (
    AccountScopes,
    InviteScopes,
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
    "InviteScopes",
    "MinecraftScopes",
    "NoticeScopes",
    "OAuthScopes",
    "PermissionCatalog",
    "PermissionValidator",
    "ProfileScopes",
    "TextureScopes",
    "WardrobeScopes",
]
