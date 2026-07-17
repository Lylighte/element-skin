# Element Skin v3.0.0

> **重大版本升级 / BREAKING CHANGES**
>
> v3.0.0 不是一次普通的小版本更新，而是从 v2.4.1 开始的后端、API、权限、认证、部署和前端管理体验的整体升级。
>
> 升级前请完整备份 PostgreSQL 数据库、站点签名密钥、材质文件、首页媒体文件和生产配置。v3.0.0 的数据库迁移只支持 **v2.4.1 -> v3.0.0**，不支持从开发期间的中间版本直接迁移。

## 版本范围

本版本以 `main` 分支的 v2.4.1 为迁移起点，以当前 Go 版本为目标版本。开发期间曾经存在过多次权限、OAuth、通知、配置和数据库结构调整；这些中间状态不属于公开兼容面，也不会继续保留兼容代码或中间迁移路径。

本版本的变更可以分为以下几类：

- 运行时从 Python 后端迁移到 Go 后端。
- 站点 API 统一迁移到 `/v1`，Yggdrasil 协议路径保持不变。
- 从粗粒度管理员判断迁移到统一的细粒度权限主体模型。
- 增加 OAuth 第三方应用、用户授权、设备码和 Client Credentials 能力。
- 增加统一通知中心、公告、定向投递和系统通知。
- 重新组织数据库、Redis、调度任务、服务层和测试层。
- 重做 Docker Compose、环境变量配置、前端管理页面和 Python SDK。

## 重大变化

### 1. 后端运行时重写

- 后端从 Python 重写为 Go 1.26。
- HTTP 使用当前 Go 标准库路由和请求处理流程，数据库使用 PostgreSQL 驱动与连接池，Redis 使用统一的 store 接口。
- 后端代码按以下职责分层：
  - `httpapi`：解析请求、认证包装、基础输入校验、状态码和响应。
  - `service`：业务规则、权限判断、跨数据库与 Redis 的流程编排。
  - `database`：SQL、事务和持久化映射。
  - `redisstore`：短期状态、缓存、限流和 token 存储。
- 路由不再直接编排数据库写入；账号、权限、通知、OAuth、材质、首页媒体、导入和 fallback 等流程均通过对应业务 service 处理。
- 站点服务不再使用一个混合职责的“大 service”承载所有账号、资源和设置逻辑，相关模块已经按领域拆分。

### 2. 站点 API 统一迁移到 `/v1`

所有站点业务 API 的正式前缀现在是：

~~~text
/v1
~~~

以下旧站点路径不再作为长期兼容接口保留：

~~~text
/me
/public
/notices
/microsoft
/remote-ygg
/admin
/site-login
/site-logout
/register
/send-verification-code
/reset-password
/textures/upload
~~~

主要的新 API 分组如下：

| 分组 | 前缀 | 内容 |
| --- | --- | --- |
| 认证 | `/v1/auth/*` | 登录、登出、注册、验证码、密码重置、站点会话刷新 |
| 当前账号 | `/v1/users/me` | 当前用户信息、密码、邮箱、注销 |
| 当前角色 | `/v1/users/me/profiles/*` | 角色创建、修改、删除、清除材质 |
| 当前材质 | `/v1/users/me/textures/*` | 材质上传、修改、删除、衣柜、应用材质 |
| 公开站点 | `/v1/public/*` | 设置、首页媒体、fallback 状态、公开皮肤库 |
| 通知 | `/v1/notifications/*` | 通知列表、详情、已读、忽略 |
| Microsoft 导入 | `/v1/imports/microsoft/*` | 授权、角色读取、角色导入 |
| 远程 Yggdrasil 导入 | `/v1/imports/remote-ygg/*` | 角色预览、单个导入、批量导入 |
| 管理员用户 | `/v1/admin/users/*` | 用户、角色、权限、封禁、密码和受保护主体 |
| 管理员资源 | `/v1/admin/profiles/*`、`/v1/admin/textures/*` | 管理角色和材质 |
| 管理员配置 | `/v1/admin/settings/*`、`/v1/admin/homepage-media/*` | 站点设置、首页图片和全景媒体 |
| 管理员通知 | `/v1/admin/notifications/*` | 通知和公告发布、替换、删除 |
| 管理员邀请码 | `/v1/admin/invites/*` | 邀请码查看、创建、删除 |
| OAuth 应用 | `/v1/oauth/apps/*` | 开发者应用、权限、审核提交和密钥轮换 |
| OAuth 授权 | `/v1/oauth/grants/*` | 当前用户已授权应用的查看和撤销 |
| Minecraft 能力 | `/v1/minecraft/*` | 公开角色查询、材质属性、服务端加入结果校验 |

