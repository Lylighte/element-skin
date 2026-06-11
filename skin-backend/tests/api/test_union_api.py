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


@pytest.mark.asyncio
async def test_union_oauth2_grant_authenticated(client, auth_headers):
    """Test OAuth2 grant: authenticated user → 302 redirect with valid base64 userInfoToken.

    Verifies:
    - Authenticated request returns 302 redirect
    - Location header contains userInfoToken param
    - Token is valid base64 (existence verified, full RSA blob not decrypted)
    - Redirect target is Union's /oauth2/continue endpoint
    - Uses follow_redirects=False to avoid hitting the Union URL
    """
    import base64
    from unittest.mock import patch, AsyncMock
    from routes_reference import union_backend

    # Set Union API root (OAuth2 is enabled by default; the user from auth_headers
    # already exists in the test DB)
    await union_backend.update_settings({
        "union_api_root": "https://test.union.example.com/api/union",
    })

    # Mock build_oauth2_token — the real implementation requires RSA keys
    # (not in test DB) and would make an HTTP call to Union's OAuth2 backend
    test_payload = b"test_oauth2_token_payload"
    test_token = base64.b64encode(test_payload).decode("utf-8")

    with patch.object(union_backend, "build_oauth2_token", new_callable=AsyncMock) as mock_build:
        mock_build.return_value = test_token

        resp = await client.get(
            "/api/union/member/oauth2/grant",
            cookies=auth_headers["cookies"],
            follow_redirects=False,
        )

        # Verify redirect (Starlette RedirectResponse defaults to 307)
        assert resp.status_code == 307, f"Expected 307, got {resp.status_code}: {resp.text}"

        # Verify Location header
        location = resp.headers.get("location", "")
        assert location, "Location header is missing"

        # Verify redirect targets Union's OAuth2 continue endpoint
        expected_prefix = "https://test.union.example.com/api/union/oauth2/continue?"
        assert location.startswith(expected_prefix), (
            f"Location should start with {expected_prefix}, got: {location}"
        )

        # Extract and verify userInfoToken param
        from urllib.parse import urlparse, parse_qs
        parsed = urlparse(location)
        params = parse_qs(parsed.query)
        assert "userInfoToken" in params, f"No userInfoToken in Location query: {parsed.query}"
        token = params["userInfoToken"][0]
        assert token == test_token, (
            f"Token mismatch: expected {test_token}, got {token}"
        )

        # Verify token is valid base64 and decodes to non-empty bytes
        decoded = base64.b64decode(token)
        assert len(decoded) > 0, "Decoded token should not be empty"
        assert decoded == test_payload, (
            f"Decoded payload mismatch: expected {test_payload}, got {decoded}"
        )

        # Verify build_oauth2_token was called once with the authenticated user
        mock_build.assert_called_once()


# ========================================================================
# GROUP D: User-facing Union endpoints (JWT auth)
# ========================================================================


@pytest.mark.asyncio
async def test_union_user_profiles_happy(client, db_session, auth_headers):
    """Test GET /union/profiles with valid auth returns profile list.

    Mocks _api_get to avoid real HTTP calls to Union.
    First call (get_profile_unmapped_byname) returns None (no duplicates).
    Second call (get_profile_detail) returns profile detail dict.
    """
    from unittest.mock import patch, AsyncMock
    from routes_reference import union_backend
    from utils.typing import PlayerProfile

    user_id = auth_headers["X-User-ID"]
    profile = PlayerProfile("test-uuid-profiles", user_id, "TestPlayer")
    await db_session.user.create_profile(profile)

    with patch.object(union_backend, "_api_get", new_callable=AsyncMock) as mock_get:
        # First: get_profile_unmapped_byname("TestPlayer") -> None (no duplicates)
        # Second: get_profile_detail("test-uuid-profiles") -> profile detail
        mock_get.side_effect = [
            None,
            {"internal_id": "test-uuid-profiles", "bind": []},
        ]

        resp = await client.get("/union/profiles", cookies=auth_headers["cookies"])
        assert resp.status_code == 200, f"Expected 200, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert "items" in data
        assert len(data["items"]) == 1
        assert data["items"][0]["name"] == "TestPlayer"
        assert data["items"][0]["self"] == {"internal_id": "test-uuid-profiles", "bind": []}
        assert data["items"][0]["dup_name"] == []


