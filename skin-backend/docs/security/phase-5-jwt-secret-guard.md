# 阶段 5：JWT 密钥启动 fail-fast（+ refresh cookie 收窄）

## 目标

主改 `utils/jwt_utils.py`：在**启动时**对 JWT 密钥做 fail-fast 校验，杜绝「沿用硬编码默认密钥 → 任意伪造 access token」的致命路径；顺带收窄 refresh cookie 作用域。

> 沿用业主决策：**密钥继续只从配置文件读取**，不引入环境变量覆盖，**不修改 `config.yaml`**。本阶段只加校验逻辑与一处 cookie 配置，绝不改 `config.yaml`。

## 问题证据

- `utils/jwt_utils.py:11`：`JWT_SECRET = config.get("jwt.secret", "dev-secret-default-key-at-least-32-chars-long")` 有硬编码 fallback，HS256。
- `decode_access_token` 只校验签名 + `exp` + `type=="access"`（`jwt_utils.py:69-79`），`get_current_user` 只查用户存在（`deps.py:44`）。知道密钥即可伪造任意 `sub` 的 token → 账号/管理员接管。
- 启动期无任何校验，运营若忘记改密钥，服务照常起来且漏洞静默存在。
- `jwt_utils.py:46` refresh cookie `path="/"`：长效 refresh 随每个请求发往每条路径，暴露面偏大。

## 设计决策

- **fail-fast 触发条件**：密钥缺失、等于已知默认值、或字节长度 `< 32`。任一命中即在启动期抛错终止，不拖到运行期变神秘 500。
- **落点**：在 `jwt_utils` 模块加载时即校验（模块级），或提供 `assert_jwt_secret_ok()` 由 `routes_reference.py` 的 `lifespan` 启动段调用。**推荐模块级**——任何 import `jwt_utils` 的路径（含测试）都会触发，最早暴露问题。但需保证测试环境提供了合规密钥（见验证）。
- **refresh cookie path**：收窄到刷新端点 `/me/refresh-token`，登出删 cookie 的 path 必须同步一致，否则删不掉。

## 改造清单

### 5.1 启动校验

`utils/jwt_utils.py`：

```python
_DEFAULT_SECRET = "dev-secret-default-key-at-least-32-chars-long"
JWT_SECRET = config.get("jwt.secret", _DEFAULT_SECRET)
JWT_ALGO = "HS256"

def _validate_jwt_secret(secret: str) -> None:
    if not secret:
        raise RuntimeError("jwt.secret 未配置：请在配置文件中设置高熵密钥后再启动")
    if secret == _DEFAULT_SECRET:
        raise RuntimeError("jwt.secret 仍为默认占位值：必须改为随机高熵密钥")
    if len(secret.encode("utf-8")) < 32:
        raise RuntimeError("jwt.secret 过短：至少 32 字节")

_validate_jwt_secret(JWT_SECRET)
```

> 若顾虑模块级抛错影响某些工具脚本（如 `gen_key.py`、迁移脚本）的 import，可改为 `assert_jwt_secret_ok()` 函数 + 在 `lifespan` 调用。二者取一，README 已注明推荐模块级。**实现时先确认测试 fixture/`conftest.py` 提供了合规密钥**，否则全测试套件会在 import 期失败。

### 5.2 refresh cookie 收窄

```python
def get_refresh_cookie_settings() -> dict:
    expire_days = int(config.get("jwt.expire_days", "7"))
    return {
        "key": "refresh_token",
        "value": "",
        "httponly": True,
        "secure": _secure_cookie(),
        "samesite": "lax",
        "max_age": expire_days * 24 * 3600,
        "path": "/me/refresh-token",   # 收窄：仅刷新端点携带
    }
```

并在 `routers/site_routes.py` 登出删 cookie 处对齐 `path="/me/refresh-token"`，否则浏览器不会删除该 cookie。

> **注意**：前端 `client.ts` 调刷新用的是 `apiClient.post('/me/refresh-token')`，加上 `baseURL`（生产 `/skinapi`）后实际路径为 `/skinapi/me/refresh-token`。若后端挂在 `root_path`/反代前缀下，cookie path 必须是**浏览器可见的完整路径**而非应用内路径。生产存在 `root_path` 时，此项需按部署前缀调整，否则刷新请求不带 refresh cookie → 刷新永远失败。**若不确定部署前缀，本阶段可暂缓 5.2、只做 5.1**，把 cookie path 收窄留到阶段 10 的部署注记里连同 Secure/CORS 一起定。

## 影响文件

- `utils/jwt_utils.py`（主）
- 必要时 `routers/site_routes.py`（登出删 cookie 的 path 对齐）

## 验证

```bash
cd skin-backend
pytest -q
```

针对性验证：

- 临时把测试密钥置为默认值/空/短串，确认 import 或启动抛 `RuntimeError`（可用 `monkeypatch` + `importlib.reload`，或直接单测 `_validate_jwt_secret`）。
- 合规密钥下全套测试照常通过（确认 `conftest.py` 已设合规 `jwt.secret`）。
- 若启用 5.2：手动验证登录后 refresh cookie 的 `Path=/me/refresh-token`，且 `/me/refresh-token` 请求确实带上该 cookie、登出后被清除。