完整的请求字段、响应结构、状态码和权限要求见 [Element-Skin v1 API 设计规范](doc/Element-Skin-v1-API设计规范.md)。该文档是第三方开发者使用 `/v1` API 的正式参考，不应继续依据旧路由猜测接口。

### 3. Yggdrasil 协议路径保持不变

以下端点不属于站点 API breaking change，继续保留原路径，供 Authlib-Injector、启动器、Minecraft 客户端和服务器使用：

~~~text
GET    /
GET    /api/publickeys/
POST   /authserver/authenticate
POST   /authserver/refresh
POST   /authserver/validate
POST   /authserver/invalidate
POST   /authserver/signout
POST   /sessionserver/session/minecraft/join
GET    /sessionserver/session/minecraft/hasJoined
GET    /sessionserver/session/minecraft/profile/{uuid}
GET    /api/users/profiles/minecraft/{playerName}
GET    /users/profiles/minecraft/{playerName}
GET    /api/profiles/minecraft/{playerName}
POST   /api/profiles/minecraft
PUT    /api/user/profile/{uuid}/{texture_type}
DELETE /api/user/profile/{uuid}/{texture_type}
~~~

Yggdrasil token、服务器 join、会话验证、材质签名和 profile 查询仍然是独立协议链路，不与 `/v1` API 的 OAuth token 或站点 Cookie 混用。

## 细粒度权限模型

### 1. 统一 Actor 与权限主体

所有请求都会先解析为 Actor，再由 service 根据权限主体计算最终权限。凭证来源不决定业务权限：

- Cookie 会话解析为 Web Session Actor。
- OAuth Bearer token 解析为 OAuth Actor。
- Yggdrasil token 解析为 Yggdrasil Actor。
- 无凭证访问公开端点时使用 Guest Actor。

公开端点也经过权限模型。Guest 只是一个权限主体，不是绕过权限检查的特殊入口；携带无效 Cookie 或 Bearer token 时不会降级成游客，而是返回 `401`。

### 2. 权限表达能力

- 一个主体可以拥有多个角色。
- 角色提供一组预设权限。
- 主体可以追加或撤销单项权限覆盖。
- 权限范围支持 `self`、`own`、`bound_profile`、`any` 等资源关系。
- 权限标识由结构化权限目录定义，权限位索引仅在运行时生成，不作为数据库业务字段持久化。
- 角色、权限、主体、资源和作用域使用结构化数据库表保存，不把权限配置序列化为 JSON 字符串塞入数据库。
- 前端只用权限做页面和操作入口展示，最终拒绝始终由后端 service 执行。

### 3. 受保护主体

受保护主体不再等同于 `super_admin` 角色，也不绕过细粒度权限模型：

- 受保护状态是权限主体的独立属性。
- 同一时间最多有一个受保护主体。
- 受保护主体仍然必须通过正常权限检查。
- 具有管理受保护主体权限的主体可以将受保护状态转让给其他用户。
- 转让后原主体失去受保护状态，新主体获得受保护状态。
- 前端可以据此区分管理页面展示，但后端不使用粗粒度的超级管理员分支替代权限判断。

### 4. 账号封禁语义

账号封禁的当前语义是禁止该账号加入 Minecraft 服务器，而不是无差别删除账号的所有站点能力：

- 封禁影响 Yggdrasil join/server 相关能力。
- 其他能力仍由细粒度权限和具体资源状态决定。
- 封禁和解封通过账号 service 完成，并清理关联认证状态。
- 封禁必须填写原因，系统会向被封禁用户发送 30 天有效的通知。

