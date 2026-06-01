# 阶段 10：过期 refresh 周期清理 + 部署注记

## 目标

1. 主改 `routes_reference.py`：把过期 refresh token 清理从「仅启动一次」改为**周期性后台任务**。
2. 新建 `docs/deployment-notes.md`：把 CORS、Cookie `Secure`、CSRF 三项**配置/部署层**事项集中记录（这些不在本轮代码改动范围，因 `config.yaml` 不改且属反代/部署职责）。

## 问题证据

- `routes_reference.py:54` `lifespan` 启动段只调一次 `delete_expired_refresh_tokens(now)`。长跑服务上，过期行无限累积；每次登录/设备新增一行，除重启外不清理。
- 配置层（不在本轮代码改动范围，记录备查）：
  - CORS `allow_origins=["*"] + allow_credentials=True`（`routes_reference.py:73-81`，源自 `config.yaml`）：Starlette 会反射 Origin 并发 `Allow-Credentials: true`。当前靠 `SameSite=Lax` 兜底，一旦放宽 SameSite 或改用 header 鉴权即被直接利用。
  - Cookie `Secure` 由 `server.site_url` 是否 `https://` 决定（`jwt_utils.py:18-20`），默认 `http://` → 反代终止 TLS 但 `site_url` 仍 http 时，token cookie 不带 Secure。
  - 无 CSRF token，状态变更端点仅靠 `SameSite=Lax`。

## 改造清单

### 10.1 周期清理后台任务

`routes_reference.py` `lifespan`：

```python
import asyncio, time

async def _refresh_cleanup_loop(db, interval_seconds: int = 3600):
    """周期清理过期 refresh token。单实例运行，进程内任务即可。"""
    while True:
        try:
            await asyncio.sleep(interval_seconds)
            await db.user.delete_expired_refresh_tokens(int(time.time() * 1000))
        except asyncio.CancelledError:
            break
        except Exception:
            # 清理失败不应中断循环；记录后继续
            logger.warning("refresh token cleanup failed", exc_info=True)

@asynccontextmanager
async def lifespan(app: FastAPI):
    await db.connect()
    await db.init()
    await db.user.delete_expired_refresh_tokens(int(time.time() * 1000))  # 启动先清一次
    cleanup_task = asyncio.create_task(_refresh_cleanup_loop(db))
    try:
        yield
    finally:
        cleanup_task.cancel()
        try:
            await cleanup_task
        except asyncio.CancelledError:
            pass
        await db.close()
```

> 单实例约束下，进程内 `asyncio` 任务足够，无需外部定时器。间隔 1 小时可配置。
> 可选增强：每用户 refresh 行数上限（类似 `tokens` 的 `delete_surplus_tokens`），防单用户多设备无限累积。本阶段先做周期清理，上限留作后续。

### 10.2 部署注记文档

新建 `docs/deployment-notes.md`，至少含：

- **CORS**：生产应将 `config.yaml` 的 `cors.allow_origins` 设为明确白名单，**不可** `*` 与 `allow_credentials: true` 并用。
- **Cookie Secure**：生产 `server.site_url` 必须为 `https://`，否则 token cookie 不带 `Secure`；或在反代统一加 `Secure`。
- **CSRF**：当前仅 `SameSite=Lax` 兜底。如未来放宽 SameSite 或跨站场景，需引入 CSRF token / 严格 Origin 校验。
- **单实例约束**：OAuth state / settings 缓存 / fallback 缓存 / refresh 清理任务均为进程内，**只能单实例 / 单 worker**。
- **限流**：应用层限流以 `request.client.host` 为 key，在 `forwarded_allow_ips="*"` 下可被 `X-Forwarded-For` 伪造绕过。**异常流量应由 nginx 在更底层先行排除**（限速/连接数/封禁），应用层不承担重限流职责。`forwarded_allow_ips` 生产应设为真实反代网段而非 `*`。

> 本阶段**不修改 `config.yaml`**，仅以文档记录运营须知。

## 影响文件

- `routes_reference.py`（主：周期清理）
- `docs/deployment-notes.md`（新建）

## 验证

```bash
cd skin-backend
pytest -q
```

针对性验证：

- 启动后台任务存在且在关闭时被取消（可在测试中缩短 interval、断言清理被调用；或单测 `_refresh_cleanup_loop` 一轮后取消）。
- 写入一条已过期 refresh，触发一轮清理后该行消失。