@pytest.mark.asyncio
async def test_union_user_profiles_multiple(client, db_session, auth_headers):
    """Test GET /union/profiles with multiple profiles owned by user."""
    from unittest.mock import patch, AsyncMock
    from routes_reference import union_backend
    from utils.typing import PlayerProfile

    user_id = auth_headers["X-User-ID"]
    p1 = PlayerProfile("uuid-profile-1", user_id, "PlayerOne")
    p2 = PlayerProfile("uuid-profile-2", user_id, "PlayerTwo")
    await db_session.user.create_profile(p1)
    await db_session.user.create_profile(p2)

    with patch.object(union_backend, "_api_get", new_callable=AsyncMock) as mock_get:
        # dup_coros: get_profile_unmapped_byname for each profile (N calls)
        # self_coros: get_profile_detail for each profile (N calls)
        mock_get.side_effect = [
            None,                              # dup for PlayerOne
            None,                              # dup for PlayerTwo
            {"internal_id": "uuid-profile-1", "bind": []},  # detail for P1
            {"internal_id": "uuid-profile-2", "bind": []},  # detail for P2
        ]

        resp = await client.get("/union/profiles", cookies=auth_headers["cookies"])
        assert resp.status_code == 200
        data = resp.json()
        assert len(data["items"]) == 2
        names = {item["name"] for item in data["items"]}
        assert names == {"PlayerOne", "PlayerTwo"}


@pytest.mark.asyncio
async def test_union_user_profiles_no_profiles(client, auth_headers):
    """Test GET /union/profiles when user has no profiles returns empty list."""
    from unittest.mock import patch, AsyncMock
    from routes_reference import union_backend

    with patch.object(union_backend, "_api_get", new_callable=AsyncMock) as mock_get:
        # get_user_profiles returns [], so no _api_get calls are made
        resp = await client.get("/union/profiles", cookies=auth_headers["cookies"])
        assert resp.status_code == 200
        data = resp.json()
        assert data == {"items": []}
        mock_get.assert_not_called()


@pytest.mark.asyncio
async def test_union_bind_happy(client, db_session, auth_headers):
    """Test POST /union/bind with valid auth and owned uuid returns token.

    Mocks _api_post to avoid real HTTP call to Union.
    """
    from unittest.mock import patch, AsyncMock
    from routes_reference import union_backend
    from utils.typing import PlayerProfile

    user_id = auth_headers["X-User-ID"]
    profile = PlayerProfile("test-uuid-bind", user_id, "BindPlayer")
    await db_session.user.create_profile(profile)

    with patch.object(union_backend, "_api_post", new_callable=AsyncMock) as mock_post:
        mock_post.return_value = {"token": "bind-token-abc123"}

        resp = await client.post(
            "/union/bind",
            json={"uuid": "test-uuid-bind"},
            cookies=auth_headers["cookies"],
        )
        assert resp.status_code == 200, f"Expected 200, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert data["token"] == "bind-token-abc123"

        # Verify _api_post was called with correct path and data
        mock_post.assert_called_once_with("profile/bind", {"uuid": "test-uuid-bind"})


@pytest.mark.asyncio
async def test_union_bind_missing_uuid(client, auth_headers):
    """Test POST /union/bind without uuid returns 400."""
    resp = await client.post(
        "/union/bind",
        json={},
        cookies=auth_headers["cookies"],
    )
    assert resp.status_code == 400
    assert "uuid is required" in resp.text


@pytest.mark.asyncio
async def test_union_bind_not_owned(client, db_session, user_factory, auth_headers):
    """Test POST /union/bind with uuid not owned by user returns 403."""
    from utils.typing import PlayerProfile

    # Create another user and a profile owned by them
    other_user = await user_factory()
    profile = PlayerProfile("uuid-not-owned", other_user.id, "OtherPlayer")
    await db_session.user.create_profile(profile)

    resp = await client.post(
        "/union/bind",
        json={"uuid": "uuid-not-owned"},
        cookies=auth_headers["cookies"],
    )
    assert resp.status_code == 403
    assert "not owned" in resp.text


