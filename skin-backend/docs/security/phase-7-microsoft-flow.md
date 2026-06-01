# 阶段 7：Microsoft 出站超时 + 导入绑定已验证会话

## 目标

主改 `routers/microsoft_routes.py` 与 `backends/microsoft_backend.py`：

1. **出站超时**：给所有 Microsoft/Xbox/Minecraft 调用加 `ClientTimeout`，避免上游挂死拖垮请求/连接（默认 5 分钟 × 5 次串联 ≈ 25 分钟）。
2. **导入绑定已验证会话**：`/import-profile` 不再信任客户端自由传入的 `profile_id`/`profile_name`，改为与 OAuth 一次性会话里**已验证**的 profile 比对，防止冒领任意正版 UUID/用户名。

> **本阶段是唯一涉及对外契约变化的阶段**：`/import-profile` 的入参语义收紧（不再接受任意 UUID/名）。需与前端协调——见下「前端协调」。故排在代码侧最后做。

## 问题证据

- `backends/microsoft_backend.py:74,99,133,172,189,211` 等处 `aiohttp.ClientSession()` 均无 `ClientTimeout`，默认 total 5 分钟。`complete_auth_flow`（`:236-248`）串联约 5 次调用。
- `routers/microsoft_routes.py:108-125` `microsoft_import_profile`：`profile_id`/`profile_name`/`skin_url`/`cape_url` 直接取自请求体，**未与** `/get-profile` 时验证并存入一次性会话的 `session_data["profile"]`（`microsoft_routes.py:88-96`）关联。任意登录用户可提交任意 UUID/用户名（仅受本地唯一性约束）→ 冒充正版身份。
- 对比 `/get-profile`（`microsoft_routes.py:80-106`）已正确做：`oauth_states.pop(ms_token)` 一次性取出、校验 `session_data["user_id"] == user_id`、只回显已验证 profile。

## 设计决策

- **超时**：每个 `ClientSession` 统一 `aiohttp.ClientSession(timeout=aiohttp.ClientTimeout(total=15))`，与 `utils/http.py`、`yggdrasil_client.py` 对齐。
- **导入绑定**：复用 `/get-profile` 的一次性 token 机制。导入时同样传 `ms_token`（或导入复用同一 token 流程），后端 `pop` 出 `session_data`，校验 `user_id` 归属，然后**以会话里的 `profile["id"]`/`name` 为准**，忽略/拒绝客户端传入的不一致值。`skin_url`/`cape_url` 同样应来自已验证 profile 的 skins/capes，而非自由字段。

## 改造清单

### 7.1 出站超时

`backends/microsoft_backend.py` 所有 `aiohttp.ClientSession()`：

```python
_MS_TIMEOUT = aiohttp.ClientTimeout(total=15)
# ...
async with aiohttp.ClientSession(timeout=_MS_TIMEOUT) as session:
    ...
```

（逐处替换；或抽一个建 session 的小工厂。）

### 7.2 导入绑定已验证会话

将导入改为消费一次性会话，而非自由入参。两种落地（择一，按前端现状定）：

- **方案 A（推荐，契约更清晰）**：`/import-profile` 接收 `ms_token`，后端 `pop` 会话、校验 `user_id`，从 `session_data["profile"]` 取 `id`/`name`/skins/capes 完成导入。客户端不再传 `profile_id`/`profile_name`/`skin_url`/`cape_url`。
- **方案 B（兼容现状字段，仅加校验）**：仍接收这些字段，但后端 `pop` 会话后**断言**客户端传入的 `profile_id`/`name` 与会话内一致，不一致返回 403；URL 必须命中会话内 skins/capes 列表。

```python
@router.post("/import-profile")
async def microsoft_import_profile(
    ms_token: str = Body(..., embed=True),
    payload: dict = Depends(get_current_user),
):
    user_id = payload.get("sub")
    session_data = oauth_states.pop(ms_token)        # 一次性
    if not session_data:
        raise HTTPException(status_code=400, detail="Invalid or expired token")
    if session_data["user_id"] != user_id:
        raise HTTPException(status_code=403, detail="Unauthorized")

    profile = session_data["profile"]["profile"]      # 已验证的正版 profile
    return await microsoft_backend.import_profile(
        user_id,
        profile["id"],
        profile["name"],
        _pick_skin_url(profile),                       # 从已验证 skins 取
        _pick_skin_variant(profile),
        _pick_cape_url(profile),                       # 从已验证 capes 取
    )
```

> 注意 `/get-profile` 用的是 `oauth_states.pop`（一次性）。若导入需在 get-profile 之后单独进行，需让 get-profile **不**消费 token、或导入用一个新的短效一次性 token 承接。实现时确认前端调用时序：是 `get-profile` 后立即 `import`，还是用户在前端选择后再 import。据此决定 token 是否在 get-profile 处 pop，避免「token 已被 get-profile 消费导致 import 取不到会话」。

## 前端协调

`/import-profile` 入参变化需前端配合：

- 方案 A：前端导入只发 `ms_token`，不再发 `profile_id` 等。
- 方案 B：前端可不变，但传入值必须与授权所得 profile 一致（本就如此，恶意篡改才会被拒）。

涉及前端文件（实现时定位）：Microsoft 导入相关的 `src/api/*.ts` 与调用组件。**本阶段实现前需先确认前端当前如何调用 `/import-profile`**，再决定 A/B。

## 影响文件

- `backends/microsoft_backend.py`（主：超时）
- `routers/microsoft_routes.py`（导入绑定）
- 前端 Microsoft 导入相关文件（方案 A 时）

## 验证

```bash
cd skin-backend
pytest tests/api/test_microsoft_import_api.py -q
pytest tests/backends/test_microsoft_backend.py -q
pytest -q
```

针对性用例：

- 伪造/篡改 `profile_id`（与会话不一致）→ 403（方案 B）或字段被忽略、以会话为准（方案 A）。
- 过期/已消费 `ms_token` → 400。
- 他人 `user_id` 的会话 → 403。
- 出站调用施加了 15s 超时（可检查 session 构造或用 mock 验证超时传播）。
