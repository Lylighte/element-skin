from __future__ import annotations

import json

from element_skin_sdk import FileTokenStore, MemoryTokenStore
from element_skin_sdk.models import TokenSet


def test_memory_token_store_round_trips_and_clears_exact_token() -> None:
    tokens = TokenSet(
        access_token="access-token-1",
        token_type="Bearer",
        expires_in=3600,
        scope="account.read.self",
        refresh_token="refresh-token-1",
        permissions=("account.read.self",),
    )
    store = MemoryTokenStore()

    assert store.load() is None
    store.save(tokens)
    assert store.load() == tokens
    store.clear()
    assert store.load() is None


def test_file_token_store_writes_structured_json_and_clears_file(tmp_path) -> None:
    path = tmp_path / "tokens.json"
    tokens = TokenSet(
        access_token="access-token-1",
        token_type="Bearer",
        expires_in=3600,
        scope="account.read.self",
        refresh_token="refresh-token-1",
        permissions=("account.read.self",),
    )
    store = FileTokenStore(path)

    store.save(tokens)

    assert json.loads(path.read_text(encoding="utf-8")) == {
        "access_token": "access-token-1",
        "token_type": "Bearer",
        "expires_in": 3600,
        "scope": "account.read.self",
        "refresh_token": "refresh-token-1",
        "permissions": ["account.read.self"],
    }
    assert store.load() == tokens
    store.clear()
    assert not path.exists()
