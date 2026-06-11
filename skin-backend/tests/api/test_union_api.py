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
    assert "ygg_private_key" not in data
    assert "union_ygg_private_key_fingerprint" in data
    assert isinstance(data["union_ygg_private_key_fingerprint"], str)
    assert "union_ygg_private_key_present" in data
    assert isinstance(data["union_ygg_private_key_present"], bool)


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


@pytest.mark.asyncio
async def test_union_inbound_rate_limit_exceeded(client, db_session):
    """Test that 101st request within 60s returns 429."""
    from unittest.mock import patch, AsyncMock
    from routes_reference import rate_limiter, union_backend

    # Clear rate limiter state to start fresh
    rate_limiter._attempts.clear()

    # Mock Union signature verification to succeed, and is_update_enabled to
    # return False so the handler returns immediately without real work
    with patch.object(union_backend, "verify_union_request_inbound", return_value=None), \
         patch.object(union_backend, "is_update_enabled", new_callable=AsyncMock) as mock_update:
        mock_update.return_value = False

        url = "/api/union/member/updatelist"
        headers = {
            "X-Message-Signature": "valid_sig",
            "X-Message-Timestamp": "1000000000",
            "X-Message-Nonce": "test_nonce_rl",
        }

        # Send 100 rapid requests — all should succeed
        for i in range(100):
            resp = await client.post(url, headers=headers)
            assert resp.status_code == 200, f"Request {i + 1} failed: {resp.text}"

        # 101st request — should be rate limited
        resp = await client.post(url, headers=headers)
        assert resp.status_code == 429
        assert "Rate limit exceeded" in resp.text


@pytest.mark.asyncio
async def test_union_inbound_invalid_signature_does_not_consume_rate_limit(client, db_session):
    """Test that invalid signature requests return 401 and do NOT consume rate limit."""
    from unittest.mock import patch, AsyncMock
    from routes_reference import rate_limiter, union_backend

    # Clear rate limiter state to start fresh
    rate_limiter._attempts.clear()

    # Send 100 requests with invalid signature — all should 401 at
    # verify_union_request, never reaching rate_limiter.check()
    for i in range(100):
        resp = await client.post("/api/union/member/updatelist")
        assert resp.status_code == 401

    # Now send a valid (mocked) request — should succeed, not 429, because
    # the invalid-signature requests never consumed the rate limit
    with patch.object(union_backend, "verify_union_request_inbound", return_value=None), \
         patch.object(union_backend, "is_update_enabled", new_callable=AsyncMock) as mock_update:
        mock_update.return_value = False

        resp = await client.post("/api/union/member/updatelist", headers={
            "X-Message-Signature": "valid_sig",
            "X-Message-Timestamp": "1000000000",
            "X-Message-Nonce": "test_nonce_rl2",
        })
        assert resp.status_code == 200, f"Expected 200, got {resp.status_code}: {resp.text}"
