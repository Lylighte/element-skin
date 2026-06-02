import pytest
import asyncio
import time
import jwt
from datetime import datetime, timedelta, timezone

from utils.jwt_utils import JWT_SECRET, JWT_ALGO, hash_refresh_token


async def _login(client, email: str, password: str) -> dict:
    """登录并返回 {access_token, refresh_token, body}（从 Set-Cookie 提取）。"""
    resp = await client.post("/site-login", json={"email": email, "password": password})
    assert resp.status_code == 200
    return {
        "access_token": resp.cookies.get("access_token"),
        "refresh_token": resp.cookies.get("refresh_token"),
        "body": resp.json(),
    }


@pytest.mark.asyncio
async def test_refresh_rotates_token(client, user_factory):
    """刷新成功并轮换：返回的新 refresh 与旧 refresh 不同。"""
    email, password = "rotate@test.com", "Password123!"
    await user_factory(email=email, password=password)
    session = await _login(client, email, password)

    resp = await client.post("/me/refresh-token", cookies={"refresh_token": session["refresh_token"]})
    assert resp.status_code == 200
    assert resp.json()["is_admin"] is False

    new_refresh = resp.cookies.get("refresh_token")
    new_access = resp.cookies.get("access_token")
    assert new_refresh and new_access
    assert new_refresh != session["refresh_token"]  # 已轮换

    # 新 access 可访问受保护接口
    me_resp = await client.get("/me", cookies={"access_token": new_access})
    assert me_resp.status_code == 200


@pytest.mark.asyncio
async def test_old_refresh_rejected_after_rotation(client, user_factory):
    """轮换后旧 refresh 一次性作废，再次使用 → 401。"""
    email, password = "oneshot@test.com", "Password123!"
    await user_factory(email=email, password=password)
    session = await _login(client, email, password)

    first = await client.post("/me/refresh-token", cookies={"refresh_token": session["refresh_token"]})
    assert first.status_code == 200

    reused = await client.post("/me/refresh-token", cookies={"refresh_token": session["refresh_token"]})
    assert reused.status_code == 401


@pytest.mark.asyncio
async def test_refresh_works_without_valid_access(client, user_factory):
    """access 过期（此处直接不带 access）但 refresh 有效时，刷新仍成功。"""
    email, password = "expired@test.com", "Password123!"
    await user_factory(email=email, password=password)
    session = await _login(client, email, password)

    # 构造一个过期的 access，确认它无法访问 /me
    expired_access = jwt.encode(
        {
            "sub": session["body"]["user_id"],
            "is_admin": False,
            "type": "access",
            "exp": datetime.now(timezone.utc) - timedelta(minutes=1),
        },
        JWT_SECRET,
        algorithm=JWT_ALGO,
    )
    me_expired = await client.get("/me", cookies={"access_token": expired_access})
    assert me_expired.status_code == 401

    # 仅凭 refresh 即可刷新出新 access
    resp = await client.post("/me/refresh-token", cookies={"refresh_token": session["refresh_token"]})
    assert resp.status_code == 200


@pytest.mark.asyncio
async def test_missing_refresh_returns_401(client):
    """不带 refresh cookie 调刷新接口 → 401。"""
    resp = await client.post("/me/refresh-token")
    assert resp.status_code == 401


@pytest.mark.asyncio
async def test_logout_revokes_refresh(client, user_factory):
    """登出撤销 refresh，之后该 refresh → 401。"""
    email, password = "logout@test.com", "Password123!"
    await user_factory(email=email, password=password)
    session = await _login(client, email, password)

    logout = await client.post("/site-logout", cookies={"refresh_token": session["refresh_token"]})
    assert logout.status_code == 200

    resp = await client.post("/me/refresh-token", cookies={"refresh_token": session["refresh_token"]})
    assert resp.status_code == 401


@pytest.mark.asyncio
async def test_change_password_revokes_all_refresh(client, user_factory):
    """改密后该用户全部 refresh 失效。"""
    email, password = "chpw@test.com", "Password123!"
    await user_factory(email=email, password=password)
    session = await _login(client, email, password)

    chpw = await client.post(
        "/me/password",
        json={"old_password": password, "new_password": "NewPassword456!"},
        cookies={"access_token": session["access_token"]},
    )
    assert chpw.status_code == 200

    resp = await client.post("/me/refresh-token", cookies={"refresh_token": session["refresh_token"]})
    assert resp.status_code == 401


@pytest.mark.asyncio
async def test_deleted_user_refresh_rejected(client, user_factory, db_session):
    """删号后 refresh 失效（级联删除）→ 401。"""
    email, password = "deluser@test.com", "Password123!"
    await user_factory(email=email, password=password)
    session = await _login(client, email, password)

    await db_session.user.delete(session["body"]["user_id"])

    resp = await client.post("/me/refresh-token", cookies={"refresh_token": session["refresh_token"]})
    assert resp.status_code == 401


@pytest.mark.asyncio
async def test_concurrent_refresh_single_winner(client, user_factory, db_session):
    """阶段3：同一 refresh 并发轮换，恰一个成功、另一个 401，且旧 hash 已不存在。

    DELETE...RETURNING 让 Postgres 串行化同一行的并发 DELETE，只有一个事务
    RETURNING 出该行（赢家），另一方拿到 None → 401，杜绝一条 refresh 裂变两条会话链。
    """
    email, password = "race@test.com", "Password123!"
    await user_factory(email=email, password=password)
    session = await _login(client, email, password)
    old_refresh = session["refresh_token"]

    r1, r2 = await asyncio.gather(
        client.post("/me/refresh-token", cookies={"refresh_token": old_refresh}),
        client.post("/me/refresh-token", cookies={"refresh_token": old_refresh}),
        return_exceptions=True,
    )
    codes = sorted(
        r.status_code for r in (r1, r2) if not isinstance(r, Exception)
    )
    # 恰好一个 200、一个 401
    assert codes == [200, 401], f"unexpected codes: {codes}"

    # 旧 refresh 的 hash 已从库中删除
    old_hash = hash_refresh_token(old_refresh)
    row = await db_session.user.get_refresh_token(old_hash)
    assert row is None


@pytest.mark.asyncio
async def test_replayed_refresh_after_use_rejected(client, user_factory):
    """阶段3：已消费的 refresh 重放 → 401（一次性）。"""
    email, password = "replay@test.com", "Password123!"
    await user_factory(email=email, password=password)
    session = await _login(client, email, password)
    old_refresh = session["refresh_token"]

    first = await client.post("/me/refresh-token", cookies={"refresh_token": old_refresh})
    assert first.status_code == 200

    replay = await client.post("/me/refresh-token", cookies={"refresh_token": old_refresh})
    assert replay.status_code == 401
