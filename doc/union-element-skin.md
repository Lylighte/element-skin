# Element-Skin Union 实现手册

> 版本：v2.3.0-union-b2
> 最后更新：2026-06-03

## 1. 架构定位

element-skin 在 Union 联邦协议中扮演**成员站（Member Station）**角色，**不实现主站功能**。

```
┌──────────────────────┐     UnionHostVerify (RSA-SHA256)      ┌──────────────────────┐
│   MUA Union 主站      │ ──────POST /api/union/member/*──────→ │  element-skin 成员站   │
│   (BSS + PHP)        │ ←────GET /api/union/{serverlist,...}── │  (FastAPI + Docker)   │
│                      │        X-Union-Member-Key              │                      │
└──────────────────────┘                                       └──────────────────────┘
```

**职责边界**：
- element-skin 实现：`/api/union/member/*`（接收主站推送）、前端 Union 管理、OAuth2 令牌
- element-skin **不实现**：`/api/union/serverlist`、`/api/union/privatekey`、`/api/union/diagnose`——这些由主站 PHP 提供

---

## 2. 路由总览

### 2.1 入站路由（接收主站推送，UnionHostVerify 保护）

| 方法 | 路径 | 功能 | 文件 |
|------|------|------|------|
| POST | `/api/union/member/updatelist` | 主站推送：触发拉取 server list | `union_routes.py:41` |
| POST | `/api/union/member/updateprivatekey` | 主站推送：触发拉取共享私钥 | `union_routes.py:51` |
| POST | `/api/union/member/updatebackendkey` | **主站下发成员密钥** | `union_routes.py:61` |
| POST | `/api/union/member/sync` | 主站推送：触发角色同步 | `union_routes.py:74` |
| POST | `/api/union/member/remapuuid` | 主站推送：UUID 重新映射 | `union_routes.py:90` |
| POST | `/api/union/member/diagnose` | 诊断 Echo | `union_routes.py:101` |
| GET  | `/api/union/member/queryemail` | 主站查询用户邮箱（黑名单） | `union_routes.py:108` |

### 2.2 公开路由（无需认证）

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | `/api/union/member` | 成员站元数据（Hello） |

### 2.3 用户端路由（JWT Cookie 认证）

| 方法 | 路径 | 功能 |
|------|------|------|
| GET | `/union/profiles` | 角色绑定状态 |
| POST | `/union/bind` | 获取绑定 Token |
| POST | `/union/unbind` | 解绑角色 |
| POST | `/union/bindto` | 使用 Token 绑定 |
| POST | `/union/remapuuid` | 请求 UUID 同步 |
| GET | `/union/security/level` | 查看安全等级 |

### 2.4 管理端路由（JWT + Admin 认证）

| 方法 | 路径 | 功能 |
|------|------|------|
| GET/POST | `/admin/union/settings` | Union 配置管理 |
| POST | `/admin/union/update-list` | 手动触发拉取 server list |
| POST | `/admin/union/update-key` | 手动触发拉取私钥 |
| POST | `/admin/union/sync` | 手动触发角色同步 |
| POST | `/admin/union/diagnose` | 手动触发诊断 |
| POST | `/admin/union/generate-keypair` | 生成 OAuth2 密钥对 |
| GET/POST/DELETE | `/admin/union/blacklist*` | 黑名单管理 |

---

## 3. UnionHostVerify 签名验证

### 3.1 流程

```
主站签名请求 ────→ element-skin 成员站
  │                      │
  │ X-Message-Signature  │ 1. 检查三个 Header 存在（否则 401）
  │ X-Message-Timestamp  │ 2. 检查 Nonce 未重复（否则 401）
  │ X-Message-Nonce      │ 3. 检查时间戳窗口 [-10s, +30s]（否则 401）
  │                      │ 4. GET {union_api_root} 获取 union_host_signature_public_key
  │                      │ 5. RSA-SHA256 + PKCS1v15 验证签名
  │                      │    signed_data = body + timestamp + nonce
  │                      │ 6. 记录 Nonce（防重放）
  │                      │ 7. 缓存 body 到 request.state.union_body
  └──────────────────────┘
```

### 3.2 实现文件

| 组件 | 文件 | 行号 |
|------|------|------|
| 签名验证 | `backends/union_backend.py` | `verify_union_request_inbound()` L79-112 |
| 签名计算 | `backends/union_backend.py` | `verify_union_signature()` L491-514 |
| 公钥获取 | `backends/union_backend.py` | `get_union_public_key()` L440-466 |
| 缓存 60s | `backends/union_backend.py` | `_union_public_key_cache` L36 |
| 依赖注入 | `routers/union_routes.py` | `verify_union_request()` L32-35 |

