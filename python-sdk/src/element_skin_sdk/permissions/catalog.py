"""Permission catalog loaded from Element Skin metadata."""

from __future__ import annotations

from typing import Iterable

from ..exceptions import InvalidScope
from ..models import PermissionDefinition


class PermissionCatalog:
    def __init__(self, permissions: Iterable[PermissionDefinition]):
        self._permissions = {permission.code: permission for permission in permissions}

    @classmethod
    def from_api_payload(cls, payload: dict[str, object]) -> "PermissionCatalog":
        raw_permissions = payload.get("permissions")
        if not isinstance(raw_permissions, list):
            raise InvalidScope("permission metadata must contain a permissions list")

        return cls(
            PermissionDefinition.from_mapping(item)
            for item in raw_permissions
            if isinstance(item, dict)
        )

    @property
    def codes(self) -> frozenset[str]:
        return frozenset(self._permissions)

    def get(self, code: str) -> PermissionDefinition | None:
        return self._permissions.get(code)

    def require_known(self, scopes: Iterable[str]) -> tuple[str, ...]:
        unknown = sorted({scope for scope in scopes if scope not in self._permissions})
        if unknown:
            raise InvalidScope(
                f"unknown permission scope: {', '.join(unknown)}",
                invalid_scopes=unknown,
            )
        return tuple(scopes)
