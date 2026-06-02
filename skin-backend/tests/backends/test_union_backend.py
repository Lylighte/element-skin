"""UnionBackend fetch_private_key tests: file write, no DB key storage, DB cleanup."""

import pytest
from unittest.mock import AsyncMock, patch

from backends.union_backend import UnionBackend

TEST_PEM = """-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQ...
-----END PRIVATE KEY-----"""


@pytest.mark.asyncio
async def test_fetch_private_key_writes_file(db_session, test_config):
    """fetch_private_key writes PEM to file with 0o600 permissions."""
    backend = UnionBackend(db_session, test_config)
    backend._api_get = AsyncMock(return_value={
        "privateKey": TEST_PEM,
        "privateKeyVersion": 42,
    })

    with patch("builtins.open") as mock_open, \
         patch("os.chmod") as mock_chmod, \
         patch("os.makedirs") as mock_makedirs:
        result = await backend.fetch_private_key()

    assert result is True
    mock_makedirs.assert_called_once_with("/app/data", exist_ok=True)
    mock_open.assert_called_once_with("/app/data/union-ygg-private.pem", "w")
    mock_open.return_value.__enter__.return_value.write.assert_called_once_with(TEST_PEM)
    mock_chmod.assert_called_once_with("/app/data/union-ygg-private.pem", 0o600)


@pytest.mark.asyncio
async def test_fetch_private_key_no_db_write(db_session, test_config):
    """fetch_private_key does NOT store union_ygg_private_key to DB as PEM value."""
    backend = UnionBackend(db_session, test_config)
    backend._api_get = AsyncMock(return_value={
        "privateKey": TEST_PEM,
        "privateKeyVersion": 42,
    })

    with patch("builtins.open"), \
         patch("os.chmod"), \
         patch("os.makedirs"):
        await backend.fetch_private_key()

    # union_ygg_private_key should be cleared (set to ""), NOT stored with the PEM value
    ygg_val = await db_session.union.get("union_ygg_private_key")
    assert ygg_val == "" or ygg_val is None, f"Expected empty, got: {ygg_val!r}"

    # Version should still be stored
    version_val = await db_session.union.get("union_private_key_version")
    assert version_val == "42"


@pytest.mark.asyncio
async def test_fetch_private_key_cleans_db(db_session, test_config):
    """fetch_private_key deletes old union_ygg_private_key from DB after file write."""
    backend = UnionBackend(db_session, test_config)

    # Pre-seed DB with old union_ygg_private_key value
    await db_session.union.set("union_ygg_private_key", "old-key-value")
    assert await db_session.union.get("union_ygg_private_key") == "old-key-value"

    backend._api_get = AsyncMock(return_value={
        "privateKey": TEST_PEM,
        "privateKeyVersion": 42,
    })

    with patch("builtins.open"), \
         patch("os.chmod"), \
         patch("os.makedirs"):
        result = await backend.fetch_private_key()

    assert result is True

    # Old DB value should be cleared
    ygg_val = await db_session.union.get("union_ygg_private_key")
    assert ygg_val == "" or ygg_val is None, f"Expected empty after cleanup, got: {ygg_val!r}"