---

## 4. 配置项

### 4.1 config.yaml

```yaml
# 是否使用 Union 下发的 Yggdrasil 私钥（替代默认密钥）
keys:
  use_union_key: false   # 默认 false

# Union 主站 API 根地址（主站提供）
union:
  api_root: "https://主站地址/api/union"

server:
  root_path: "/skinapi"  # 主路径前缀；额外路径（/api/yggdrasil/、/api/union/member/）由 nginx 处理
  site_url: "https://你的域名"
```

### 4.2 数据库配置（settings 表，实时生效无需重启）

| Key | 说明 | 来源 |
|-----|------|------|
| `union_api_root` | 主站 API 根地址 | Admin 面板填写 |
| `union_member_key` | 主站分配的成员密钥 | 主站通过 `updatebackendkey` 下发，或手动写库 |
| `union_enable_update` | 是否接受主站推送 | 默认 "true" |
| `union_enable_oauth2` | 是否启用 OAuth2 | 默认 "true" |

### 4.3 密钥文件

| 文件 | 说明 |
|------|------|
| `/app/data/private.pem` | 默认 Yggdrasil 私钥 |
| `/app/data/public.pem` | 默认 Yggdrasil 公钥 |
| `/app/data/union-ygg-private.pem` | Union 下发的共享私钥（`use_union_key=true` 时使用） |

---

## 5. Nginx 配置

### 5.1 生产环境推荐配置

