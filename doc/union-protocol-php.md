# Union 联合认证网络协议规范（PHP 参考实现）

> 基于 MUAlliance/LittleSkin 的 `yggdrasil-connect` Blessing Skin 插件。
> 本文档面向需要理解或重新实现该协议的后端开发者。

---

## 1. 概述

Union 是 Blessing Skin 服务器之间的联合认证网络协议。它让多个独立的 Blessing Skin 站点组成一个联合认证网络，实现跨站点的身份互认、OAuth2 联合登录、黑名单共享和 UUID 重映射。

**角色模型：**

| 角色 | 说明 |
|------|------|
| **Union 主站（Hub）** | 联合网络的核心节点，维护服务器列表、分发密钥、协调跨站操作 |
| **成员站（Member）** | 加入联合网络的 Blessing Skin 站点，受 Union 主站管理 |

**核心能力：**

- **跨站身份互认**：一个站点的角色可以在其他站点被识别
- **OAuth2 联合登录**：用户在一个站点登录后可以无缝跳转到另一个站点
- **黑名单共享**：Union 主站维护全局黑名单，成员站可以查询和提交
- **UUID 重映射**：Union 主站可以协调不同站点间的 UUID 映射关系
- **自动同步**：角色新增/改名/删除时自动通知 Union 主站

**路由前缀约定：**

| 前缀 | 用途 | 认证 |
|------|------|------|
| `/api/union/member` | 服务器间 API（Union 主站 → 成员站） | UnionHostVerify 签名 |
| `/api/union/member` | 服务器间 API（公开元数据） | 无 |
| `/api/union/member/oauth2` | OAuth2 相关 | 混合 |
| `/union` | 用户端 API（登录用户） | Web Session + Auth |
| `/admin/union` | 管理端 API（管理员） | Web Session + Auth + Admin |
| `/api/union`（成员站 → Union 主站） | 成员站主动发起的请求 | X-Union-Member-Key |

---

## 2. 认证机制

Union 协议使用两套独立的认证机制，分别用于两个方向的请求。

### 2.1 UnionHostVerify：Union 主站 → 成员站的请求认证

当 Union 主站向成员站的 `/api/union/member/*` 端点发起请求时，成员站通过 `UnionHostVerify` 中间件验证请求的合法性。

**请求头：**

| 头部 | 说明 |
|------|------|
| `X-Message-Signature` | RSA-SHA256 签名的 Base64 编码 |
| `X-Message-Timestamp` | Unix 时间戳（秒） |
| `X-Message-Nonce` | 一次性随机字符串，防重放 |

**签名算法：**

```
待签名字符串 = request_body + timestamp + nonce
签名 = RSA-SHA256(待签名字符串, Union主站的私钥)
X-Message-Signature = Base64(签名)
```

其中 `request_body` 是 HTTP 请求体的原始字符串（对于 GET 请求为空字符串），`timestamp` 和 `nonce` 直接拼接在后面。

**验签流程：**

1. 从请求头中提取 `signature`、`timestamp`、`nonce`
2. **防重放检查**：检查 nonce 是否已在缓存中。如果已存在，说明是重放攻击，拒绝请求。nonce 缓存时间为 60 秒。
3. **时间戳检查**：`timestamp` 必须在 `[当前时间 - 10, 当前时间 + 30]` 秒范围内。
4. **获取公钥**：向 Union 主站的 API root 发送 `GET {union_api_root}` 请求，从响应 JSON 中提取 `union_host_signature_public_key` 字段。
5. **验签**：使用 `openssl_verify()` 验证 `signature` 是否匹配 `body + timestamp + nonce`。

```
public_key = GET(union_api_root).union_host_signature_public_key
result = openssl_verify(body + timestamp + nonce, base64_decode(signature), public_key, OPENSSL_ALGO_SHA256)
```

6. 验签通过后，将 nonce 存入缓存，有效期 60 秒。