@pytest.mark.asyncio
async def test_union_unbind_happy(client, db_session, auth_headers):
    """Test POST /union/unbind with valid auth and owned uuid succeeds.

    Mocks _api_post to avoid real HTTP call to Union.
    """
    from unittest.mock import patch, AsyncMock
    from routes_reference import union_backend
    from utils.typing import PlayerProfile

    user_id = auth_headers["X-User-ID"]
    profile = PlayerProfile("test-uuid-unbind", user_id, "UnbindPlayer")
    await db_session.user.create_profile(profile)

    with patch.object(union_backend, "_api_post", new_callable=AsyncMock) as mock_post:
        mock_post.return_value = {"ok": True}

        resp = await client.post(
            "/union/unbind",
            json={"uuid": "test-uuid-unbind"},
            cookies=auth_headers["cookies"],
        )
        assert resp.status_code == 200, f"Expected 200, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert data["ok"] is True

        mock_post.assert_called_once_with("profile/unbind", {"uuid": "test-uuid-unbind"})


@pytest.mark.asyncio
async def test_union_unbind_missing_uuid(client, auth_headers):
    """Test POST /union/unbind without uuid returns 400."""
    resp = await client.post(
        "/union/unbind",
        json={},
        cookies=auth_headers["cookies"],
    )
    assert resp.status_code == 400
    assert "uuid is required" in resp.text


@pytest.mark.asyncio
async def test_union_unbind_not_owned(client, db_session, user_factory, auth_headers):
    """Test POST /union/unbind with uuid not owned by user returns 403."""
    from utils.typing import PlayerProfile

    other_user = await user_factory()
    profile = PlayerProfile("uuid-unbind-not-owned", other_user.id, "OtherPlayer")
    await db_session.user.create_profile(profile)

    resp = await client.post(
        "/union/unbind",
        json={"uuid": "uuid-unbind-not-owned"},
        cookies=auth_headers["cookies"],
    )
    assert resp.status_code == 403
    assert "not owned" in resp.text


@pytest.mark.asyncio
async def test_union_bindto_happy(client, db_session, auth_headers):
    """Test POST /union/bindto with valid auth, uuid, and token succeeds.

    Mocks _api_post to avoid real HTTP call to Union.
    """
    from unittest.mock import patch, AsyncMock
    from routes_reference import union_backend
    from utils.typing import PlayerProfile

    user_id = auth_headers["X-User-ID"]
    profile = PlayerProfile("test-uuid-bindto", user_id, "BindtoPlayer")
    await db_session.user.create_profile(profile)

    with patch.object(union_backend, "_api_post", new_callable=AsyncMock) as mock_post:
        mock_post.return_value = {"ok": True}

        resp = await client.post(
            "/union/bindto",
            json={"uuid": "test-uuid-bindto", "token": "bindto-token-xyz"},
            cookies=auth_headers["cookies"],
        )
        assert resp.status_code == 200, f"Expected 200, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert data["ok"] is True

        mock_post.assert_called_once_with(
            "profile/bindto",
            {"uuid": "test-uuid-bindto", "token": "bindto-token-xyz"},
        )


@pytest.mark.asyncio
async def test_union_bindto_missing_fields(client, auth_headers):
    """Test POST /union/bindto without uuid or token returns 400."""
    resp = await client.post(
        "/union/bindto",
        json={"uuid": "some-uuid"},
        cookies=auth_headers["cookies"],
    )
    assert resp.status_code == 400
    assert "uuid and token are required" in resp.text


@pytest.mark.asyncio
async def test_union_remapuuid_happy(client, db_session, auth_headers):
    """Test POST /union/remapuuid with valid auth, me, and target succeeds.

    Mocks _api_post to avoid real HTTP call to Union.
    """
    from unittest.mock import patch, AsyncMock
    from routes_reference import union_backend
    from utils.typing import PlayerProfile

    user_id = auth_headers["X-User-ID"]
    profile = PlayerProfile("test-uuid-remap", user_id, "RemapPlayer")
    await db_session.user.create_profile(profile)

    with patch.object(union_backend, "_api_post", new_callable=AsyncMock) as mock_post:
        mock_post.return_value = {"ok": True}

        resp = await client.post(
            "/union/remapuuid",
            json={"me": "test-uuid-remap", "target": "target-uuid-abc"},
            cookies=auth_headers["cookies"],
        )
        assert resp.status_code == 200, f"Expected 200, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert data["ok"] is True

        mock_post.assert_called_once_with(
            "profile/remapuuid",
            {"me": "test-uuid-remap", "target": "target-uuid-abc"},
        )