## OAuth 与第三方应用

### 1. 支持的 OAuth 流程

当前实现提供以下 OAuth 2.1 方向的后端能力：

| 流程 | 用途 | 是否需要回调 |
| --- | --- | --- |
| Authorization Code + PKCE | Web、桌面应用和可打开浏览器的 CLI | 是 |
| Device Authorization Grant | CLI、启动器、电视和无回调设备 | 否 |
| Client Credentials Grant | 代表应用自身调用已审核的服务能力 | 否 |
| Refresh Token | 延续用户授权会话 | 否 |
| Revoke | 撤销 token 或用户授权 | 否 |
| Introspection | 查询 token 当前状态 | 否 |

OAuth 标准端点保持在 `/oauth/*` 和 `/.well-known/*`，不放入 `/v1`：

~~~text
GET  /.well-known/oauth-authorization-server
GET  /.well-known/oauth-protected-resource
GET  /oauth/authorize
POST /oauth/authorize
POST /oauth/device/code
GET  /oauth/device
POST /oauth/device
POST /oauth/token
POST /oauth/revoke
POST /oauth/introspect
~~~

### 2. 应用生命周期

应用开发者可以在“第三方应用”页面：

- 创建公开应用或机密应用。
- 设置应用名称、网站地址、回调地址和说明。
- 查看应用状态和管理员审核结果。
- 选择应用申请的权限。
- 提交审核、查看驳回原因并重新提交。
- 轮换机密应用的 client secret。
- 删除自己创建的应用。

管理员可以在独立的第三方应用管理页面：

- 分页查看全站应用，而不是一次加载全部详情。
- 查看应用所有者、客户端类型、网站、回调地址、申请权限、审核历史和当前状态。
- 通过、驳回或停用应用。
- 驳回和停用必须填写原因，原因会发送给应用开发者。
- 管理员页面的应用列表和应用详情分离加载，避免列表页因详情数据过多而卡顿。

### 3. 用户授权

用户授权页会展示应用身份和逐项申请权限。用户只能授权自己当前有权授予的用户能力；没有满足授权条件时，页面会明确提示无权授权该应用。

用户可以在“第三方应用”页面管理已授权应用：

- 查看应用名称、授权范围、授权时间和状态。
- 撤销单个应用的授权。
- 撤销后，相关 grant、refresh token 和短期 access token 立即失效。
- 撤销记录不会永久作为有效授权展示；过期或撤销的授权由系统清理任务处理，页面会显示清理状态和自动清理时间。

### 4. Client Credentials

Client Credentials 不代表某个用户，而是使用独立的 `client:{client_id}` 权限主体：

- 应用必须通过管理员审核后才能使用。
- 应用只可以调用管理员批准的应用权限。
- Server/Minecraft 能力不会通过普通用户授权流授予。
- 应用权限集合与用户授权 grant 分开，不能借用某个用户的权限。
- access token 存放在 Redis 等短期 token store 中；客户端、审核状态和长期授权状态存放在 PostgreSQL。

### 5. 授权与凭证失效

以下事件会触发明确的 OAuth 依赖处理：

- 用户撤销应用授权。
- 管理员驳回或停用应用。
- 应用创建者权限变化导致应用不再满足应用条件。
- 用户授权者权限变化导致 grant 不再满足授权条件。
- 应用密钥轮换导致旧凭证失效。
- 用户注销导致其授权、创建的应用和相关长期状态清理。
- refresh token 过期或失效后，相关授权进入待清理状态。

没有明确事件的自然过期由统一后台清理任务处理，包括过期 refresh token、失效授权和超过保留期的撤销记录。

## 通知与公告系统

### 1. 统一通知模型

公告不是独立的临时功能，而是统一通知模型中的 `announcement` 类型。通知模型已经为系统通知、OAuth 事件和未来通知类型保留扩展能力：

- `notices`：通知主体和展示内容。
- `notice_targets`：定向投递目标。
- `notice_receipts`：用户已读、忽略等状态。

投递目标与阅读状态分离，不会把用户状态混入公告正文或 audience 字段。

### 2. 公告