### 2.2 Member Key：成员站 → Union 主站的反向认证

当成员站主动向 Union 主站发起请求时（例如同步角色、查询黑名单），通过 HTTP 头部携带成员密钥。

**请求头：**

| 头部 | 说明 |
|------|------|
| `X-Union-Member-Key` | 成员密钥字符串 |

**密钥分发方式：**

- **方式一（手动）**：管理员在 Union 主站的后台生成 member key，手动填入成员站的配置页面
- **方式二（自动下发）**：Union 主站通过 `POST /api/union/member/updatebackendkey` 推送 member key 到成员站，成员站自动保存

**密钥用途：**

- 成员站 → Union 主站所有 API 请求的认证（HTTP Header）
- OAuth2 流程中生成 HMAC 的共享密钥

---

## 3. 密钥体系

Union 协议共涉及四种密钥，各自有不同的生成方、持有方和用途。

### 3.1 union_member_key

| 属性 | 值 |
|------|------|
| **类型** | 对称密钥（任意字符串） |
| **生成方** | Union 主站 |
| **持有方** | Union 主站和成员站各持一份 |
| **用途** | OAuth2 Token 中的 HMAC-SHA256 计算；成员站 → Union 主站请求的 HTTP Header 认证 |
| **存储位置** | 成员站 `options` 表中的 `union_member_key` 字段 |
| **分发方式** | 手工配置或 `updatebackendkey` 推送 |

### 3.2 ygg_private_key

| 属性 | 值 |
|------|------|
| **类型** | RSA 4096 位私钥（PEM 格式） |
| **生成方** | Union 主站 |
| **持有方** | 成员站 |
| **用途** | Yggdrasil API 的 `signaturePublickey` 签名；Union 主站 → 成员站请求的签名私钥 |
| **存储位置** | 成员站 `options` 表中的 `ygg_private_key` 字段 |
| **分发方式** | 自动下发（首次启用插件时生成，或通过 `updateprivatekey` 推送更新） |

### 3.3 union_oauth2_sig_{private,public}_key

| 属性 | 值 |
|------|------|
| **类型** | RSA 4096 位密钥对（PEM 格式） |
| **生成方** | 成员站自身（首次启用时自动生成） |
| **持有方** | 成员站持有私钥和公钥；Union 主站通过 API 获取公钥 |
| **用途** | 私钥用于签名 OAuth2 的 userInfo Token；公钥暴露在 `/api/union/member/oauth2/` 供 Union 主站验证 |
| **存储位置** | 成员站 `options` 表中的 `union_oauth2_sig_private_key` 和 `union_oauth2_sig_public_key` |
| **生成时机** | 插件启用时，如果这对密钥为空则自动生成 |

### 3.4 union_host_signature_public_key

| 属性 | 值 |
|------|------|
| **类型** | RSA 公钥（PEM 格式） |
| **生成方** | Union 主站 |
| **持有方** | 成员站（动态拉取，不持久化存储） |
| **用途** | 验证 Union 主站发起的请求签名（UnionHostVerify 中间件使用） |
| **获取方式** | 成员站通过 `GET {union_api_root}` 的响应 JSON 中的 `union_host_signature_public_key` 字段动态获取 |

### 3.5 关于密钥生成的 BUG

`ConfigController@generate()` 方法中，`publicKey` 字段错误地返回了私钥值：

```php
// ConfigController.php 第 200-206 行
$keypair = ygg_generate_rsa_keys();
return json([
    'code' => 0,
    'privateKey' => $keypair['private'],
    'publicKey' => $keypair['private'],  // BUG：应该是 $keypair['public']
]);
```

实现者应注意此问题：该端点返回的两个字段实际上都是私钥。

---

## 4. 服务器间 API

服务器间 API 分为两大部分：Union 主站主动向成员站发起的请求（受 UnionHostVerify 中间件保护），以及公开的元数据端点。

