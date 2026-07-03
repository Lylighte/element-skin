from __future__ import annotations

import pytest

from element_skin_sdk.exceptions import InvalidScope
from element_skin_sdk.permissions import AccountScopes, MinecraftScopes, PermissionCatalog, PermissionValidator

from .fixtures import PERMISSION_CATALOG_RESPONSE


def test_permission_catalog_loads_exact_definitions() -> None:
    catalog = PermissionCatalog.from_api_payload(PERMISSION_CATALOG_RESPONSE)

    assert catalog.codes == frozenset(
        {"account.read.self", "minecraft_session.hasjoined.server"}
    )
    definition = catalog.get("minecraft_session.hasjoined.server")
    assert definition is not None
    assert definition.category == "minecraft_session"
    assert definition.action == "hasjoined"
    assert definition.scope == "server"
    assert definition.description == "Query joined Minecraft session"


def test_permission_catalog_rejects_unknown_scope_with_exact_list() -> None:
    catalog = PermissionCatalog.from_api_payload(PERMISSION_CATALOG_RESPONSE)

    with pytest.raises(InvalidScope) as exc:
        catalog.require_known(["unknown.scope.self", "account.read.self"])

    assert exc.value.invalid_scopes == ["unknown.scope.self"]
    assert str(exc.value) == "unknown permission scope: unknown.scope.self"


def test_validator_accepts_delegated_user_scope_and_rejects_server_scope() -> None:
    validator = PermissionValidator()

    assert validator.validate_delegated(AccountScopes.READ_SELF) == ("account.read.self",)
    with pytest.raises(InvalidScope) as exc:
        validator.validate_delegated([MinecraftScopes.SESSION_HASJOINED_SERVER])

    assert exc.value.invalid_scopes == ["minecraft_session.hasjoined.server"]


def test_validator_accepts_client_credentials_server_scope_and_rejects_self_scope() -> None:
    validator = PermissionValidator()

    assert validator.validate_client_credentials(MinecraftScopes.SESSION_HASJOINED_SERVER) == (
        "minecraft_session.hasjoined.server",
    )
    with pytest.raises(InvalidScope) as exc:
        validator.validate_client_credentials([AccountScopes.READ_SELF])

    assert exc.value.invalid_scopes == ["account.read.self"]