```nginx
server {
    listen 80;
    server_name yourdomain.com;
    root /your/path/to/frontend;
    index index.html;

    # 前端 API 调用
    location /skinapi/ {
        proxy_pass http://localhost:8000/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # Union 成员路由 → element-skin docker
    location /api/union/member/ {
        proxy_pass http://localhost:8000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # SPA 兜底
    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

**重要**：
- **不要**用 `location /api/union/` ——会将主站的 `/api/union/serverlist` 等端点劫持到 element-skin
- 仅代理 `/api/union/member/` 成员路由
- 主站自身的 `/api/union/*` 端点由主站 PHP 处理

---

## 6. 部署流程

### 6.1 首次接入 Union

1. **部署 element-skin**（Docker Compose，nginx 配置如上）
2. **通知主站管理员**，添加你的站点为成员并下发 `union_member_key`
3. **验证 key 到达**：查看 Admin → Union Settings，或查库
   ```sql
   SELECT value FROM settings WHERE key = 'union_member_key';
   ```
4. **等待主站推送**或手动在 Admin 面板点同步
5. **验证成功**：后台日志 `updatelist`/`updateprivatekey`/`sync` 全部返回 200

### 6.2 启用 Union 共享私钥（可选）

```yaml
keys:
  use_union_key: true
```

重启后 element-skin 使用 Union 下发的共享私钥。前提是主站已下发过 key（`updateprivatekey` 返回 200），文件 `/app/data/union-ygg-private.pem` 已存在。

---

## 7. 故障排查

### 7.1 `POST /api/union/member/*` 返回 401

- 检查 `union_api_root` 是否配置正确
- 确认主站的 `GET {union_api_root}` 返回包含 `union_host_signature_public_key`
- 确认主站使用正确的私钥签名

### 7.2 `POST /api/union/member/updatelist` 返回 502

**主因**：`union_member_key` 为空或无效。
验证：
```bash
docker exec element-skin python3 -c "
import urllib.request
r = urllib.request.urlopen('https://主站/api/union/serverlist')
# 无 key → 403；有 key 应返回 200
"
```

### 7.3 `POST /api/union/member/sync` 返回 422

**已修复**（v2.3.0-union-b2）：body 双重消费 bug。`verify_union_request` 消费了 body 后，FastAPI `Body()` 读到空流。

### 7.4 Admin 诊断返回 404

**已修复**：nginx `location /api/union/` 过宽，将主站 `/diagnose` 劫持到 element-skin。改为 `location /api/union/member/` 解决。

### 7.5 `GET /api/union/diagnose` 返回 404

element-skin **没有**这个路由（这是主站端点）。Admin 诊断的正确路径是 `POST /skinapi/admin/union/diagnose`（JWT 认证），内部调用 `union_backend.trigger_diagnose()` 向主站发起诊断。

---

## 8. 与 PHP 参考实现的差异

| 特性 | PHP (yggdrasil-connect) | element-skin |
|------|------------------------|-------------|
| Yggdrasil API 路由前缀 | 硬编码 `/api/yggdrasil` | 完全可配置（`root_path` + nginx） |
| Union 路由前缀 | 框架层面 `Route::prefix` | 绝对路径 |
| Nonce 存储 | Laravel Cache (60s) | PostgreSQL `union_nonces` 表 |
| 公钥缓存 | HTTP 响应缓存 | 内存缓存 60s |
| 成员密钥存储 | BSS `options` 表 | PostgreSQL `settings` 表 |
| 私钥存储 | BSS `options` 表 (ygg_private_key) | 文件 `/app/data/union-ygg-private.pem` |
| 架构 | BSS 插件 | 独立 FastAPI 应用 |
| Yggdrasil 端点路径 | 主站 + 成员站在同一 BSS 实例 | 前端（`/skinapi/`）+ 成员站（`/api/union/member/`）+ Yggdrasil（`/api/yggdrasil/`）通过 nginx 分流 |

---

## 9. 相关文件清单

| 文件 | 用途 |
|------|------|
| `skin-backend/routers/union_routes.py` | 所有 Union 路由（5 组 28 个端点） |
| `skin-backend/backends/union_backend.py` | Union 业务逻辑、签名验证、出站 HTTP |
| `skin-backend/database_module/modules/union.py` | Union 数据库模块（settings 缓存、nonce） |
| `skin-backend/routes_reference.py` | App 装配、UnionBackend 初始化 |
| `skin-backend/config.yaml` | 配置模板 |
| `skin-backend/entrypoint.sh` | 启动脚本（Union key 模式判断） |
| `nginx-host.conf` | Nginx 模板 |
| `doc/union-protocol-php.md` | PHP Union 协议规范（参考） |
| `doc/union-element-skin.md` | 本文档 |
| `element-skin/src/components/admin/AdminUnion.vue` | Admin Union 管理页面 |
| `element-skin/src/components/dashboard/DashboardUnion.vue` | 用户 Union 角色绑定页面 |
| `element-skin/src/api/union.ts` | 前端 Union API 层 |
| `element-skin/src/router/index.ts` | 前端路由定义 |

---

## 10. 版本历史

### v2.3.0-union-b2（2026-06-02）

**密钥管理重构**：Yggdrasil 私钥从数据库迁移到文件存储（`/app/data/union-ygg-private.pem`），新增 SHA-256 指纹计算与展示，`use_union_key` 配置开关支持启动时自动选用 Union 密钥，Admin 面板展示密钥指纹而非完整 PEM。重命名 `ygg_private_key` → `union_ygg_private_key`（7 文件 +30/−30）。新增 Union key 管理集成测试与 fallback 测试。

**关键 Bug 修复**：修复 `verify_union_request_inbound` 消费 `request.body()` 后下游 handler `Body()` 读到空流的 422 错误（影响 `updatebackendkey`、`sync`、`remapuuid`、`diagnose`）；修复 Nginx `location /api/union/` 过宽导致劫持主站 `/diagnose`、`/serverlist` 端点，改为仅代理 `/api/union/member/`；兼容主站 sync body 的数组 `[]` 与对象 `{"profileList":{}}` 两种格式。

**前端优化**：从 Dashboard 导航菜单移除「角色绑定」，改为仅通过「角色管理」页面的「Union 角色」按钮访问；补全 Admin 桌面导航「更多设置」组中缺失的 Union 入口；修复 DashboardUnion 页面布局与其他 Dashboard 页面不一致的问题；新增 `btn-gradient-pink` 按钮样式。

---

### v2.3.0-union-b1（2026-06-01）

**初始 Union 实现**：完整的 Union 联邦协议成员站功能，包括 UnionModule 与数据库 schema、5 组 28 个路由端点（入站 UnionHostVerify、公开元数据、OAuth2、用户端 JWT、管理端 Admin）、UnionBackend 业务逻辑层、签名验证（RSA-SHA256 + PKCS1v15）、出站 HTTP 调用（`X-Union-Member-Key` 认证）。

**前端实现**：Admin Union 管理页面（API 根地址、成员密钥、OAuth2 密钥、server list、私钥指纹、黑名单管理）、Dashboard Union 角色绑定页面（绑定/解绑/UUID 同步/安全等级）、Vue Router 路由定义、`@/api/union.ts` API 层。

**Profile 同步**：角色增删改操作自动向主站同步（`profile/add`、`profile/{uuid}` PUT、`profile/{uuid}` DELETE），UUID 重新映射支持。

**协议文档**：新增 PHP Union 联邦协议规范文档（`doc/union-protocol-php.md`），修复 5 处不准确描述，补充 Nginx 生产配置（Union + Yggdrasil 规则）。

**其他**：Union 路由改用 Cookie-based JWT 认证（非 Bearer header），支持双 Yggdrasil discovery 路径，CI 手动触发构建。