### 4.1 GROUP A：Union 主站 → 成员站（需 UnionHostVerify）

所有端点前缀 `/api/union/member`，需携带 `X-Message-Signature`、`X-Message-Timestamp`、`X-Message-Nonce` 三个请求头。

---

#### `POST /api/union/member/updatelist`

Union 主站通知成员站更新服务器列表。

**认证：** UnionHostVerify

**流程：** 成员站收到请求后，向 Union 主站发起 `GET {union_api_root}/serverlist`（带 `X-Union-Member-Key` 请求头），拉取最新的服务器列表和版本号，保存到 `union_server_list` 和 `union_server_list_version` 选项中。

**请求体：** 无特定字段（UnionHostVerify 需要请求体以生成签名，可传空 JSON `{}`）

**响应：** 无特定响应体，返回 `204 No Content` 或 `200 OK`

---

#### `POST /api/union/member/updateprivatekey`

Union 主站通知成员站更新 Yggdrasil 私钥。

**认证：** UnionHostVerify

**流程：** 成员站收到请求后，向 Union 主站发起 `GET {union_api_root}/privatekey`（带 `X-Union-Member-Key`），从响应中获取 `privateKey` 和 `privateKeyVersion`，保存到 `ygg_private_key` 和 `union_private_key_version` 选项中。

**请求体：** 无特定字段

**响应：** 无特定响应体

---

#### `POST /api/union/member/updatebackendkey`

Union 主站向成员站推送 member key。这是 member key 自动下发的关键机制。

**认证：** UnionHostVerify

**请求体：**

```json
{
  "key": "new_member_key_string"
}
```

**处理逻辑：** 成员站直接从请求中提取 `key` 字段，保存到 `union_member_key` 选项：

```php
option(['union_member_key' => $request->input('key')]);
```

**响应：** 无特定响应体

---

#### `POST /api/union/member/sync`

Union 主站触发成员站的角色同步。

**认证：** UnionHostVerify

**请求体：**

```json
{
  "profileList": {
    "playerName1": "uuid1",
    "playerName2": "uuid2"
  }
}
```

**处理逻辑：** 成员站从本地数据库中查询所有玩家的 PID 和名称，以及 UUID 表中的 UUID 和名称，构建交集列表（取本地实际拥有的角色），然后向 Union 主站发起 `POST {union_api_root}/sync`（带 `X-Union-Member-Key`），将 `profileList` 转发。

```php
$names = Player::all()->pluck('pid', 'name');
$uuids = DB::table('uuid')->pluck('uuid', 'name');
$profiles = $uuids->intersectByKeys($names)->flip();
// 结果格式：{ "playerName": "uuid" }
```

**响应：** 无特定响应体

---

#### `POST /api/union/member/remapuuid`

Union 主站向成员站推送 UUID 重映射。

**认证：** UnionHostVerify

**请求体：**

```json
{
  "remapped_uuid": {
    "old_uuid_1": "new_uuid_1",
    "old_uuid_2": "new_uuid_2"
  }
}
```

**处理逻辑：** 成员站遍历 `remapped_uuid` 对象，对每一对 `old -> new` 映射，在本地 UUID 表中执行更新：

```sql
UPDATE uuid SET uuid = 'new_uuid' WHERE uuid = 'old_uuid'
```

**响应：** 无特定响应体

---

#### `POST /api/union/member/updateplugin`

Union 主站通知成员站更新插件。此端点是 Blessing Skin Server 的插件自动更新机制。

**认证：** UnionHostVerify

**请求体：**

```json
{ "url": "插件zip下载地址", "plugin": "插件id（可选，默认yggdrasil-connect）" }
```

**处理逻辑：** 成员站从给定 URL 下载 ZIP 包并解压到插件目录，然后禁用再启用插件（触发热加载）。

