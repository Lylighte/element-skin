# 阶段 1：数据库层原子性与返回值

## 目标

修正 DB 层一切「读—判断—写」缺乏原子保证、以及写操作不回报影响行数的问题。本阶段**只动 `database_module/modules/user.py`**（外加 `database_module/modules/texture.py` 中邀请相关如有），为后续阶段提供可靠的原子原语：

1. 新增 `consume_refresh_token(token_hash)`：用 `DELETE ... RETURNING` 原子「删并取」，是阶段 3 轮换的基石。
2. 新增 `delete_profile_cascade(profile_id)`：事务内同时删 profile 及其 Yggdrasil token，供阶段 4 用。
3. 新增 `create_user_with_profile(...)`：事务内建 user + profile（+ 可选核销邀请），供阶段 3 注册原子化用。
4. `use_invite` 改为自带条件的原子核销，并回报是否成功（防超额核销 TOCTOU）。
5. `delete_profile` / `update_profile_name` 解析 command tag，按真实影响行数返回。

> 本阶段只新增/修正 DB 方法，**不改调用方**。调用方切换在阶段 3、4。这样阶段 1 可独立提交且不改变现有行为（新方法尚无人调用，旧方法签名兼容）。

## 问题证据

- `database_module/modules/user.py:462-463` `delete_refresh_token`：`await self.db.execute(...)` 不返回 command tag，调用方无法判断是否真的删到行 → 阶段 3 的轮换无法做单赢者判定。
- `use_invite`（`user.py:508-518`）：`UPDATE invites SET used_count = used_count + 1 WHERE code=$1` 无条件自增，与 `site_backend.py:321-331` 的「先查 `used_count >= total_uses`」分离 → 并发注册可让 `total_uses=1` 的码被多次核销（TOCTOU）。
- `admin_backend.py:130-131` 删 profile：`delete_tokens_by_profile` 与 `delete_profile` 两条独立自动提交语句，非原子；`site_backend.py:574` 删 profile 完全不删其 token（`tokens` 表无 `profile_id` FK，见 `initsql.py:42-48`）→ 孤儿 token。
- `site_backend.py:350-359` 注册：`user.create()` 成功后 `create_profile` 在 try 之外，profile 失败留下已提交的孤儿 user，邮箱被永久占用。
- `user.py:340-343` `delete_profile` 返回 `result is not None`——`conn.execute` 即使 0 行也返回 `"DELETE 0"`（非 None），恒为 `True`；`update_profile_name`（`user.py:372-378`）0 行更新也返回 `True`。契约错误。
- 已确认 `BaseDB.execute` 透传 `conn.execute` 的 command tag 字符串（`core.py:45-48`），asyncpg 形如 `"DELETE 1"` / `"UPDATE 0"`，可 `.split()[-1]` 取行数。

## 改造清单

### 1.1 `consume_refresh_token`：原子删并取

`user.py`，在现有 refresh token 方法区新增（保留旧 `delete_refresh_token` 供登出/级联用）：

```python
async def consume_refresh_token(self, token_hash: str):
    """原子地删除并返回该 refresh token 行（DELETE ... RETURNING）。

    返回被删行（含 user_id, expires_at）；若 token 不存在或已被并发请求消费，
    返回 None。轮换的单赢者语义即建立在「恰好删到一行的请求才是赢家」之上。
    """
    return await self.db.fetchrow(
        "DELETE FROM site_refresh_tokens WHERE token_hash=$1 "
        "RETURNING token_hash, user_id, expires_at, created_at",
        token_hash,
    )
```

### 1.2 `delete_profile_cascade`：事务内删 profile + token

```python
async def delete_profile_cascade(self, profile_id: str):
    """事务内删除 profile 及其 Yggdrasil 游戏 token，避免孤儿 token。"""
    async with self.db.get_conn() as conn:
        async with conn.transaction():
            await conn.execute("DELETE FROM tokens WHERE profile_id=$1", profile_id)
            result = await conn.execute("DELETE FROM player_profiles WHERE uuid=$1", profile_id)
    return result.split()[-1] != "0"
```