@pytest.mark.asyncio
async def test_union_remapuuid_missing_fields(client, auth_headers):
    """Test POST /union/remapuuid without me or target returns 400."""
    resp = await client.post(
        "/union/remapuuid",
        json={"me": "some-uuid"},
        cookies=auth_headers["cookies"],
    )
    assert resp.status_code == 400
    assert "me and target are required" in resp.text


@pytest.mark.asyncio
async def test_union_remapuuid_not_owned(client, db_session, user_factory, auth_headers):
    """Test POST /union/remapuuid with me not owned by user returns 403."""
    from utils.typing import PlayerProfile

    other_user = await user_factory()
    profile = PlayerProfile("uuid-remap-not-owned", other_user.id, "OtherPlayer")
    await db_session.user.create_profile(profile)

    resp = await client.post(
        "/union/remapuuid",
        json={"me": "uuid-remap-not-owned", "target": "target-uuid-abc"},
        cookies=auth_headers["cookies"],
    )
    assert resp.status_code == 403
    assert "not owned" in resp.text


@pytest.mark.asyncio
async def test_union_security_level_happy(client, auth_headers):
    """Test GET /union/security/level with valid auth returns security level.

    Mocks get_security_level (which makes direct HTTP calls, not via _api_*).
    """
    from unittest.mock import patch, AsyncMock
    from routes_reference import union_backend

    with patch.object(union_backend, "get_security_level", new_callable=AsyncMock) as mock_level:
        mock_level.return_value = 2

        resp = await client.get("/union/security/level", cookies=auth_headers["cookies"])
        assert resp.status_code == 200, f"Expected 200, got {resp.status_code}: {resp.text}"
        data = resp.json()
        assert data["security_level"] == 2

        mock_level.assert_called_once()


@pytest.mark.asyncio
async def test_union_security_level_zero(client, auth_headers):
    """Test GET /union/security/level returns 0 (valid security level value)."""
    from unittest.mock import patch, AsyncMock
    from routes_reference import union_backend

    with patch.object(union_backend, "get_security_level", new_callable=AsyncMock) as mock_level:
        mock_level.return_value = 0

        resp = await client.get("/union/security/level", cookies=auth_headers["cookies"])
        assert resp.status_code == 200
        data = resp.json()
        assert data["security_level"] == 0


@pytest.mark.asyncio
async def test_union_bind_unauthenticated(client):
    """Test POST /union/bind without auth returns 401."""
    resp = await client.post("/union/bind", json={"uuid": "test-uuid"})
    assert resp.status_code == 401


@pytest.mark.asyncio
async def test_union_unbind_unauthenticated(client):
    """Test POST /union/unbind without auth returns 401."""
    resp = await client.post("/union/unbind", json={"uuid": "test-uuid"})
    assert resp.status_code == 401


@pytest.mark.asyncio
async def test_union_bindto_unauthenticated(client):
    """Test POST /union/bindto without auth returns 401."""
    resp = await client.post("/union/bindto", json={"uuid": "test-uuid", "token": "test-token"})
    assert resp.status_code == 401


@pytest.mark.asyncio
async def test_union_remapuuid_unauthenticated(client):
    """Test POST /union/remapuuid without auth returns 401."""
    resp = await client.post("/union/remapuuid", json={"me": "test-uuid", "target": "target-uuid"})
    assert resp.status_code == 401


@pytest.mark.asyncio
async def test_union_oauth2_grant_unauthenticated(client):
    """Test OAuth2 grant endpoint returns 401 for unauthenticated requests."""
    resp = await client.get(
        "/api/union/member/oauth2/grant",
        follow_redirects=False,
    )
    assert resp.status_code == 401
    assert "not authenticated" in resp.text


# ========================================================================
# Admin Blacklist CRUD Tests
# ========================================================================


@pytest.mark.asyncio
async def test_admin_blacklist_list(client, admin_headers):
    """Test admin can list Union blacklist entries."""
    from unittest.mock import patch
    from routes_reference import union_backend

    fake_page = {
        "data": [
            {"id": "bl_001", "email": "spam@example.com", "reason": "Spamming",
             "banned_at": "2025-01-01T00:00:00Z", "active": True},
        ],
        "total": 1,
        "page": 1,
        "per_page": 20,
    }

    with patch.object(union_backend, "get_blacklist", return_value=fake_page):
        resp = await client.get("/admin/union/blacklist", cookies=admin_headers["cookies"])
        assert resp.status_code == 200
        data = resp.json()
        assert data["total"] == 1
        assert len(data["data"]) == 1
        assert data["data"][0]["email"] == "spam@example.com"
        assert data["data"][0]["reason"] == "Spamming"