- 短公告只需要标题和短内容，不要求 Markdown 正文。
- 长公告需要标题、摘要和 Markdown 正文。
- 管理员可以设置级别、置顶、启用、可忽略、可见人群、生效时间和过期时间。
- 过期时间可以为空，表示无期限。
- 修改公告采用“删除原公告并发布新公告”的语义，不迁移旧公告的阅读、忽略和历史状态。
- 删除是真删除，避免公告历史无限积累。

### 3. 用户通知

- 通知中心支持通知列表、详情、已读和忽略。
- 左侧列表按通知条目滚动，右侧展示当前通知全文，窄屏自动切换为单列。
- 顶栏通知中心图标显示未读小红点。
- 未读统计不仅包含公告，也包含系统通知和未来新增通知类型。
- 定向通知只对目标用户可见，其他用户按资源不存在处理，避免泄露通知存在性。

### 4. 系统通知

系统维护 actor 会发送以下通知：

- OAuth 应用提交审核：通知管理员。
- 应用通过、驳回或停用：通知应用开发者。
- 驳回和停用原因：写入通知正文。
- 用户角色、权限覆盖和受保护主体发生变化：通知对应用户。
- 权限变化导致 OAuth 授权失效：通知授权用户。
- 权限变化导致应用停用：通知应用创建者。
- 管理员封禁账号：通知被封禁用户并附带封禁原因和截止时间。

系统通知默认有效 30 天，由统一调度任务清理。

## Yggdrasil、fallback 与 Minecraft API

### 1. fallback 健康检查

- 保留原有 fallback Services 健康检查的行为和展示方式。
- 健康检查继续记录最近 24 小时状态，仪表盘显示服务在线情况和探测结果。
- fallback 检查通过独立 service 处理，不将协议发现逻辑塞进 HTTP handler。
- fallback 请求经过 URL 校验、超时、连接生命周期和出站请求防护。

### 2. 公钥发现

新增并完善 Yggdrasil 公钥聚合：

- `GET /api/publickeys/` 返回 `playerCertificateKeys` 和 `profilePropertyKeys`。
- 站点公钥始终排在第一位。
- fallback 公钥通过 Yggdrasil 发现协议获取；只有发现失败时才尝试 fallback 的 Services URL `/publickeys/`。
- 发现入口成功后不重复请求 Services URL。
- 三个 fallback 路由无法形成有效发现根、请求解析失败或没有有效公钥时，判定本次发现失败。
- fallback 公钥按来源写入 Redis，并设置过期时间。
- 同一轮检查对同一来源只请求一次。
- 对外聚合结果按规范化公钥内容去重。
- fallback 删除、配置变更、缓存过期和发现失败都会执行对应缓存失效处理。
- 根元数据继续提供单数 `signaturePublickey`，同时提供包含本站和 fallback 公钥的复数 `signaturePublickeys`。

### 3. Minecraft 能力 API

`/v1/minecraft/*` 为第三方应用提供站点能力的正式 API，不替代 Yggdrasil 协议：

- 按名称查询公开角色。
- 批量按名称查询公开角色。
- 按角色 ID 查询公开角色。
- 获取角色的 `textures` property。
- 请求 Minecraft 服务端 join 结果校验。

Yggdrasil 的 authenticate、join、hasJoined、profile 和材质写入端点仍保持协议语义，并继续使用对应的 Yggdrasil 权限。

### 4. 导入链路

- Microsoft 导入是“导入正版角色”，不是将 Microsoft 账号绑定为站点登录身份。
- 远程 Yggdrasil 导入提供预览、单个导入和批量导入。
- 导入角色时会下载并保存材质文件，不再只保存角色元数据或远程 URL。
- 导入 service 负责远程请求、角色数据转换、材质下载和站点资源落库；路由层不直接处理这些业务流程。

## 前端与用户界面

### 1. 站点布局

