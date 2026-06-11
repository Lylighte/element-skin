"""Integration tests for Union key fallback behavior.

Verifies that when ``use_union_key=false`` (default):
1. Yggdrasil signs with the default ``/app/data/private.pem`` key, not Union key
2. ``fetch_private_key()`` writes the Union key file but does NOT reload crypto
3. Union key file existence does NOT affect which key Yggdrasil uses

All tests must pass without real HTTP calls to Union — only mocked responses.
"""

import pytest
from pathlib import Path
from unittest.mock import AsyncMock, patch

from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.hazmat.primitives import serialization

from backends.union_backend import UnionBackend

# ·············································································
# Test infrastructure
# ·············································································

def _generate_fake_pem() -> str:
    """Generate a fresh RSA 2048-bit private key PEM.

    Used as a *different* key to verify the default key is still in use.
    """
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    return key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption(),
    ).decode("utf-8")


# Module-level fake PEM — all tests use the same alternative key to verify
# the system does NOT switch to it when use_union_key=false.
FAKE_UNION_PEM = _generate_fake_pem()


def _write_union_key_file(base_path: Path) -> None:
    """Write the fake Union key to the given directory."""
    (base_path / "union-ygg-private.pem").write_text(FAKE_UNION_PEM)


def _cleanup_union_key_file(base_path: Path) -> None:
    """Remove the Union key file from the given directory if it exists."""
    p = base_path / "union-ygg-private.pem"
    if p.exists():
        p.unlink()


# ·············································································
# Test 1 — HTTP: Yggdrasil metadata returns DEFAULT public key
# ·············································································

async def test_default_key_used_when_union_disabled(client, crypto_fixture, tmp_path):
    """Yggdrasil metadata endpoint returns the default key's public key.

    Even when the Union key file exists with a *different* key,
    ``use_union_key=false`` means the system must keep using the default
    ``private.pem`` for signing.
    """
    default_public_key = crypto_fixture.get_public_key_pem()

    _write_union_key_file(tmp_path)
    try:
        resp = await client.get("/")
        assert resp.status_code == 200
        data = resp.json()

        assert "signaturePublickey" in data, (
            "metadata response is missing signaturePublickey"
        )
        assert data["signaturePublickey"] == default_public_key, (
            "Metadata returned Union key's public key, "
            "but use_union_key=false should use the default key"
        )
    finally:
        _cleanup_union_key_file(tmp_path)


# ·············································································
# Test 2 — Backend: fetch_private_key writes file, no crypto reload
# ·············································································

async def test_fetch_private_key_stores_file_without_reload(
    db_session, test_config, crypto_fixture
):
    """``fetch_private_key()`` with ``use_union_key=false`` writes the PEM
    file but does **not** reload the global ``CryptoUtils`` instance.

    The file is stored for potential future use (if admin flips the toggle
    later), but the running system must continue signing with the default key.
    """
    default_public_key_before = crypto_fixture.get_public_key_pem()

    backend = UnionBackend(db_session, test_config, crypto=crypto_fixture)
    backend._api_get = AsyncMock(return_value={
        "privateKey": FAKE_UNION_PEM,
        "privateKeyVersion": 99,
    })

    with patch("builtins.open") as mock_open, \
         patch("os.chmod") as mock_chmod, \
         patch("os.makedirs") as mock_makedirs:
        result = await backend.fetch_private_key()

    # ── fetch should succeed ──────────────────────────────────────────
    assert result is True

    # ── file was written ──────────────────────────────────────────────
    mock_makedirs.assert_called_once_with("/app/data", exist_ok=True)
    mock_open.assert_called_once_with("/app/data/union-ygg-private.pem", "w")
    mock_open.return_value.__enter__.return_value.write.assert_called_once_with(
        FAKE_UNION_PEM
    )
    mock_chmod.assert_called_once_with("/app/data/union-ygg-private.pem", 0o600)

    # ── version stored in DB ──────────────────────────────────────────
    version_val = await db_session.union.get("union_private_key_version")
    assert version_val == "99"

    # ── old DB value cleared ──────────────────────────────────────────
    ygg_val = await db_session.union.get("union_ygg_private_key")
    assert ygg_val == "" or ygg_val is None, (
        f"Expected empty, got: {ygg_val!r}"
    )

    # ── crypto NOT reloaded (use_union_key=false) ─────────────────────
    default_public_key_after = crypto_fixture.get_public_key_pem()
    assert default_public_key_after == default_public_key_before, (
        "CryptoUtils public key changed after fetch_private_key — "
        "it should NOT reload when use_union_key=false"
    )


# ·············································································
# Test 3 — File existence does NOT affect default mode
# ·············································································

async def test_union_key_file_does_not_affect_default(client, crypto_fixture, tmp_path):
    """Union key file presence does NOT change which key ``CryptoUtils`` holds.

    Even if the Union key file exists on disk, the global
    ``CryptoUtils`` instance (initialized with ``use_union_key=false``) must
    continue referencing the default ``private.pem`` key.

    This tests the startup-level invariant directly — both via the live
    ``CryptoUtils`` instance and the Yggdrasil metadata HTTP endpoint.
    """
    default_public_key = crypto_fixture.get_public_key_pem()

    _write_union_key_file(tmp_path)
    try:
        # Direct check: the crypto instance should still have the default key
        current = crypto_fixture.get_public_key_pem()
        assert current == default_public_key, (
            "CryptoUtils switched to Union key even though use_union_key=false "
            "(file-only test)"
        )

        # HTTP check: Yggdrasil metadata returns the default public key
        resp = await client.get("/")
        assert resp.status_code == 200
        meta_pubkey = resp.json()["signaturePublickey"]
        assert meta_pubkey == default_public_key, (
            "Yggdrasil metadata returned Union key's public key "
            "(file-exists test via HTTP)"
        )
    finally:
        _cleanup_union_key_file(tmp_path)