> **注意：** 如果成员站 `union_enable_update` 选项为 `false`，该请求将被忽略。这是 Blessing Skin Server 的 PHP 插件架构特有的自动更新机制。

---

#### `GET /api/union/member/queryemail?username=xxx`

Union 主站查询指定角色名对应的用户邮箱（用于黑名单验证）。如果指定的用户名不存在，返回 **204 No Content**。

**认证：** UnionHostVerify

**查询参数：**

| 参数 | 说明 |
|------|------|
| `username` | 角色名 |

**响应：**

```http
HTTP/1.1 204 No Content
```
（角色名不存在时）

```json
{
  "email": "user@example.com"
}
```
（角色名存在时）

---

#### `POST /api/union/member/diagnose`

Union 主站对成员站进行连通性诊断。

**认证：** UnionHostVerify

**请求体：** 包含 `nonce` 字段

**响应：**

```json
{
  "nonce": "请求中传入的 nonce",
  "timestamp": 1234567890.123
}
```

`timestamp` 为 `microtime(true)` 返回值（浮点数，含微秒）。

---

### 4.2 GROUP B：公开端点（无认证）

---

#### `GET /api/union/member/`

成员站的统一元数据端点（hello）。**无任何认证**，响应的 `Access-Control-Allow-Origin: *`。

**响应：**