- 仪表盘正式名称为“仪表盘”，不再使用“我的首页”。
- 仪表盘采用左侧主内容、右侧公告列的桌面布局，窄屏切换为单列。
- 资源统计、快速接入启动器和服务状态使用统一 UI 卡片与 Element Plus 封装组件。
- 快速接入启动器保持三行居中结构：说明文字、受长度限制的地址输入框、复制/拖拽按钮。
- 服务状态支持自动刷新倒计时和手动刷新。
- 顶栏导航根据可用空间逐项隐藏，而不是超过单一阈值后全部隐藏。
- footer 使用 fixed 布局。

### 2. 首页

- 首页标题区域根据顶栏下边缘与 fixed footer 上边缘计算可用垂直空间。
- 未登录和已登录首页使用统一布局计算，避免先显示错误登录态再跳变。
- 中央玻璃按钮保持 fixed 顶层结构，避免被普通布局容器破坏 backdrop blur。
- 首页背景支持图片、Minecraft 全景图、遮罩透明度和全景旋转速度配置。
- 全景图面序、预览裁切和媒体上传校验已统一处理。
- 首页渲染限制更新频率并减少无效渲染，降低高分辨率背景的资源消耗。

### 3. 用户、权限和 OAuth 页面

- 用户管理页支持分页、用户详情、角色、继承权限、权限覆盖、账号操作和受保护主体转移。
- 权限标签按类别选择和折叠展示，减少大量权限同时出现时的空间浪费。
- 前端所有页面入口和导航项按细粒度权限展示，不再只判断管理员身份。
- 第三方应用页面统一为开发者应用管理和用户授权管理两个职责区，不再使用与管理员用户权限编辑器混淆的控件。
- 应用列表和应用详情分离加载，管理员查看全站应用时不会一次拉取全部详情。
- Authorization Code 和 Device Code 共用同一套 OAuth 授权确认组件。
- 邮箱变更改为个人资料中的弹窗流程：输入新邮箱、向新邮箱发送验证码并完成确认。
- 注册页只在站点配置要求邀请码时显示邀请码输入项。

### 4. 浏览器缓存

- 新增统一浏览器存储抽象，集中处理 localStorage 和 IndexedDB。
- 材质文件、角色卡片和材质卡片静态渲染结果可以进入 IndexedDB 缓存。
- 渲染缓存使用 LRU 策略和总大小限制。
- localStorage 清理逻辑能够枚举并删除未使用的站点键，避免历史键堆积导致存储异常。
- 页面和组件不再直接散落访问浏览器存储对象。

## 数据库与 Redis

### 1. v2.4.1 迁移范围

v3.0.0 启动时只执行从 v2.4.1 到当前结构所需的迁移：

- 将皮肤库主键调整为 `(skin_hash, texture_type)`，允许同一 hash 分别保存不同材质类型。
- 为皮肤库补充并回填 `usage_count`。
- 为用户补充并回填 `created_at`。
- 将旧 fallback 皮肤域名数据迁移到结构化的 `fallback_skin_domains` 表。
- 删除不再使用的旧 `tokens` 和 `sessions` 表。
- 根据 v2.4.1 的管理员数据初始化当前细粒度权限主体、角色和权限状态。
- 迁移逻辑不包含开发阶段曾经存在过的临时字段、旧配置清理或中间版本兼容分支。

### 2. 当前持久化模型

当前数据库按领域保存结构化数据，主要包括：

- 用户、角色、站点 refresh token、邀请码、设置、验证记录。
- 角色 profile、用户材质、皮肤库、fallback endpoint、fallback 域名、官方白名单。
- 首页媒体、启用的彩蛋、通知、通知目标、通知回执。
- 权限主体、资源、动作、作用域、权限目录、角色权限、主体角色、权限覆盖、会话权限策略。
- OAuth delegated client、客户端权限、用户授权 grant、授权码、refresh token、设备码以及相关权限记录。
- 权限审计日志。

可查询、可筛选、可更新和可约束的业务数据都使用列或关联表保存，不使用 JSON 字符串替代关系模型。

### 3. Redis 生命周期

Redis 是生产运行时必需依赖，主要用于：

- OAuth access token、Yggdrasil token、短期会话和设备码。
- 认证缓存、权限缓存、公开站点配置和首页媒体缓存。
- fallback 健康状态、公钥缓存、邮件验证码、限流计数和调度任务运行状态。
- 权限变化、授权撤销、应用状态变化和配置变更时的缓存失效。

