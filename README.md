<p align="center">
  <img src="./img/readme-header.svg" width="100%" alt="Element-Skin Header">
</p>

<p align="center">
  面向高并发场景的现代化外置登录与材质平台
</p>

<p align="center">
  <a href="https://deepwiki.com/water2004/element-skin">
    <img src="https://deepwiki.com/badge.svg">
  </a>
  <a href="LICENSE">
    <img src="https://img.shields.io/github/license/water2004/element-skin">
  </a>
  <img src="https://img.shields.io/badge/Vue-3-4FC08D?logo=vue.js&logoColor=white">
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white">
  <img src="https://img.shields.io/badge/PostgreSQL-4169E1?logo=postgresql&logoColor=white">
  <img src="https://img.shields.io/badge/Redis-required-DC382D?logo=redis&logoColor=white">
</p>

![](./img/root.png)

## ✨ 功能特性

- **✅ 极致性能**: 后端基于 Go 重构，使用 PostgreSQL + Redis 支撑高并发读写路径。
- **✅ 现代化数据库**: 使用 **PostgreSQL 18** 作为主存储，Go 后端通过高性能 PostgreSQL 驱动与连接池访问数据。
- **✅ 完整协议支持**: 完美实现 Yggdrasil API，无缝对接 Authlib-Injector 等主流加载器。
- **✅ 完整的Fallback机制**: 支持多个第三方服务作为数据源，允许其他其他皮肤站的用户进入服务器。
- **✅ 正版登录支持**: 集成 Mojang 官方认证服务，允许正版用户直接使用 Minecraft 账号登录。
- **✅ 皮肤管理**: 支持皮肤/披风上传，集成 SkinView3D 提供丝滑的 3D 实时预览。
- **✅ 完善的用户系统**: 包含邮箱验证、注册验证码、密码找回流程（支持 SMTP）。
- **✅ 强大的管理后台**: 响应式设计，支持用户管理、邀请码机制、首页媒体配置及邮件服务测试。
- **✅ 安全与防护**: 内置 API 速率限制 (Rate Limiting) 及多种安全防护机制。
- **✅ 灵活部署**: 既支持 Docker 一键部署，也支持复杂的子目录 (Sub-path) 架构。

---

## 🚀 Docker 部署指南 (推荐)

项目现在默认使用 **PostgreSQL 18 + Redis** 并支持自动化初始化。PostgreSQL 保存用户、设置、材质元数据等持久化数据；Redis 负责公开配置/首页媒体缓存、邮件验证码、限流数据和短期用户鉴权缓存等临时状态。

### 1. 准备 `.env`

Docker 部署只使用仓库根目录的 `docker-compose.yml`。复制 `.env.example` 为 `.env`，然后只修改 `.env`：

```bash
cp .env.example .env
```

必须重点修改这些值：

- `ELEMENT_SKIN_IMAGE`：后端镜像，默认 `ghcr.io/water2004/element-skin:latest`
- `JWT_SECRET`：生产环境随机长密钥
- `DATABASE_PASSWORD` / `REDIS_PASSWORD`：数据库和 Redis 密码
- `SERVER_SITE_URL`：站点外部访问地址
- `SERVER_API_URL`：后端 API 外部访问地址
- `CORS_ALLOW_ORIGINS`：允许访问 API 的前端来源

后端启动时会读取环境变量，并从 `DATABASE_HOST/PORT/USER/PASSWORD/NAME/SSLMODE` 和 `REDIS_HOST/PORT` 派生连接地址。Docker 部署不需要挂载 `config.yaml`，也不维护第二份 Compose 配置。

首次启动时如果 `/app/data/private.pem` 和 `/app/data/public.pem` 不存在，系统会自动生成并保存。请持久化 `./data` 目录，其中 `./data/db` 会挂载到 PostgreSQL 容器的 `/var/lib/postgresql`。后续不要删除或替换私钥，否则已有 Yggdrasil 客户端会看到服务端签名身份变化。

**Nginx 主机配置**
只需将 Nginx 的 `root` 指向宿主机的 `./frontend` 目录。

```nginx
server {
    listen 80;
    server_name yourdomain.com;

    # 1. 前端根目录 (index.html, assets, 以及皮肤 static/)
    root /your/path/to/frontend; 
    index index.html;

    location / {
        add_header X-Authlib-Injector-API-Location "http://yourdomain.com/skinapi" always;
        try_files $uri $uri/ /index.html;
    }

    # 2. 后端 API 转发
    location /skinapi/ {
        proxy_pass http://localhost:8000/;
        proxy_set_header Host $host;
    }
    
    # 直接转发不带斜杠的 API 请求
    location = /skinapi {
        return 308 /skinapi/;
    }
}
```
### 2. 启动服务

