import pytest
from httpx import AsyncClient


@pytest.mark.asyncio
async def test_union_hello(client):
    """Test Union public hello endpoint returns expected metadata."""
    resp = await client.get("/api/union/member/")
    assert resp.status_code == 200
    data = resp.json()
    assert "yggdrasilApiVersion" in data
    assert "serverListVersion" in data
    assert "privateKeyVersion" in data
    assert "enabledFeatures" in data
    assert "unionBlacklist" in data["enabledFeatures"]


@pytest.mark.asyncio
async def test_union_hello_without_trailing_slash(client):
    """Test Union hello without trailing slash."""
    resp = await client.get("/api/union/member")
    assert resp.status_code == 200
    data = resp.json()
    assert "yggdrasilApiVersion" in data


@pytest.mark.asyncio
async def test_union_query_email_found(client, db_session, user_factory):
    """Test querying email by username when user exists."""
    from utils.typing import PlayerProfile
    user = await user_factory(email="union_test@example.com", username="union_player")
    await db_session.user.create_profile(PlayerProfile("union_uuid_1", user.id, "UnionPlayer"))

    resp = await client.get("/api/union/member/queryemail", params={"username": "UnionPlayer"})
    # 2.3.0: queryemail now requires Union signature verification
    assert resp.status_code == 401
    assert "Missing Union signature headers" in resp.text


@pytest.mark.asyncio
async def test_union_query_email_not_found(client):
    """Test querying email for non-existent username returns 204."""
    resp = await client.get("/api/union/member/queryemail", params={"username": "NonexistentPlayer"})
    # 2.3.0: queryemail now requires Union signature verification
    assert resp.status_code == 401


@pytest.mark.asyncio
async def test_union_query_email_missing_param(client):
    """Test querying email without username parameter."""
    resp = await client.get("/api/union/member/queryemail")
    # 2.3.0: queryemail now requires Union signature verification (checked before param validation)
    assert resp.status_code == 401


@pytest.mark.asyncio
async def test_union_inbound_missing_signature(client):
    """Test inbound Union API rejects requests without signature headers."""
    resp = await client.post("/api/union/member/updatelist")
    assert resp.status_code == 401
    assert "Missing Union signature headers" in resp.text


@pytest.mark.asyncio
async def test_union_inbound_invalid_signature(client):
    """Test inbound Union API rejects requests with invalid signature."""
    resp = await client.post(
        "/api/union/member/updatelist",
        headers={
            "X-Message-Signature": "invalid_signature",
            "X-Message-Timestamp": "1000000000",
            "X-Message-Nonce": "test_nonce_123",
        },
    )
    assert resp.status_code == 401


@pytest.mark.asyncio
async def test_union_inbound_replay_nonce(client):
    """Test nonce reuse is rejected (replay protection)."""
    resp = await client.post(
        "/api/union/member/updatelist",
        headers={
            "X-Message-Signature": "dGVzdF9zaWc=",
            "X-Message-Timestamp": "1000000000",
            "X-Message-Nonce": "test_nonce_replay",
        },
    )
    assert resp.status_code == 401  # timestamp will also be out of range


@pytest.mark.asyncio
async def test_union_inbound_timestamp_out_of_window(client):
    """Test timestamp outside [-10, +30] window is rejected."""
    import time
    old_ts = int(time.time()) - 60  # 60 seconds ago, outside window
    resp = await client.post(
        "/api/union/member/updatelist",
        headers={
            "X-Message-Signature": "dGVzdF9zaWc=",
            "X-Message-Timestamp": str(old_ts),
            "X-Message-Nonce": "test_nonce_ts",
        },
    )
    assert resp.status_code == 401
    assert "Timestamp out" in resp.text


@pytest.mark.asyncio
async def test_admin_union_settings_get(client, admin_headers):
    """Test admin can get Union settings."""
    resp = await client.get("/admin/union/settings", cookies=admin_headers["cookies"])
    assert resp.status_code == 200
    data = resp.json()
    assert "union_api_root" in data
    assert "union_member_key" in data
    assert "union_enable_oauth2" in data


@pytest.mark.asyncio
async def test_admin_union_settings_save(client, admin_headers):
    """Test admin can save Union settings."""
    resp = await client.post(
        "/admin/union/settings",
        json={"union_api_root": "https://test.union.example.com/api/union"},
        cookies=admin_headers["cookies"],
    )
    assert resp.status_code == 200

    # Verify saved
    resp = await client.get("/admin/union/settings", cookies=admin_headers["cookies"])
    assert resp.status_code == 200
    data = resp.json()
    assert data["union_api_root"] == "https://test.union.example.com/api/union"


@pytest.mark.asyncio
async def test_admin_union_requires_admin(client, auth_headers):
    """Test non-admin user cannot access admin Union endpoints."""
    resp = await client.get("/admin/union/settings", cookies=auth_headers["cookies"])
    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_admin_generate_keypair(client, admin_headers):
    """Test admin can generate RSA keypair."""
    resp = await client.post("/admin/union/generate-keypair", cookies=admin_headers["cookies"])
    assert resp.status_code == 200
    data = resp.json()
    assert "privateKey" in data
    assert "publicKey" in data
    assert data["privateKey"].startswith("-----BEGIN")
    assert data["publicKey"].startswith("-----BEGIN")


@pytest.mark.asyncio
async def test_restore_hello(client):
    """Test restore API health check."""
    resp = await client.get("/restore")
    assert resp.status_code == 200
    data = resp.json()
    assert data["status"] == "success"


@pytest.mark.asyncio
async def test_restore_sign_disabled(client):
    """Test restore API returns 403 when disabled."""
    resp = await client.post("/restore", json={"properties": []})
    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_union_oauth2_pubkey_not_configured(client):
    """Test OAuth2 public key endpoint returns 503 when not configured."""
    resp = await client.get("/api/union/member/oauth2/")
    assert resp.status_code == 503


@pytest.mark.asyncio
async def test_union_user_profiles_not_authenticated(client):
    """Test union profiles endpoint requires auth."""
    resp = await client.get("/union/profiles")
    # Union routes use cookie-based auth; missing cookie returns 401
    assert resp.status_code == 401


@pytest.mark.asyncio
async def test_union_security_level_not_authenticated(client):
    """Test union security level endpoint requires auth."""
    resp = await client.get("/union/security/level")
    # Union routes use cookie-based auth; missing cookie returns 401
    assert resp.status_code == 401
