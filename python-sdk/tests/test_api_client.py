from __future__ import annotations

import pytest

from element_skin_sdk import ElementSkinAPI
from element_skin_sdk.exceptions import APIError, PermissionDenied
from element_skin_sdk.models import TokenSet
from element_skin_sdk.permissions import AccountScopes, MinecraftScopes, ProfileScopes, TextureScopes

from .conftest import RequestRecorder
from .fixtures import ME_RESPONSE, MINECRAFT_HAS_JOINED_RESPONSE, PROFILE_PAGE_RESPONSE


def test_me_sends_bearer_token_and_returns_exact_body(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(ME_RESPONSE))
    api = ElementSkinAPI(
        "https://skin.example.test",
        token=TokenSet(
            access_token="access-token-1",
            token_type="Bearer",
            expires_in=3600,
            permissions=(AccountScopes.READ_SELF,),
        ),
        transport=recorder.transport(),
    )

    body = api.me()

    assert body == ME_RESPONSE
    assert recorder.requests[0].method == "GET"
    assert recorder.requests[0].path == "/v1/users/me"
    assert recorder.requests[0].headers["authorization"] == "Bearer access-token-1"


def test_local_permission_check_blocks_request_before_transport(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(ME_RESPONSE))
    api = ElementSkinAPI(
        "https://skin.example.test",
        access_token="access-token-1",
        permissions=(ProfileScopes.READ_OWNED,),
        transport=recorder.transport(),
    )

    with pytest.raises(PermissionDenied) as exc:
        api.me()

    assert exc.value.status_code == 403
    assert exc.value.detail == "missing required permission: account.read.self"
    assert exc.value.response_body == {
        "detail": "missing required permission",
        "missing_permissions": ["account.read.self"],
    }
    assert recorder.requests == []


def test_list_profiles_uses_exact_cursor_params(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(PROFILE_PAGE_RESPONSE))
    api = ElementSkinAPI(
        "https://skin.example.test",
        access_token="access-token-1",
        permissions=(ProfileScopes.READ_OWNED,),
        transport=recorder.transport(),
    )

    body = api.list_profiles(cursor="cursor-1", page_size=20)

    assert body == PROFILE_PAGE_RESPONSE
    assert recorder.requests[0].path == "/v1/users/me/profiles"
    assert recorder.requests[0].query == {"cursor": ["cursor-1"], "page_size": ["20"]}


def test_list_textures_uses_backend_texture_type_param(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json({"items": [], "has_next": False}))
    api = ElementSkinAPI(
        "https://skin.example.test",
        access_token="access-token-1",
        permissions=(TextureScopes.READ_OWNED,),
        transport=recorder.transport(),
    )

    body = api.list_textures(texture_type="skin", cursor="texture-cursor", page_size=10)

    assert body == {"items": [], "has_next": False}
    assert recorder.requests[0].path == "/v1/users/me/textures"
    assert recorder.requests[0].query == {
        "texture_type": ["skin"],
        "cursor": ["texture-cursor"],
        "page_size": ["10"],
    }


def test_update_texture_uses_hash_and_type_path_with_exact_body(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json({"hash": "hash-1", "type": "skin"}))
    api = ElementSkinAPI(
        "https://skin.example.test",
        access_token="access-token-1",
        permissions=(TextureScopes.UPDATE_OWNED,),
        transport=recorder.transport(),
    )

    body = api.update_texture("hash-1", "skin", note="A note", model="slim")

    assert body == {"hash": "hash-1", "type": "skin"}
    assert recorder.requests[0].method == "PATCH"
    assert recorder.requests[0].path == "/v1/users/me/textures/hash-1/skin"
    assert recorder.requests[0].json_body == {"note": "A note", "model": "slim"}


def test_minecraft_has_joined_posts_exact_json(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json(MINECRAFT_HAS_JOINED_RESPONSE))
    api = ElementSkinAPI(
        "https://skin.example.test",
        token=TokenSet(
            access_token="server-token-1",
            token_type="Bearer",
            expires_in=1800,
            permissions=(MinecraftScopes.SESSION_HASJOINED_SERVER,),
        ),
        transport=recorder.transport(),
    )

    body = api.minecraft_has_joined(username="Alice", server_id="server-hash", ip="127.0.0.1")

    assert body == MINECRAFT_HAS_JOINED_RESPONSE
    assert recorder.requests[0].method == "POST"
    assert recorder.requests[0].path == "/v1/minecraft/session/has-joined"
    assert recorder.requests[0].json_body == {
        "username": "Alice",
        "server_id": "server-hash",
        "ip": "127.0.0.1",
    }


def test_http_error_maps_detail_exactly(response_json) -> None:
    recorder = RequestRecorder(lambda request: response_json({"detail": "texture not found"}, 404))
    api = ElementSkinAPI(
        "https://skin.example.test",
        access_token="access-token-1",
        permissions=(TextureScopes.READ_OWNED,),
        transport=recorder.transport(),
    )

    with pytest.raises(APIError) as exc:
        api.get_texture("missing-hash", "skin")

    assert exc.value.status_code == 404
    assert exc.value.detail == "texture not found"
    assert exc.value.response_body == {"detail": "texture not found"}