@pytest.mark.asyncio
async def test_admin_blacklist_list_with_query(client, admin_headers):
    """Test admin can search blacklist entries by query."""
    from unittest.mock import patch
    from routes_reference import union_backend

    fake_page = {
        "data": [
            {"id": "bl_002", "email": "abuse@example.com", "reason": "Abuse",
             "banned_at": "2025-02-01T00:00:00Z", "active": True},
        ],
        "total": 1,
        "page": 1,
        "per_page": 20,
    }

    with patch.object(union_backend, "get_blacklist", return_value=fake_page) as mock_get:
        resp = await client.get(
            "/admin/union/blacklist",
            params={"q": "abuse"},
            cookies=admin_headers["cookies"],
        )
        assert resp.status_code == 200
        data = resp.json()
        assert data["total"] == 1
        assert data["data"][0]["email"] == "abuse@example.com"

        # Verify the backend was called with the search param
        mock_get.assert_awaited_once_with({"q": "abuse", "page": 1})


@pytest.mark.asyncio
async def test_admin_blacklist_list_failure(client, admin_headers):
    """Test blacklist list returns 502 when backend returns None."""
    from unittest.mock import patch
    from routes_reference import union_backend

    with patch.object(union_backend, "get_blacklist", return_value=None):
        resp = await client.get("/admin/union/blacklist", cookies=admin_headers["cookies"])
        assert resp.status_code == 502
        assert "Failed to query blacklist" in resp.text


@pytest.mark.asyncio
async def test_admin_blacklist_list_requires_admin(client, auth_headers):
    """Test non-admin user cannot list blacklist entries."""
    resp = await client.get("/admin/union/blacklist", cookies=auth_headers["cookies"])
    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_admin_blacklist_create(client, admin_headers):
    """Test admin can create a new blacklist entry on Union."""
    from unittest.mock import patch
    from routes_reference import union_backend

    fake_entry = {
        "id": "bl_new_001",
        "email": "cheater@example.com",
        "reason": "Cheating",
        "banned_at": "2025-06-11T00:00:00Z",
        "active": True,
    }

    with patch.object(union_backend, "create_blacklist", return_value=fake_entry) as mock_create:
        resp = await client.post(
            "/admin/union/blacklist",
            json={"email": "cheater@example.com", "reason": "Cheating"},
            cookies=admin_headers["cookies"],
        )
        assert resp.status_code == 200
        data = resp.json()
        assert data["id"] == "bl_new_001"
        assert data["email"] == "cheater@example.com"
        assert data["active"] is True

        mock_create.assert_awaited_once_with({"email": "cheater@example.com", "reason": "Cheating"})


@pytest.mark.asyncio
async def test_admin_blacklist_create_default_reason(client, admin_headers):
    """Test creating blacklist entry without reason uses empty string."""
    from unittest.mock import patch
    from routes_reference import union_backend

    fake_entry = {
        "id": "bl_new_002",
        "email": "bot@example.com",
        "reason": "",
        "banned_at": "2025-06-11T00:00:00Z",
        "active": True,
    }

    with patch.object(union_backend, "create_blacklist", return_value=fake_entry) as mock_create:
        resp = await client.post(
            "/admin/union/blacklist",
            json={"email": "bot@example.com"},
            cookies=admin_headers["cookies"],
        )
        assert resp.status_code == 200
        mock_create.assert_awaited_once_with({"email": "bot@example.com", "reason": ""})


@pytest.mark.asyncio
async def test_admin_blacklist_create_missing_email(client, admin_headers):
    """Test creating blacklist entry without email returns 400."""
    resp = await client.post(
        "/admin/union/blacklist",
        json={"reason": "No email"},
        cookies=admin_headers["cookies"],
    )
    assert resp.status_code == 400
    assert "email is required" in resp.text