```json
{
  "yggdrasilApiVersion": "1.2.3",
  "serverListVersion": 0,
  "privateKeyVersion": 0,
  "enabledFeatures": [
    "unionBlacklist",
    "emailVerification",
    "invitationCodesForUnion",
    "unionOAuth2"
  ]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `yggdrasilApiVersion` | string | 插件版本号（`plugin('yggdrasil-connect')->version`） |
| `serverListVersion` | int | 当前缓存的服务器列表版本号 |
| `privateKeyVersion` | int | 当前缓存的私钥版本号 |
| `enabledFeatures` | string[] | 启用的功能列表：`unionBlacklist`（始终启用）、`emailVerification`（如果站点开启了邮箱验证）、`invitationCodesForUnion`（`invitation_codes_for_union_enabled` 选项）、`unionOAuth2`（`union_enable_oauth2` 选项） |

---

## 5. OAuth2 联合登录

Union 的 OAuth2 联合登录允许用户从一个成员站（当前站点）安全地跳转到 Union 主站，反之亦然。

### 5.1 端点

#### `GET /api/union/member/oauth2/`

暴露成员站的 OAuth2 签名公钥。**无认证**，CORS 只允许 Union 主站域名。

**响应：**

```json
{
  "signaturePublicKey": "-----BEGIN PUBLIC KEY-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8A...\n-----BEGIN PUBLIC KEY-----"
}
```

#### `GET /api/union/member/oauth2/grant`

OAuth2 授权端点。需要 `web` + `auth` 中间件（用户必须已登录）。

**前提条件：**

- `union_enable_oauth2` 必须为 `true`
- 成员站的 OAuth2 签名密钥对必须有效（`openssl_pkey_get_private` 和 `openssl_pkey_get_public` 均能成功）
- Union 主站的 `/oauth2/backend` 端点必须可访问并返回有效的 `publicKey`

### 5.2 Token 构造（5 层结构）

OAuth2 授权流程的核心是构造一个多层加密的 Token，称为 `userInfoToken`。

**第 1 层 — UserInfo（Base64 JSON）：**

```json
// 原始 JSON
{
  "uid": 1,
  "nickname": "用户名",
  "email": "user@example.com",
  "expires_at": 1700000000
}
```

在 PHP 中：`$userInfo = base64_encode(json_encode($data))`，其中 `expires_at` 为 `time() + 600`（10 分钟有效期）。

**第 2 层 — HMAC：**

```
mac = HMAC-SHA256(userInfo, union_member_key)
```

使用 `hash_hmac('sha256', $userInfo, option('union_member_key'))` 计算。

**第 3 层 — RSA 签名：**

```
待签名字符串 = userInfo + "." + mac
signature = RSA-SHA256(待签名字符串, union_oauth2_sig_private_key)
```

使用 `openssl_sign()` 生成签名，然后 `base64_encode`。

**第 4 层 — 内部 Token（JSON）：**

```json
{
  "userInfo": "YmFzZTY0...",
  "mac": "hmac_hex_string",
  "signature": "YmFzZTY0X3NpZ25hdHVyZQ=="
}
```

**第 5 层 — RSA 加密：**

```
final = RSA_public_encrypt(JSON(inner_token), Union主站的OAuth2公钥)
```

使用 `RSAPublicUtil.public_encrypt()` 方法，从 `GET {union_api_root}/oauth2/backend` 获取 Union 主站的公钥。加密后返回 Base64 编码。

### 5.3 重定向

构造完成后，用户被重定向到：

```
{union_api_root}/oauth2/continue?userInfoToken={final}&{原有query参数保持}
```

原有 query 参数通过 `$request->all()` 获取并拼接到重定向 URL 中。

### 5.4 加密实现细节

`RSAPublicTrait::public_encrypt` 方法：

- 密钥位宽：4096 位 → 单次加密数据块大小为 `512 - 11 = 501` 字节（PKCS1 填充）
- 将数据分为 501 字节的块，逐块用 `openssl_public_encrypt` 加密
- 加密结果拼接后整体 `base64_encode`

---

## 6. 用户端 API

所有端点前缀 `/union`，需要 `web` + `auth` 中间件（用户必须已登录 Blessing Skin）。

### `GET /union`

角色绑定管理页面。此接口返回服务端渲染的 HTML 页面（Blade 模板），非 JSON API。

**流程：** 获取当前登录用户的所有角色，通过并发请求查询 Union 主站：
- `GET {union_api_root}/profile/unmapped/byname/{角色名}` — 查询该角色在其他站点的绑定情况
- `GET {union_api_root}/profile/detail/{UUID}` — 查询该角色在当前 Union 网络的详细信息

**返回：** `union` 视图（Blade 模板），包含 `profiles`（角色绑定数据）、`servers`（服务器列表）、`union_api_root`。

### `POST /union/bind`

请求绑定 token。

**请求体：**

```json
{
  "uuid": "角色UUID"
}
```

**流程：** 向 Union 主站发起 `POST {union_api_root}/profile/bind`（带 `X-Union-Member-Key`），携带 `uuid`。

**响应：**

```json
{
  "token": "绑定token"
}
```

### `POST /union/bindto`

使用 token 完成绑定。

**请求体：**

```json
{
  "uuid": "角色UUID",
  "token": "步骤 bind 获取的 token"
}
```

**流程：** 向 Union 主站发起 `POST {union_api_root}/profile/bindto`（带 `X-Union-Member-Key`），携带 `uuid` 和 `token`。

### `POST /union/unbind`

解除角色绑定。

**请求体：**

```json
{
  "uuid": "角色UUID"
}
```

**流程：** 向 Union 主站发起 `POST {union_api_root}/profile/unbind`（带 `X-Union-Member-Key`）。

### `POST /union/remapuuid`

请求 UUID 重映射。

**请求体：**

```json
{
  "me": "当前用户的UUID",
  "target": "目标UUID"
}
```

**流程：** 向 Union 主站发起 `POST {union_api_root}/profile/remapuuid`（带 `X-Union-Member-Key`）。

### `GET /union/security/level`

获取 Union 安全等级。实际执行两步验证：

1. `POST {union_api_root}/code` body: `{"token": union_member_key}` — 换取临时授权码 code
2. `GET {union_api_root}/backend/{code}/security/level` — 用临时授权码获取安全等级

如果任一步失败，返回 HTTP 500。

---

## 7. 管理端 API

所有端点前缀 `/admin/union`，需要 `web` + `auth` + `role:admin` 中间件。

### `POST /admin/union/member/updatelist`

手动触发服务器列表更新。

**流程：** 与 `POST /api/union/member/updatelist` 相同，但由管理员主动触发。

### `POST /admin/union/member/updateprivatekey`

手动触发私钥更新。

**流程：** 与 `POST /api/union/member/updateprivatekey` 相同，但由管理员主动触发。

### `POST /admin/union/member/sync`

手动触发角色同步。

**流程：** 与 `POST /api/union/member/sync` 相同，但由管理员主动触发。

### `POST /admin/union/member/diagnose`

手动触发连通性诊断（成员站主动调用 Union 主站）。

**流程：** 成员站向 Union 主站发起 `POST {union_api_root}/diagnose`（带 `X-Union-Member-Key`），返回诊断结果。

**响应：**

```json
// 成功时
{
  "status": "ok",
  "data": { "nonce": "...", "timestamp": ... }
}