> 表/列名以仓库现状为准（删 token 用现 `delete_tokens_by_profile` 的同款 SQL，删 profile 用现 `delete_profile` 的同款 SQL）。本阶段实现时先 grep 现有两条 SQL 原样搬入事务，避免改写。

### 1.3 `create_user_with_profile`：注册原子化

```python
async def create_user_with_profile(self, user, profile, invite_code: str | None = None, used_by: str | None = None):
    """事务内创建 user + profile（可选核销邀请），任一失败整体回滚。

    返回 True；邮箱/角色名唯一冲突抛 asyncpg.UniqueViolationError 由上层转 400。
    """
    async with self.db.get_conn() as conn:
        async with conn.transaction():
            # 内联现有 create / create_profile 的 INSERT（参数顺序照搬现状）
            await conn.execute("<INSERT INTO users ...>", ...)
            await conn.execute("<INSERT INTO player_profiles ...>", ...)
            if invite_code:
                updated = await conn.execute(
                    "UPDATE invites SET used_count = used_count + 1 "
                    "WHERE code=$1 AND (total_uses IS NULL OR used_count < total_uses)",
                    invite_code,
                )
                if updated.split()[-1] == "0":
                    raise HTTPException(status_code=400, detail="invite code has no remaining uses")
                if used_by:
                    await conn.execute(
                        "UPDATE invites SET used_by=$1 WHERE code=$2 AND used_by IS NULL",
                        used_by, invite_code,
                    )
    return True
```

> 邀请核销并入同一事务，使「建号」与「耗码」同生共死——这同时解决 1.4 的超额核销（原子条件 UPDATE + 行数判定）。

### 1.4 `use_invite` 原子化（独立路径兜底）

若仍保留独立 `use_invite`（非注册场景调用），同样改为条件自增并回报：

```python
async def use_invite(self, code: str, used_by: str = None) -> bool:
    async with self.db.get_conn() as conn:
        async with conn.transaction():
            updated = await conn.execute(
                "UPDATE invites SET used_count = used_count + 1 "
                "WHERE code=$1 AND (total_uses IS NULL OR used_count < total_uses)",
                code,
            )
            if updated.split()[-1] == "0":
                return False
            if used_by:
                await conn.execute(
                    "UPDATE invites SET used_by=$1 WHERE code=$2 AND used_by IS NULL",
                    used_by, code,
                )
    return True
```

### 1.5 写操作回报真实行数

```python
async def delete_profile(self, profile_id: str) -> bool:
    result = await self.db.execute("DELETE FROM player_profiles WHERE uuid=$1", profile_id)
    return result.split()[-1] != "0"

async def update_profile_name(self, ...) -> bool:
    try:
        result = await self.db.execute("<UPDATE ...>", ...)
    except asyncpg.UniqueViolationError:
        return False
    return result.split()[-1] != "0"
```

## 影响文件

- `database_module/modules/user.py`（主）
- 必要时 `database_module/modules/texture.py`（若邀请相关方法位于此处——实现时以 grep 定位为准）

## 验证

```bash
cd skin-backend
pytest tests/database/test_user.py -q
pytest -q
```

新增针对性用例（`tests/database/test_user.py`）：

- `consume_refresh_token`：存在→返回行且二次调用返回 None（验证一次性）。
- `delete_profile_cascade`：删后 profile 与其 token 同时消失。
- `use_invite`：`total_uses=1` 被消费一次后再次调用返回 `False`，`used_count` 不超过 `total_uses`。
- `delete_profile` / `update_profile_name`：对不存在的 id 返回 `False`。
- 并发模拟：对同一 invite/refresh 并发调用，断言只有一个赢家（可用 `asyncio.gather` + 计数）。