@pytest.mark.asyncio
async def test_admin_blacklist_create_failure(client, admin_headers):
    """Test blacklist create returns 502 when backend returns None."""
    from unittest.mock import patch
    from routes_reference import union_backend

    with patch.object(union_backend, "create_blacklist", return_value=None):
        resp = await client.post(
            "/admin/union/blacklist",
            json={"email": "fail@example.com", "reason": "Test"},
            cookies=admin_headers["cookies"],
        )
        assert resp.status_code == 502
        assert "Failed to create blacklist entry" in resp.text


@pytest.mark.asyncio
async def test_admin_blacklist_create_requires_admin(client, auth_headers):
    """Test non-admin user cannot create blacklist entries."""
    resp = await client.post(
        "/admin/union/blacklist",
        json={"email": "hacker@example.com"},
        cookies=auth_headers["cookies"],
    )
    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_admin_blacklist_invalidate(client, admin_headers):
    """Test admin can invalidate (unban) a blacklist entry."""
    from unittest.mock import patch
    from routes_reference import union_backend

    with patch.object(union_backend, "invalidate_blacklist", return_value=True) as mock_invalidate:
        resp = await client.post(
            "/admin/union/blacklist/bl_001/invalidate",
            cookies=admin_headers["cookies"],
        )
        assert resp.status_code == 200
        data = resp.json()
        assert data["ok"] is True

        mock_invalidate.assert_awaited_once_with("bl_001")


@pytest.mark.asyncio
async def test_admin_blacklist_invalidate_failure(client, admin_headers):
    """Test invalidate returns 502 when backend returns False."""
    from unittest.mock import patch
    from routes_reference import union_backend

    with patch.object(union_backend, "invalidate_blacklist", return_value=False):
        resp = await client.post(
            "/admin/union/blacklist/bl_001/invalidate",
            cookies=admin_headers["cookies"],
        )
        assert resp.status_code == 502
        assert "Failed to invalidate blacklist entry" in resp.text


@pytest.mark.asyncio
async def test_admin_blacklist_invalidate_requires_admin(client, auth_headers):
    """Test non-admin user cannot invalidate blacklist entries."""
    resp = await client.post(
        "/admin/union/blacklist/bl_001/invalidate",
        cookies=auth_headers["cookies"],
    )
    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_admin_blacklist_delete(client, admin_headers):
    """Test admin can delete a blacklist entry."""
    from unittest.mock import patch
    from routes_reference import union_backend

    with patch.object(union_backend, "delete_blacklist", return_value=True) as mock_delete:
        resp = await client.delete(
            "/admin/union/blacklist/bl_001",
            cookies=admin_headers["cookies"],
        )
        assert resp.status_code == 200
        data = resp.json()
        assert data["ok"] is True

        mock_delete.assert_awaited_once_with("bl_001")


@pytest.mark.asyncio
async def test_admin_blacklist_delete_failure(client, admin_headers):
    """Test delete returns 502 when backend returns False."""
    from unittest.mock import patch
    from routes_reference import union_backend

    with patch.object(union_backend, "delete_blacklist", return_value=False):
        resp = await client.delete(
            "/admin/union/blacklist/bl_001",
            cookies=admin_headers["cookies"],
        )
        assert resp.status_code == 502
        assert "Failed to delete blacklist entry" in resp.text


@pytest.mark.asyncio
async def test_admin_blacklist_delete_requires_admin(client, auth_headers):
    """Test non-admin user cannot delete blacklist entries."""
    resp = await client.delete(
        "/admin/union/blacklist/bl_001",
        cookies=auth_headers["cookies"],
    )
    assert resp.status_code == 403


@pytest.mark.asyncio
async def test_admin_union_sync(client, admin_headers):
    """Test admin can trigger profile sync."""
    from unittest.mock import patch, AsyncMock
    from routes_reference import union_backend

    with patch.object(union_backend, "sync_profiles", new_callable=AsyncMock) as mock_sync:
        mock_sync.return_value = True
        resp = await client.post("/admin/union/sync", cookies=admin_headers["cookies"])
        assert resp.status_code == 200
        data = resp.json()
        assert data == {"ok": True}
        mock_sync.assert_called_once()