// 失败时
{
  "status": "error",
  "data": { "status_code": 500, "headers": {...}, "body": "..." }
}
```

### `GET /admin/union/view/blacklist`

黑名单管理页面（Blade 视图）。

### `GET /admin/union/view/blacklist/list`

黑名单列表 API。

**流程：** 向 Union 主站发起 `GET {union_api_root}/blacklist/query`（带 `X-Union-Member-Key`），将响应原样返回。

### `POST /admin/union/blacklist/create`

创建黑名单条目。

**请求体：** 传递给 Union 主站的 `POST {union_api_root}/blacklist/restful`（带 `X-Union-Member-Key`）。

### `POST /admin/union/blacklist/invalidate/{id}`

撤销（失效）指定黑名单条目。

**流程：** 向 Union 主站发起 `PUT {union_api_root}/blacklist/invalidate/{id}`（带 `X-Union-Member-Key`）。

### `POST /admin/union/blacklist/delete/{id}`

删除指定黑名单条目。

**流程：** 向 Union 主站发起 `DELETE {union_api_root}/blacklist/restful/{id}`（带 `X-Union-Member-Key`）。

---

## 8. 配置管理

成员站通过 Blessing Skin 的选项表单配置 Union 连接参数。

### 8.1 配置字段

| 选项键 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `union_api_root` | string | `https://skin.mualliance.ltd/api/union` | Union 主站 API 根地址 |
| `union_member_key` | string | `''` | 成员认证令牌（明文），可由 Union 主站通过 `updatebackendkey` 自动下发 |
| `union_enable_update` | bool | `true` | 是否允许 Union 主站推送插件更新 |
| `union_enable_oauth2` | bool | `true` | 是否启用 OAuth2 联合登录功能 |
| `union_oauth2_sig_private_key` | textarea | 自动生成 | OAuth2 签名私钥（PEM 格式），成员站自持 |
| `union_oauth2_sig_public_key` | textarea | 自动生成 | OAuth2 签名公钥（PEM 格式），暴露在 `/api/union/member/oauth2/` |
| `ygg_private_key` | textarea（只读） | 自动生成 | Union 下发的 Yggdrasil 私钥（PEM 格式） |
| `union_private_key_version` | int | `0` | 私钥版本号 |
| `union_server_list_version` | int | `0` | 服务器列表版本号 |
| `union_server_list` | string（JSON） | `'{}'` | 缓存的服务器列表 |

### 8.2 密钥生成端点

**`POST /admin/plugins/config/yggdrasil-connect/generate`**

生成 RSA 4096 位密钥对。由管理配置页面的"生成密钥"按钮调用。

**响应：**

```json
{
  "code": 0,
  "privateKey": "-----BEGIN RSA PRIVATE KEY-----\n...",
  "publicKey": "-----BEGIN RSA PRIVATE KEY-----\n..."
}
```

> **注意**：`publicKey` 字段实际返回的是私钥（见第 3.5 节的 BUG 说明）。

