# 后端安全修复计划（第二轮）

承接上一轮（`access token + refresh token` 模型上线后）的复审。本轮目标：**关闭新引入的会话/令牌可利用缺口、修正数据库层的原子性与越权问题、收敛资源耗尽面**，且不改变对外 API 契约（唯一例外见阶段 7，已显式标注需前端配合）。

> **限流不在本轮范围**：按业主决策，应用层限流不应做得过重，异常流量应由 nginx 在更底层先行排除。`utils/rate_limiter.py` 的 `request.client.host` keying 问题（XFF 伪造绕过）留给反代/网关层处理，本计划不改限流代码。

每个阶段是一个独立文件，**一个阶段对应一个主改动文件**，可单独执行、单独验证、单独提交。阶段之间尽量解耦，仅少数存在软依赖（见「执行顺序」）。

## 背景：复审结论

并行复审了 `backends/`、`routers/`、`database_module/`、`utils/`、`services/` 后，确认下列问题（已剔除经判定无需处理的项：SHA-1/MD5 是 Yggdrasil 协议强制、删材质不删盘文件、封禁不锁主站）。

按严重程度：

- **严重**：refresh token 轮换非原子且无重放检测——同一 refresh 被并发使用会裂变成两条有效会话链，被盗 refresh 可与正版并行续期且受害者无感。
- **高**：衣柜接口可越权把他人**私有**材质拷入自己衣柜（IDOR）；管理员重置密码不吊销既有会话；JWT 密钥有硬编码默认值且启动不校验；纹理下载先全量读入内存再校验大小（用户可控 URL → 内存 DoS）。
- **中**：邀请码超额核销（TOCTOU）；分页 `limit` 无上下界（DoS / 500 / 死循环）；Microsoft 导入信任客户端传入的 UUID/用户名（冒领正版身份）；验证/重置码用 `random` 而非 `secrets`；profile 删除非原子、留孤儿 token；注册非原子留孤儿 user；过期 refresh 仅启动清理一次；Microsoft 出站调用无超时。
- **低**：异常细节回显；缺索引；`delete_profile`/`update_profile_name` 不检查影响行数；refresh cookie 作用域过宽。

> 关于 JWT 密钥：沿用上一轮决策，**继续只从配置文件读取**，不引入环境变量覆盖、不修改 `config.yaml`。本轮只在**启动时**对密钥做 fail-fast 校验（缺失/等于默认值/过短即拒绝启动）。

## 修复原则

- **行为不变优先**：除明确的安全收紧，不改对外 API 形状。阶段 7（Microsoft 导入绑定）是唯一契约变化，已单独标注并附前端协调说明。
- **原子用事务**：凡「读—判断—写」或多条相互依赖的写，统一收进 `async with conn.transaction()`，由数据库保证单赢者语义。
- **一个阶段一个文件**：主改动集中在单文件，少数因耦合需触及第二文件的，已在该阶段「影响文件」中显式列出。
- **小步提交**：一个阶段一个提交，便于 review 与回滚。

## 阶段总览

| 阶段 | 主文件 | 主题 | 严重度 | 风险 |
|------|--------|------|--------|------|
| 1 | [phase-1-db-atomicity.md](./phase-1-db-atomicity.md) | DB 层原子性与返回值（轮换/邀请/级联删/注册/行数） | 严重·中 | 中 |
| 2 | [phase-2-wardrobe-idor.md](./phase-2-wardrobe-idor.md) | 衣柜越权：`add_to_user_wardrobe` 校验可见性 | 高 | 低 |
| 3 | [phase-3-refresh-rotation.md](./phase-3-refresh-rotation.md) | refresh 轮换原子化+重放检测、注册原子化、随机码、邮箱校验 | 严重·中 | 中 |
| 4 | [phase-4-admin-account.md](./phase-4-admin-account.md) | 管理员重置吊销会话、级联删 profile | 高·中 | 低 |
| 5 | [phase-5-jwt-secret-guard.md](./phase-5-jwt-secret-guard.md) | JWT 密钥启动 fail-fast（+ refresh cookie 收窄） | 高·低 | 低 |
| 6 | [phase-6-download-cap.md](./phase-6-download-cap.md) | 纹理下载内存硬上限（流式+提前拒绝） | 高 | 低 |
| 7 | [phase-7-microsoft-flow.md](./phase-7-microsoft-flow.md) | Microsoft 出站超时 + 导入绑定已验证会话 | 中 | 中（需前端配合） |
| 8 | [phase-8-pagination-clamp.md](./phase-8-pagination-clamp.md) | 分页 `limit` 上下界 clamp | 中 | 低 |
| 9 | [phase-9-indexes.md](./phase-9-indexes.md) | 补热路径索引 | 低 | 低 |
| 10 | [phase-10-ops-and-notes.md](./phase-10-ops-and-notes.md) | 过期 refresh 周期清理 + 部署注记（CORS/Secure/CSRF） | 中·低 | 低 |
| 11 | [phase-11-error-leak.md](./phase-11-error-leak.md) | 收敛导入异常回显 | 低 | 低 |

## 执行顺序

推荐顺序：**1 → 2 → 3 → 4 → 5 → 6 → 8 → 9 → 11 → 7 → 10**。

- **阶段 1** 是基础层：它新增 `consume_refresh_token`（原子删+返回）、`delete_profile_cascade`、`create_user_with_profile`、以及邀请码原子核销，**阶段 3、4 依赖这些方法**，故最先做。
- **阶段 2** 高危且完全独立，紧随其后立即堵上私有材质越权。
- **阶段 3、4** 消费阶段 1 的新方法，落地轮换与账号侧加固。
- **阶段 5、6** 是低风险高收益的独立加固。
- **阶段 8、9、11** 改动面极小、风险低，可穿插随时做。
- **阶段 7** 涉及契约/前端协调，放在代码侧最后；**阶段 10** 收尾运维与部署注记。

## 通用验证

每阶段执行后：

```bash
cd skin-backend
pytest -q                    # 全量（基线 212 通过）
pytest tests/api -q          # 接口层（行为契约）
```

通过标准：无新增失败；每阶段额外列出的针对性用例通过。涉及安全收紧的阶段，需手动验证「攻击路径被拒、正常路径不受影响」。

## 部署约束

后端在进程内维护多份内存状态（OAuth state、settings 缓存、fallback 缓存），**不跨进程共享**，因此仍**只能单实例 / 单 worker 运行**。CORS、Cookie `Secure`、CSRF 三项属配置/部署层（且 `config.yaml` 不在本轮改动范围），统一记录在阶段 10 的部署注记中。