拉取镜像并启动：

```bash
docker compose pull
docker compose up -d
```

对于希望前端或后端地址部署在子目录的用户，可以通过 `.env` 灵活配置路径：
- **前端路径**: 通过 `VITE_BASE_PATH` 定义前端资源的基础路径
- **后端路径**: 通过 `VITE_API_BASE` 定义后端 API 的基础路径

根据你的路径需求修改 `.env`，然后重新执行 `docker compose up -d`。前端会根据这些参数在容器启动时替换路径，并自动释放到宿主机的 `./frontend` 目录：

| 场景 | `VITE_BASE_PATH` | `VITE_API_BASE` |
|-----|---------|---------|
| **场景 1** | `/skin/` | `/skinapi` |
| **场景 2** | `/skin/` | `/skin/api/` |

需要注意的是，`.env` 中的 `SERVER_SITE_URL` 和 `SERVER_API_URL` 也需要根据实际部署路径进行调整，以确保生成的链接正确。
当 `VITE_API_BASE` 使用 `/skinapi`、`/skin/api` 这类前缀时，Nginx 的 `proxy_pass` 末尾必须带 `/`，这样会把前缀剥掉再转发给后端。例如 `/skinapi/v1/users/me` 会转成后端实际路由 `/v1/users/me`。

**Nginx 主机配置 (对应场景 1)**
```nginx
# 1. 前端静态文件
location /skin/ {
    add_header X-Authlib-Injector-API-Location "http://yourdomain.com/skinapi" always;
    alias /your/path/to/frontend/;
    index index.html;
    try_files $uri $uri/ /skin/index.html;
}
location = /skin {
    alias /your/path/to/frontend/;
    try_files $uri $uri/ /skin/index.html;
}

# 2. 后端 API 转发
location /skinapi/ {
    proxy_pass http://localhost:8000/;
    proxy_set_header Host $host;
}
location = /skinapi {
    return 308 /skinapi/;
}
```

**Nginx 主机配置 (对应场景 2)**
```nginx
# 1. 前端静态文件
location /skin/ {
    add_header X-Authlib-Injector-API-Location "http://yourdomain.com/skin/api" always;
    alias /your/path/to/frontend/;
    index index.html;
    try_files $uri $uri/ /skin/index.html;
}
location = /skin {
    alias /your/path/to/frontend/;
    try_files $uri $uri/ /skin/index.html;
}

# 2. 后端 API 转发 (嵌套路径)
location /skin/api/ {
    proxy_pass http://localhost:8000/;
    proxy_set_header Host $host;
}
location = /skin/api {
    return 308 /skin/api/;
}
```
---

## 🛠️ 本地开发环境

### 本地开发环境

#### 1. 数据库配置 (PostgreSQL 18+)
本地开发需要手动安装并初始化数据库：

1.  **安装 PostgreSQL**: 确保本地已安装 PostgreSQL 18（或 16+）。
2.  **创建数据库**: 使用 `psql` 或 GUI 工具（如 pgAdmin/DBeaver）创建用户和数据库：
    ```sql
    -- 建议创建专用用户和库
    CREATE USER elementskin WITH PASSWORD 'password123';
    CREATE DATABASE elementskin OWNER elementskin;
    ```
3.  **修改配置**: 编辑 `skin-backend/config.yaml` 中的数据库字段：
    ```yaml
    database:
      host: "localhost"
      port: "5432"
      user: "elementskin"
      password: "password123"
      name: "elementskin"
      sslmode: "disable"
    ```
    > 💡 **自动初始化**: 后端在每次启动时会自动同步数据库结构（创建缺失的表及默认配置），无需手动执行 SQL 脚本。

#### 2. Redis 配置
本地开发需要 Redis 运行在 `127.0.0.1:6379`。如果你的 Redis 设置了密码，请同步修改 `skin-backend/config.yaml`：

```yaml
redis:
  host: "127.0.0.1"
  port: "6379"
  password: ""
  db: 0
  key_prefix: "elementskin:"
```

#### 3. 后端 (Go 1.26+)
```bash
cd skin-backend
go run ./cmd/element-skin
```

#### 4. 前端 (Node.js)
```bash
cd element-skin
npm install
npm run dev
```

---

## 📂 项目结构

```text
element-skin/
├── element-skin/       # 前端源码 (Vue 3 + Element Plus)
├── skin-backend/       # Go 后端源码
│   ├── cmd/            # 进程入口
│   ├── internal/       # HTTP、服务、数据库与测试模块
│   └── config.yaml     # 后端配置文件
├── .env.example        # Docker 部署环境变量模板
├── data/               # Docker 持久化数据 (自动生成)
├── frontend/           # Docker 释放的前端静态文件 (自动生成)
├── docker-compose.yml  
└── README.md
```