@pytest.mark.asyncio
async def test_admin_union_sync_failure(client, admin_headers):
    """Test admin sync returns 502 when backend fails."""
    from unittest.mock import patch, AsyncMock
    from routes_reference import union_backend

    with patch.object(union_backend, "sync_profiles", new_callable=AsyncMock) as mock_sync:
        mock_sync.return_value = False
        resp = await client.post("/admin/union/sync", cookies=admin_headers["cookies"])
        assert resp.status_code == 502
        assert "Failed to sync profiles" in resp.text


@pytest.mark.asyncio
async def test_admin_union_diagnose(client, admin_headers):
    """Test admin can run connectivity diagnostic."""
    from unittest.mock import patch, AsyncMock
    from routes_reference import union_backend

    with patch.object(union_backend, "trigger_diagnose", new_callable=AsyncMock) as mock_diag:
        mock_diag.return_value = {"status": "ok", "data": {"reachable": True}}
        resp = await client.post("/admin/union/diagnose", cookies=admin_headers["cookies"])
        assert resp.status_code == 200
        data = resp.json()
        assert data == {"status": "ok", "data": {"reachable": True}}
        mock_diag.assert_called_once()


@pytest.mark.asyncio
async def test_admin_union_diagnose_error(client, admin_headers):
    """Test admin diagnose returns error status from backend."""
    from unittest.mock import patch, AsyncMock
    from routes_reference import union_backend

    with patch.object(union_backend, "trigger_diagnose", new_callable=AsyncMock) as mock_diag:
        mock_diag.return_value = {"status": "error", "data": {"exception": "Connection refused"}}
        resp = await client.post("/admin/union/diagnose", cookies=admin_headers["cookies"])
        assert resp.status_code == 200
        data = resp.json()
        assert data["status"] == "error"
        assert "Connection refused" in data["data"]["exception"]


@pytest.mark.asyncio
async def test_admin_union_update_list(client, admin_headers):
    """Test admin can trigger server list update."""
    from unittest.mock import patch, AsyncMock
    from routes_reference import union_backend

    with patch.object(union_backend, "fetch_server_list", new_callable=AsyncMock) as mock_fetch:
        mock_fetch.return_value = True
        resp = await client.post("/admin/union/update-list", cookies=admin_headers["cookies"])
        assert resp.status_code == 200
        data = resp.json()
        assert data == {"ok": True}
        mock_fetch.assert_called_once()


@pytest.mark.asyncio
async def test_admin_union_update_list_failure(client, admin_headers):
    """Test admin update-list returns 502 when backend fails."""
    from unittest.mock import patch, AsyncMock
    from routes_reference import union_backend

    with patch.object(union_backend, "fetch_server_list", new_callable=AsyncMock) as mock_fetch:
        mock_fetch.return_value = False
        resp = await client.post("/admin/union/update-list", cookies=admin_headers["cookies"])
        assert resp.status_code == 502
        assert "Failed to update server list" in resp.text


@pytest.mark.asyncio
async def test_admin_union_update_key(client, admin_headers):
    """Test admin can trigger private key update."""
    from unittest.mock import patch, AsyncMock
    from routes_reference import union_backend

    with patch.object(union_backend, "fetch_private_key", new_callable=AsyncMock) as mock_fetch:
        mock_fetch.return_value = True
        resp = await client.post("/admin/union/update-key", cookies=admin_headers["cookies"])
        assert resp.status_code == 200
        data = resp.json()
        assert data == {"ok": True}
        mock_fetch.assert_called_once()


@pytest.mark.asyncio
async def test_admin_union_update_key_failure(client, admin_headers):
    """Test admin update-key returns 502 when backend fails."""
    from unittest.mock import patch, AsyncMock
    from routes_reference import union_backend

    with patch.object(union_backend, "fetch_private_key", new_callable=AsyncMock) as mock_fetch:
        mock_fetch.return_value = False
        resp = await client.post("/admin/union/update-key", cookies=admin_headers["cookies"])
        assert resp.status_code == 502
        assert "Failed to update private key" in resp.text


@pytest.mark.asyncio
async def test_admin_union_endpoints_require_admin(client, auth_headers):
    """Test all admin union endpoints reject non-admin users."""
    endpoints = [
        "/admin/union/sync",
        "/admin/union/diagnose",
        "/admin/union/update-list",
        "/admin/union/update-key",
    ]
    for ep in endpoints:
        resp = await client.post(ep, cookies=auth_headers["cookies"])
        assert resp.status_code == 403, f"{ep} should 403 for non-admin, got {resp.status_code}"