access token 等短期凭证不作为长期 PostgreSQL 记录保存；refresh token、客户端、grant、审核状态和权限配置等长期状态保存在 PostgreSQL。

## 配置与部署

### 1. 单一 Docker Compose 部署

仓库根目录的 `docker-compose.yml` 是正式部署方式：

- `db\)：PostgreSQL 18 Alpine。
- `redis\)：Redis 8 Alpine。
- `backend\)：包含后端和前端静态资源的单镜像。
- PostgreSQL 数据目录挂载到 `./data/db:/var/lib/postgresql`。
- Redis 数据目录挂载到 `./data/redis:/data`。
- 前端静态资源释放到 `./frontend`。
- 配置通过根目录 `.env` 注入，不需要在 Compose 中重复填写完整 DSN。

### 2. 环境变量覆盖

后端可以读取配置文件，也可以在没有配置文件时由完整环境变量启动。环境变量优先级高于配置文件；生产环境中的数据库、Redis、JWT、密钥、站点 URL、API URL 和 CORS 配置必须显式提供。

关键配置分组包括：

- `JWT_*`
- `KEYS_*`
- `DATABASE_*`
- `REDIS_*`
- `TEXTURES_DIRECTORY`、`CAROUSEL_DIRECTORY`
- `SERVER_*`
- `CORS_*`
- `VITE_BASE_PATH`、`VITE_API_BASE`

未提供完整生产配置时后端会明确启动失败，不再用危险默认值静默运行。

### 3. CORS 与安全配置

- CORS 来源从配置读取。
- 启用凭据时拒绝 `*` 来源。
- OAuth 后端应用使用服务器到服务器请求时通常不依赖浏览器 CORS；浏览器跨源调用 `/v1` API 时必须使用站点允许的来源。
- 站点 URL、API URL、OAuth 回调地址、上传 URL 和出站 fallback URL 会进行合法性和安全校验。

## 安全性与可靠性改进

- 认证 Cookie 保留安全属性，生产部署可以使用 Secure、HttpOnly 和 SameSite 策略。
- OAuth access token、refresh token、授权码、设备码和 Yggdrasil token 使用独立链路，避免相互混用。
- refresh token 轮换和失败回滚不会错误丢失仍然有效的 token。
- 授权、撤销、应用停用、密钥轮换和权限变化会同步处理缓存与关联依赖。
- 用户删除、账号清理、材质更新、文件写入和数据库写入使用事务或补偿逻辑，避免只写入一半。
- 失败的注册、验证码、设置、首页媒体、材质和 token 操作不会留下半成品数据。
- 出站 HTTP 请求增加 URL 校验、响应大小限制、超时、连接生命周期限制和安全防护。
- 结构化 JSON 请求体、multipart 文件、首页图片和材质 hash 均有大小、格式和引用校验。
- 防止 Vite 静态资源路径穿越。
- 限流地址解析和登录限流逻辑避免被伪造请求头绕过。
- 升级 Go 依赖和 Axios 安全基线。

## Python SDK

新增 `python-sdk/`，面向第三方 Python 开发者提供：

- Authorization Code + PKCE。
- Device Code Flow。
- Client Credentials。
- Refresh、Revoke 和 Introspection。
- `/v1` API 客户端。
- 权限目录、权限 scope 和本地校验器。
- PKCE 工具、token 存储和明确的异常类型。
- 代表管理员的 Client Credentials 邀请码 demo。
- 代表用户授权的设备码和回调 demo。

SDK 文档统一放在 `python-sdk/doc/`，包括快速开始、OAuth 流程、权限模型、API 客户端、错误与 token、测试规范。demo 不包含真实 token 或 client secret，敏感配置和本地 token 文件由 `.gitignore` 忽略。

## 测试、覆盖率与性能

### 1. 测试组织

测试按源码职责组织，而不是将所有 case 堆进单个文件：

- 数据库 store 测试覆盖 SQL、事务、分页、迁移和错误路径。
- service 测试覆盖权限、业务状态转换、缓存失效、通知投递和 OAuth 依赖处理。
- HTTP API 测试覆盖精确状态码、错误体、请求参数、响应字段和认证路径。
- integration 测试使用真实 PostgreSQL 和 Redis 验证跨层行为。
- Yggdrasil、OAuth、权限、导入、通知、配置、调度器和缓存测试按职责拆分。
- 前端 API wrapper、存储、缓存、权限页面、导入和关键 composable 使用独立 fixture 与精确断言。

### 2. 覆盖范围

- 后端测试目标保持 90% 以上覆盖率，重点覆盖错误路径、权限拒绝、事务回滚、token 轮换、并发、缓存一致性和过期清理。
- Python SDK 以 100% 行覆盖和 100% 分支覆盖为目标，并覆盖 OAuth、权限、HTTP、token 存储和异常分支。
- 测试不以“没有报错”“返回非空”或“长度大于 0”作为替代断言，而是验证具体状态码、字段、数据库行、Redis key 和副作用。

### 3. 压测

仓库保留公开接口、登录、用户中心、管理后台、Yggdrasil、OAuth Cookie、OAuth Bearer 和混合并发压测报告。权限缓存和 Redis 优化后，权限计算不再对每个请求重复执行完整数据库查询；会话权限策略和有效权限集合尽量在初始化或缓存命中时预计算。

报告位于：

- `reports/concurrency-load-test.md`
- `reports/concurrency-load-test-permission-current.md`
- `reports/oauth-mixed-load-test.md`
- `reports/permission-loadtest-impact.md`

## 升级步骤

1. 将现有站点先升级或确认在 v2.4.1。
2. 停止旧服务，完整备份 PostgreSQL 数据库。
3. 备份 `.env`、配置文件、`data` 目录、材质目录、首页媒体目录和 `private.pem` / `public.pem`。
4. 准备 Redis，并确认 Redis 密码、持久化目录和连接配置正确。
5. 复制 `.env.example` 为 `.env`，填写生产所需环境变量。不要把生产 secret 写入仓库或 release note。
6. 使用根目录 Compose 启动 v3.0.0：

~~~bash
docker compose pull
docker compose up -d
~~~

7. 查看后端启动日志，确认：
   - PostgreSQL 连接成功。
   - v2.4.1 数据库迁移成功。
   - Redis 连接成功。
   - 站点签名密钥已加载。
   - CORS、站点 URL 和 API URL 校验成功。
8. 登录后检查管理员角色、细粒度权限、受保护主体和用户封禁状态。
9. 检查用户角色、材质、皮肤库、邀请码、fallback、首页媒体和通知。
10. 检查 OAuth 应用、应用审核状态、用户授权和 token 撤销流程。
11. 使用 Authlib-Injector 验证根发现、Yggdrasil 登录、join、hasJoined、profile 和材质下载。
12. 检查 Nginx 的前端静态目录、API 反向代理和 `X-Authlib-Injector-API-Location` 配置。

### 升级后的注意事项

- 前端调用站点能力必须改用 `/v1` 路径，旧站点路径不会作为长期兼容接口继续维护。
- Yggdrasil 调用方不需要把协议请求迁移到 `/v1`。
- 旧站点 Cookie 和旧 Python 后端 token 不应被当作 v3.0.0 的 OAuth token 使用。
- 旧 `tokens` 和 `sessions` 表会被删除；短期认证状态由 Redis 管理。
- 不要删除或替换站点签名密钥，否则客户端会认为服务端身份发生变化。
- 应用开发者需要按照新的 OAuth 应用审核和权限申请流程重新检查应用配置。
- 第三方应用应使用 OAuth 标准端点和 `/v1` API，不应依赖管理员页面内部接口。

## 回滚说明

- v3.0.0 的数据库迁移不是可逆迁移。
- 需要回滚时，停止 v3.0.0 服务，从升级前的 PostgreSQL 备份、配置和数据目录恢复，再部署 v2.4.1。
- 不要让 v2.4.1 直接连接已经完成 v3.0.0 迁移的数据库。
- 不要只回滚镜像而不恢复数据库；权限主体、OAuth、通知和结构化数据已经发生变化。