## 📋 功能状态

### 核心功能
- [x] 完整的yggdrasil协议支持
- [x] 用户注册与登录
- [x] 用户材质上传
- [x] 游戏角色管理
- [x] 邮箱验证码与密码找回
- [x] 邀请码注册机制
- [x] Mojang服务fallback机制
- [x] 用户封禁与解封
- [x] 公共皮肤库
- [x] 用户材质管理
  - [x] 允许用户删除自己上传到公共库的材质
  - [x] 允许用户配置已有的材质信息, 如模型类型等
  - [x] 公共皮肤库添加材质名称
  - [x] 公共皮肤库按名称搜索
  - [x] 公共皮肤库按上传时间排序
- [x] 多个fallback服务支持
- [x] 导入第三方皮肤站的角色和材质数据

### 安全与性能
- [x] PostgreSQL 数据库模块
- [x] JWT认证机制
- [x] API速率限制
- [x] Redis 缓存、限流、邮件验证码与短期鉴权缓存
- [x] 管理员设置细粒度API
- [x] 数据库性能优化
- [x] PostgreSQL 连接池
- [x] Redis缓存支持
- [ ] 材质存储优化（如使用云存储或CDN）

### 前端优化
- [x] 响应式设计
- [x] 深色模式支持
- [x] 页脚信息（如站点名称、版权信息等）
- [ ] 国际化 (i18n) 支持
- [ ] 移动端适配优化
- [x] 前端性能优化（如图片懒加载、代码分割等）

### 端点与集成
- [ ] 移动端 App 认证接口
- [ ] 第三方登录（GitHub、微博等）
- [ ] 批量材质导入工具

### 测试
- [x] Go 分层自动化测试框架
- [x] 数据库层 (Database Layer) 核心接口覆盖
- [x] 业务逻辑层 (Service Layer) 核心规则覆盖
- [x] API 接口层 (HTTP Integration Layer) 核心流程覆盖
- [x] 固定并发压测覆盖公开接口、用户中心、管理后台与 Yggdrasil 常用端点

---

## 🧪 自动化测试

Go 后端采用分层测试架构，确保从底层数据库到顶层 API 的稳定性。

### 测试架构
1.  **数据库层 (`internal/database`)**: 验证 SQL 逻辑、数据迁移及缓存一致性。
2.  **业务逻辑层 (`internal/service`)**: 验证核心业务规则（如注册权限、材质级联更新）。
3.  **HTTP 集成层 (`internal/integration`)**: 使用真实 PostgreSQL 和真实 Redis，模拟真实 HTTP 请求。

### 运行测试
测试会自动创建临时数据库和文件目录，不会影响本地开发数据。

```bash
cd skin-backend
go test ./...
```

### 编写新测试
单元测试使用内存 Redis mock；`internal/integration` 使用真实 Redis，并通过唯一 key 前缀自动清理测试数据，不会清空你的本地 Redis。

## 📈 并发压测结果

最新一次 v3.0.0 压测在本机通过 `skin-backend/cmd/loadtest` 启动隔离测试数据库、真实 Redis key 前缀和进程内 HTTP 服务完成，不会触碰正常运行数据库。命令如下：

```bash
cd skin-backend
LOADTEST_ENABLE=1 LOADTEST_CONCURRENCY=200 LOADTEST_DURATION=1s go test ./cmd/loadtest -run TestRealBackendLoad -count=1 -v
```

测试数据：100 个用户、300 个角色、500 条材质记录、50 个邀请码、1 个预置 Yggdrasil join 会话。固定并发：200；每个场景窗口：1s；数据库连接池：20。当前报告包含公开端点、Cookie、OAuth delegated、Client Credentials、管理员和 Yggdrasil 场景，全部 0 失败。

### v3.0.0 与 v2.4.1 功能对照

下表比较的是同一业务功能在两个版本中的实际接口，不是 Go 与 Python 实现语言对比。v2.4.1 使用旧站点路径，v3.0.0 使用 `/v1` 路径；Yggdrasil 协议路径保持不变。