### 8.3 自动初始化

插件启用时（`callbacks.php`），如果以下选项为空则自动初始化：

- `ygg_private_key` 为空 → 自动生成新的 RSA 4096 位密钥对
- `union_oauth2_sig_private_key` 或 `union_oauth2_sig_public_key` 为空 → 自动生成密钥对

---

## 9. 事件钩子

`bootstrap.php` 中注册了以下 Union 相关的事件监听。

### 9.1 角色生命周期同步

| 事件 | 触发时机 | 操作 |
|------|----------|------|
| `PlayerWasAdded` | 新增角色 | `POST {union_api_root}/profile` 带 `{ "id": uuid, "name": 角色名 }` |
| `PlayerProfileUpdated` | 修改角色（改名） | `PUT {union_api_root}/profile/{uuid}` 带 `{ "name": 新角色名 }` |
| `PlayerWillBeDeleted` | 删除角色 | `DELETE {union_api_root}/profile/{uuid}` |

以上三个事件的 HTTP 请求均携带 `X-Union-Member-Key: union_member_key` 请求头，超时时间为 5 秒。

UUID 由 `Profile::getUuidFromName($player->name)` 生成。

### 9.2 安全相关

| 事件 | 触发时机 | 操作 |
|------|----------|------|
| `user.profile.updated`（password 或 email） | 用户修改密码或邮箱 | 撤销该用户的所有 Access Token |

---

## 10. 路由表总览

### 10.1 服务器间 API（前缀 `/api/union/member`）

| 方法 | 路径 | 中间件 | 控制器方法 |
|------|------|--------|-----------|
| POST | `/api/union/member/updatelist` | UnionHostVerify | `UnionController@updateList` |
| POST | `/api/union/member/updateprivatekey` | UnionHostVerify | `UnionController@updatePrivateKey` |
| POST | `/api/union/member/updatebackendkey` | UnionHostVerify | `UnionController@serverUpdatesBackendKey` |
| POST | `/api/union/member/sync` | UnionHostVerify | `UnionController@triggerSync` |
| POST | `/api/union/member/remapuuid` | UnionHostVerify | `UnionController@remapUUID` |
| POST | `/api/union/member/updateplugin` | UnionHostVerify | `UpdateController@update` |
| GET | `/api/union/member/queryemail` | UnionHostVerify | `UnionBlacklistController@queryEmail` |
| POST | `/api/union/member/diagnose` | UnionHostVerify | `UnionController@diagnose` |
| GET | `/api/union/member/` | 无 | `UnionController@hello` |

### 10.2 OAuth2 端点（前缀 `/api/union/member/oauth2`）

| 方法 | 路径 | 中间件 | 控制器方法 |
|------|------|--------|-----------|
| GET | `/api/union/member/oauth2/` | 无（CORS 限 Union 域名） | `UnionOAuth2Controller@getSigPublicKey` |
| GET | `/api/union/member/oauth2/grant` | Constructor: auth + web | `UnionOAuth2Controller@grant` |

### 10.3 用户端 API（前缀 `/union`）

| 方法 | 路径 | 中间件 | 控制器方法 |
|------|------|--------|-----------|
| GET | `/union` | web + auth | `UnionProfileController@render` |
| POST | `/union/bind` | web + auth | `UnionProfileController@bind` |
| POST | `/union/bindto` | web + auth | `UnionProfileController@bindto` |
| POST | `/union/unbind` | web + auth | `UnionProfileController@unbind` |
| POST | `/union/remapuuid` | web + auth | `UnionProfileController@requestRemapUUID` |
| GET | `/union/security/level` | web + auth | `UnionController@getSecurityLevel` |

### 10.4 管理端 API（前缀 `/admin/union`）

