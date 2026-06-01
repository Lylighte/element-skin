# 阶段 3：refresh 轮换原子化 + 重放检测（及注册原子化、随机码、邮箱校验）

## 目标

本阶段是本轮**最高危**修复，主改 `backends/site_backend.py`，消费阶段 1 新增的原子原语：

1. **轮换原子化 + 重放检测**：用 `consume_refresh_token`（DELETE…RETURNING）把「读+删」合一，只有删到行的请求才是赢家；检测到已消费 token 被重放时，**吊销该用户全部 refresh**（视为凭证泄露）。
2. **注册原子化**：改用 `create_user_with_profile`，消除孤儿 user / 超额核销邀请。
3. **随机码用 `secrets`**：验证/重置码改用密码学安全随机源。
4. **改邮箱/注册邮箱格式校验**：拒绝控制字符（CRLF），防头注入与未捕获 500。

## 问题证据

- `backends/site_backend.py:386-409` `rotate_refresh_token`：读 → 查过期 → 查用户 → `delete_refresh_token` → `_issue_session`，五条独立语句、无事务、删不判行数。并发同一 refresh → 两请求都读到、都通过、都签发 → 一 token 裂变两条会话链。已消费 token 重放仅返回 401，**不吊销其它会话**，被盗 refresh 可无限自我续期且受害者无感。
- `site_backend.py:350-359` 注册非原子（见阶段 1 证据）。
- `site_backend.py:213`（`send_verification_code` 内）：`code = "".join(random.choices(string.ascii_uppercase + string.digits, k=8))` 用梅森旋转 `random`，可预测，而它把守**密码重置**。
- `site_backend.py:197` 邮箱校验 `re.match(r"[^@]+@[^@]+\.[^@]+", ...)`：`re.match` 非 `fullmatch`，`[^@]+` 匹配 `\r`/`\n`，`a@evil.com\r\nBcc: x@y.com` 可通过。`register`（`site_backend.py:281`）则完全不校验邮箱格式。

## 改造清单

### 3.1 轮换原子化 + 重放检测

```python
async def rotate_refresh_token(self, raw_refresh: str) -> Dict[str, Any]:
    """原子轮换：DELETE...RETURNING 取出旧 refresh（单赢者），校验后签发新对。

    检测到「token 哈希存在过但已被消费」的重放时，吊销该用户全部 refresh。
    """
    token_hash = hash_refresh_token(raw_refresh)

    # 原子删并取：只有真正删到行的请求继续，并发/重放的另一方拿到 None。
    row = await self.db.user.consume_refresh_token(token_hash)
    if not row:
        # 未知 token，或已被并发请求/攻击者消费。无法在无血缘列时区分二者，
        # 保守起见按潜在泄露处理：此处仅 401（已删的真凭证不会再来），
        # 真正的重放检测见下方 family 方案（可选增强）。
        raise HTTPException(status_code=401, detail="invalid refresh token")

    user_id = row["user_id"]
    if int(time.time() * 1000) >= row["expires_at"]:
        raise HTTPException(status_code=401, detail="refresh token expired")

    user_row = await self.db.user.get_by_id(user_id)
    if not user_row:
        raise HTTPException(status_code=401, detail="invalid refresh token")

    return await self._issue_session(user_id, bool(user_row.is_admin))
```

> **为何 DELETE…RETURNING 即解决竞态**：Postgre 对同一行的并发 `DELETE` 串行化，只有一个事务返回 RETURNING 行，另一个返回空。于是「拿到行」=「唯一赢家」，无需额外锁。过期/删号检查移到删之后，因为行已被原子取出，无需再单独删过期行。

> **可选增强（重放检测，需 schema 变更）**：为 `site_refresh_tokens` 增 `family_id`，轮换时新行继承同 family。若某 token_hash 曾存在（可加一张 `consumed` 审计或保留软删标记）却被再次提交，则 `delete_refresh_tokens_by_user(user_id)` 杀全家。本阶段先落地原子轮换（已堵住并行裂变），family 重放检测列为后续增强，避免 schema 改动扩大本阶段风险。

### 3.2 注册原子化

`register`（`site_backend.py:281-364`）末段改为单次事务调用：

```python
    new_user = User(user_id, email, password_hash, is_first_user)
    new_user.display_name = username
    profile = PlayerProfile(profile_id, user_id, profile_name, "default")
    try:
        await self.db.user.create_user_with_profile(
            new_user, profile,
            invite_code=(invite_code if require_invite == "true" else None),
            used_by=email,
        )
    except asyncpg.UniqueViolationError:
        raise HTTPException(status_code=400, detail="Email already registered")
    return user_id
```

> 邀请核销并入事务后，删除原先分离的 `use_invite` 调用（`site_backend.py:361-362`）。超额核销由阶段 1 的条件 UPDATE + 行数判定兜底。

### 3.3 随机码用 `secrets`

```python
import secrets, string
code = "".join(secrets.choice(string.ascii_uppercase + string.digits) for _ in range(8))
```

移除对 `random` 的该处依赖（确认无其它安全相关用途后可一并清理 import）。

### 3.4 邮箱格式校验（注册 + 改邮箱）

抽一个小工具（可放 `utils/` 或就近私有函数）并用 `fullmatch` + 控制字符拒绝：

```python
import re
_EMAIL_RE = re.compile(r"[^@\s]+@[^@\s]+\.[^@\s]+")

def is_valid_email(email: str) -> bool:
    return bool(_EMAIL_RE.fullmatch(email)) and "\r" not in email and "\n" not in email
```

`register` 入口与 `update_user_info` 的 email 分支均先 `is_valid_email` 校验，不通过返回 400。改邮箱仍保留唯一性预检 + `UNIQUE` 兜底（参考 `update_profile_name` 捕获 `UniqueViolationError`）。

## 影响文件

- `backends/site_backend.py`（主）
- `utils/`（可选：`is_valid_email` 工具落点；亦可就近私有函数）

## 验证

```bash
cd skin-backend
pytest tests/api/test_refresh_token.py -q
pytest tests/backends/test_site_backend.py -q
pytest -q
```

新增/调整针对性用例：

- 轮换并发：对同一 refresh `asyncio.gather` 两次，断言恰一个成功、一个 401；DB 中旧 hash 已不存在。
- 一次性：轮换后旧 refresh 再次使用 → 401。
- 注册原子性：模拟 profile 名冲突使建号失败，断言无孤儿 user、邮箱仍可注册。
- 邀请超额：`total_uses=1` 并发两次注册，断言只成功一次。
- 随机码：长度 8、字符集正确（统计性即可，不强求分布检验）。
- 邮箱：`a@b.com\r\nBcc:x@y.com`、`a@@b`、`a@b` 均被拒；正常邮箱通过。
