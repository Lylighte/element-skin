from __future__ import annotations

from urllib.parse import parse_qs, urlparse

import httpx
import pytest

from element_skin_sdk import MemoryTokenStore, OAuthClient
from element_skin_sdk.exceptions import InvalidScope, OAuthError
from element_skin_sdk.permissions import AccountScopes, MinecraftScopes, ProfileScopes

from .conftest import RequestRecorder
from .fixtures import CLIENT_CREDENTIALS_TOKEN_RESPONSE, DEVICE_CODE_RESPONSE, TOKEN_RESPONSE


def test_authorization_url_builds_pkce_request_with_exact_query() -> None:
    oauth = OAuthClient(
        "https://skin.example.test/",
        "client-1",
        redirect_uri="https://app.example.test/callback",
    )

    session = oauth.authorization_url(
        [AccountScopes.READ_SELF, ProfileScopes.READ_OWNED],
        state="state-1",
        code_verifier="a" * 64,
    )

    parsed = urlparse(session.authorization_url)
    query = parse_qs(parsed.query)
    assert parsed.scheme == "https"
    assert parsed.netloc == "skin.example.test"
    assert parsed.path == "/oauth/authorize"
    assert query == {
        "response_type": ["code"],
        "client_id": ["client-1"],
        "redirect_uri": ["https://app.example.test/callback"],
        "scope": ["account.read.self profile.read.owned"],
        "state": ["state-1"],
        "code_challenge": [session.code_challenge],
        "code_challenge_method": ["S256"],
    }
    assert session.code_verifier == "a" * 64
    assert session.scopes == ("account.read.self", "profile.read.owned")


def test_authorization_url_rejects_server_scope_for_delegated_flow() -> None:
    oauth = OAuthClient("https://skin.example.test", "client-1", redirect_uri="https://app/cb")

    with pytest.raises(InvalidScope) as exc:
        oauth.authorization_url([MinecraftScopes.SESSION_HASJOINED_SERVER], state="state-1")

    assert exc.value.invalid_scopes == ["minecraft_session.hasjoined.server"]
    assert str(exc.value) == "authorization code and device flows cannot request server or system scopes"


def test_exchange_code_posts_exact_form_and_saves_tokens(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(TOKEN_RESPONSE))
    store = MemoryTokenStore()
    oauth = OAuthClient(
        "https://skin.example.test",
        "client-1",
        client_secret="secret-1",
        token_store=store,
        transport=recorder.transport(),
    )

    tokens = oauth.exchange_code(code="auth-code-1", code_verifier="verifier-1")

    assert tokens.access_token == "access-token-1"
    assert tokens.refresh_token == "refresh-token-1"
    assert tokens.permissions == ("account.read.self", "profile.read.owned")
    assert store.load() == tokens
    assert len(recorder.requests) == 1
    request = recorder.requests[0]
    assert request.method == "POST"
    assert request.path == "/oauth/token"
    assert request.form == {
        "grant_type": ["authorization_code"],
        "client_id": ["client-1"],
        "code": ["auth-code-1"],
        "code_verifier": ["verifier-1"],
        "client_secret": ["secret-1"],
    }


def test_refresh_posts_exact_form_without_scope_when_not_requested(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(TOKEN_RESPONSE))
    oauth = OAuthClient("https://skin.example.test", "client-1", transport=recorder.transport())

    tokens = oauth.refresh("refresh-token-1", store=False)

    assert tokens.access_token == "access-token-1"
    assert recorder.requests[0].form == {
        "grant_type": ["refresh_token"],
        "client_id": ["client-1"],
        "refresh_token": ["refresh-token-1"],
    }


def test_start_device_flow_posts_exact_form(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(DEVICE_CODE_RESPONSE))
    oauth = OAuthClient("https://skin.example.test", "client-1", transport=recorder.transport())

    device = oauth.start_device_flow([ProfileScopes.READ_OWNED])

    assert device.device_code == "device-code-1"
    assert device.verification_uri_complete == "https://skin.example.test/oauth/device?user_code=ABCD-EFGH"
    assert device.permissions == ("profile.read.owned",)
    assert recorder.requests[0].path == "/oauth/device/code"
    assert recorder.requests[0].form == {
        "client_id": ["client-1"],
        "scope": ["profile.read.owned"],
    }


def test_exchange_device_code_maps_oauth_error_exactly(response_json) -> None:
    recorder = RequestRecorder(
        lambda request: response_json(
            {"error": "authorization_pending", "error_description": "authorization pending"},
            400,
        )
    )
    oauth = OAuthClient("https://skin.example.test", "client-1", transport=recorder.transport())

    with pytest.raises(OAuthError) as exc:
        oauth.exchange_device_code("device-code-1")

    assert exc.value.status_code == 400
    assert exc.value.error == "authorization_pending"
    assert exc.value.detail == "authorization pending"
    assert recorder.requests[0].form == {
        "grant_type": ["urn:ietf:params:oauth:grant-type:device_code"],
        "client_id": ["client-1"],
        "device_code": ["device-code-1"],
    }


def test_client_credentials_accepts_server_scope_and_posts_exact_form(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(CLIENT_CREDENTIALS_TOKEN_RESPONSE))
    oauth = OAuthClient(
        "https://skin.example.test",
        "server-client",
        client_secret="server-secret",
        transport=recorder.transport(),
    )

    tokens = oauth.client_credentials([MinecraftScopes.SESSION_HASJOINED_SERVER])

    assert tokens.access_token == "server-token-1"
    assert tokens.permissions == ("minecraft_session.hasjoined.server",)
    assert recorder.requests[0].form == {
        "grant_type": ["client_credentials"],
        "client_id": ["server-client"],
        "scope": ["minecraft_session.hasjoined.server"],
        "client_secret": ["server-secret"],
    }


def test_client_credentials_rejects_user_delegated_scope() -> None:
    oauth = OAuthClient("https://skin.example.test", "server-client")

    with pytest.raises(InvalidScope) as exc:
        oauth.client_credentials([AccountScopes.READ_SELF])

    assert exc.value.invalid_scopes == ["account.read.self"]
    assert str(exc.value) == "client credentials flow can only request public or server scopes"


def test_revoke_posts_exact_form_and_ignores_empty_body() -> None:
    recorder = RequestRecorder(lambda request: httpx.Response(204))
    oauth = OAuthClient(
        "https://skin.example.test",
        "client-1",
        client_secret="secret-1",
        transport=recorder.transport(),
    )

    oauth.revoke("access-token-1")

    assert recorder.requests[0].path == "/oauth/revoke"
    assert recorder.requests[0].form == {
        "token": ["access-token-1"],
        "client_id": ["client-1"],
        "client_secret": ["secret-1"],
    }
