# Element Skin Python SDK 文档

本目录面向第三方 Python 应用开发者，说明如何使用 `element-skin-sdk` 接入
Element Skin 的 OAuth 和 `/v1` API。

## 阅读顺序

1. [快速开始](快速开始.md)
2. [OAuth 流程](OAuth流程.md)
3. [权限模型](权限模型.md)
4. [API 客户端](API客户端.md)
5. [错误与 Token](错误与Token.md)
6. [测试规范](测试规范.md)

## 包结构

```text
element_skin_sdk
├── OAuthClient             OAuth 2.1 流程封装
├── ElementSkinAPI          `/v1` API 客户端
├── permissions             权限常量和校验器
├── oauth                   PKCE 与 token 存储
├── models                  token、权限等数据模型
└── exceptions              SDK 异常层级
```

## 已支持的 OAuth 流程

| 流程 | SDK 方法 | 典型应用 |
| --- | --- | --- |
| Authorization Code + PKCE | `authorization_url`、`exchange_code` | Web 应用、桌面应用、可打开浏览器的 CLI |
| Device Code Flow | `start_device_flow`、`poll_device_token` | CLI、启动器、无法方便接收回调的设备 |
| Client Credentials | `client_credentials` | 经管理员审核的应用自身能力 |
| Refresh Token | `refresh` | 长期用户委托访问 |
| Revoke | `revoke` | 退出登录或取消连接 |
| Introspection | `introspect` | token 调试或管理侧校验 |

## 已封装的 API 分组

当前同步客户端覆盖常用用户 API 和 Minecraft 能力 API：

- `GET /v1/users/me`
- `GET/POST/PATCH/DELETE /v1/users/me/profiles`
- `GET/PATCH/DELETE /v1/users/me/textures/{hash}/{texture_type}`
- `POST /v1/users/me/textures/{hash}/wardrobe`
- `POST /v1/users/me/textures/{hash}/apply`
- `GET /v1/minecraft/profiles/by-name/{name}`
- `POST /v1/minecraft/profiles/by-names`
- `POST /v1/minecraft/session/has-joined`

后续增加更多 `/v1` wrapper 不需要改变 OAuth 行为。