| 方法 | 路径 | 中间件 | 控制器方法 |
|------|------|--------|-----------|
| POST | `/admin/union/member/updatelist` | web + auth + role:admin | `UnionController@updateList` |
| POST | `/admin/union/member/updateprivatekey` | web + auth + role:admin | `UnionController@updatePrivateKey` |
| POST | `/admin/union/member/sync` | web + auth + role:admin | `UnionController@triggerSync` |
| POST | `/admin/union/member/diagnose` | web + auth + role:admin | `UnionController@triggerDiagnose` |
| GET | `/admin/union/view/blacklist` | web + auth + role:admin | 视图: `blacklist` |
| GET | `/admin/union/view/blacklist/list` | web + auth + role:admin | `UnionBlacklistController@viewBlacklist` |
| POST | `/admin/union/blacklist/create` | web + auth + role:admin | `UnionBlacklistController@create` |
| POST | `/admin/union/blacklist/invalidate/{id}` | web + auth + role:admin | `UnionBlacklistController@invalidate` |
| POST | `/admin/union/blacklist/delete/{id}` | web + auth + role:admin | `UnionBlacklistController@delete` |

---

## 11. 部署模式

### 11.1 Blessing Skin 路由约定

标准 Blessing Skin 部署中，Union 插件使用以下路由前缀：

| 子系统 | 路由前缀 | 用途 |
|--------|----------|------|
| Yggdrasil API | `/api/yggdrasil` | Minecraft 外置登录（authlib-injector） |
| Union API | `/api/union` | Union 联合网络协议 |
| Yggdrasil Connect | `/yggc` | OpenID Connect 流程（Janus 配合） |

### 11.2 authlib-injector 配置

客户端启动器通过 authlib-injector 配置 Yggdrasil API 地址：

```
yggdrasil.url = https://domain.com/api/yggdrasil
```

### 11.3 Feature Flags

Union 部分功能通过 Blessing Skin 的选项系统控制：

| 标志 | 默认值 | 影响 |
|------|--------|------|
| `union_enable_update` | `true` | 控制是否处理 `updateplugin` 推送（插件自动更新） |
| `union_enable_oauth2` | `true` | 控制 OAuth2 联合登录功能和 `enabledFeatures` 中的 `unionOAuth2` |
| `invitation_codes_for_union_enabled` | `false` | 控制 `enabledFeatures` 中的 `invitationCodesForUnion` |

### 11.4 X-Union-Member-Key 的自动下发机制

Union 主站通过以下链路实现 member key 的自动下发：

1. Union 主站生成（或管理员设置）一个随机字符串作为 member key
2. Union 主站向成员站发起 `POST /api/union/member/updatebackendkey`（带 UnionHostVerify 签名），请求体包含 `{ "key": "..." }`
3. 成员站验证签名通过后，执行 `option(['union_member_key' => $request->input('key')])`
4. 此后成员站即可使用该 key 向 Union 主站发起认证请求

### 11.5 密钥版本管理

`union_server_list_version` 和 `union_private_key_version` 用于版本跟踪：

- Union 主站每次更新服务器列表时增加版本号
- 成员站通过 `hello` 端点暴露当前版本号
- Union 主站可以据此判断是否需要推送更新
- 版本号仅在同步时更新，不参与缓存验证逻辑

---

### 11.6 Yggdrasil Metadata 与 Union 的关联

在 `ConfigController@hello` 中，Yggdrasil 服务发现接口会将 Union 服务器列表中的域名自动注入到 `skinDomains` 字段中：

```php
$unionServers = array_column(json_decode(option('union_server_list'), true), 'bs_root');
foreach ($unionServers as &$server) {
    $server = parse_url($server, PHP_URL_HOST);
}
$skinDomains = array_merge($extra, $unionServers, [
    parse_url(option('site_url'), PHP_URL_HOST),
    $request->getHost(),
]);
```

这意味着：客户端的 authlib-injector 会自动信任所有 Union 成员站的材质域名，无需手动逐个添加。