| 功能（v2.4.1 → v3.0.0） | v3.0.0 req/s | v2.4.1 req/s | 变化 | v3.0.0 P95 | v2.4.1 P95 |
| --- | ---: | ---: | ---: | ---: | ---: |
| 公开设置（`/public/settings` → `/v1/public/settings`） | 26733.6 | 35839.7 | -25.4% | 8.4ms | 6.9ms |
| 首页媒体（`/public/homepage-media` → `/v1/public/homepage-media`） | 32634.7 | 34373.7 | -5.1% | 7.8ms | 11.4ms |
| 公开皮肤库（`/public/skin-library` → `/v1/public/skin-library`） | 18196.2 | 22222.8 | -18.1% | 16.1ms | 13.4ms |
| 登录（`/site-login` → `/v1/auth/login`） | 311.7 | 271.1 | +15.0% | 890.7ms | 1.17s |
| Yggdrasil 元数据（`/` → `/`） | 26109.0 | 33210.8 | -21.4% | 10.2ms | 10.9ms |
| Yggdrasil authenticate | 289.6 | 287.9 | +0.6% | 1.11s | 1.16s |
| Yggdrasil validate | 16188.3 | 17246.6 | -6.1% | 13.9ms | 23.9ms |
| Yggdrasil profile | 70172.7 | 76284.7 | -8.0% | 4.6ms | 4.5ms |
| Yggdrasil 按名称查询 | 75233.8 | 80444.3 | -6.5% | 4.2ms | 4.4ms |
| Yggdrasil hasJoined | 1976.9 | 2046.7 | -3.4% | 158.5ms | 147.9ms |
| 当前用户（`/me` → `/v1/users/me`） | 12896.7 | 20109.5 | -35.9% | 18.7ms | 12.3ms |
| 我的角色（`/me/profiles` → `/v1/users/me/profiles`） | 17094.2 | 20785.0 | -17.8% | 13.4ms | 15.1ms |
| 我的材质（`/me/textures` → `/v1/users/me/textures`） | 17070.5 | 20894.3 | -18.3% | 13.6ms | 11.8ms |
| 材质详情（`/me/textures/{hash}/skin` → `/v1/users/me/textures/{hash}/skin`） | 16641.1 | 21344.9 | -22.0% | 13.8ms | 11.4ms |
| 管理员用户列表（`/admin/users` → `/v1/admin/users`） | 1879.5 | 3797.7 | -50.5% | 124.8ms | 65.0ms |
| 管理员用户详情（`/admin/users/{id}` → `/v1/admin/users/{id}`） | 12154.6 | 19608.9 | -38.0% | 19.4ms | 12.6ms |
| 管理员角色列表（`/admin/profiles` → `/v1/admin/profiles`） | 14260.2 | 14156.7 | +0.7% | 17.0ms | 29.5ms |
| 管理员材质列表（`/admin/textures` → `/v1/admin/textures`） | 14997.6 | 19838.2 | -24.4% | 17.0ms | 15.0ms |
| 管理员邀请码（`/admin/invites` → `/v1/admin/invites`） | 14821.4 | 17875.0 | -17.1% | 16.0ms | 15.9ms |
| 管理员站点设置（`/admin/settings/site` → `/v1/admin/settings/site`） | 2607.6 | 2237.5 | +16.5% | 80.8ms | 129.2ms |

这组对比只能说明固定测试条件下的吞吐和延迟差异，不能把路径迁移本身当作性能原因。3.0.0 额外增加了细粒度权限、Redis 权限缓存和统一 Actor 处理，因此当前用户和管理员列表等复杂权限路径需要重点关注。

### v3.0.0 新增 OAuth 压测场景

v2.4.1 没有对应 OAuth 功能，因此以下场景不参与跨版本对比：

| 场景 | 接口 | 成功 req/s | P95 |
| --- | --- | ---: | ---: |
| OAuth delegated 当前用户 | `/v1/users/me` | 9588.1 | 28.1ms |
| OAuth delegated 角色列表 | `/v1/users/me/profiles` | 13213.5 | 18.9ms |
| OAuth delegated 材质列表 | `/v1/users/me/textures` | 11809.0 | 23.5ms |
| OAuth delegated 材质详情 | `/v1/users/me/textures/{hash}/skin` | 13124.8 | 19.1ms |
| OAuth delegated 管理员用户列表 | `/v1/admin/users` | 1712.3 | 136.3ms |
| OAuth delegated 管理员用户详情 | `/v1/admin/users/{id}` | 9374.1 | 29.7ms |
| OAuth delegated 管理员邀请码 | `/v1/admin/invites` | 11404.4 | 22.5ms |
| Client Credentials 管理员邀请码 | `/v1/admin/invites` | 6709.2 | 42.1ms |
| OAuth delegated 管理员设置 | `/v1/admin/settings/site` | 2525.0 | 83.0ms |

完整报告见 [`reports/concurrency-load-test.md`](reports/concurrency-load-test.md)。压测报告使用隔离 PostgreSQL 数据库和 Redis key 前缀，测试结束后自动清理测试数据。

## 📄 许可证

[MIT License](LICENSE)
