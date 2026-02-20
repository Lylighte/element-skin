# Element-Skin — Minecraft Yggdrasil 皮肤站
[![Ask DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/water2004/element-skin)

基于 Vue 3 + FastAPI 的 Minecraft 外置登录系统，提供现代化的 UI 体验和完整的 Yggdrasil 协议支持。

![](./img/root.png)

## 功能特性

- ✅ **完整协议支持**: 完善的 Yggdrasil API 实现，兼容所有主流启动器。
- ✅ **皮肤管理**: 支持皮肤/披风上传及 3D 实时预览。
- ✅ **邮箱验证**: 完整的注册验证码及密码找回流程（支持 SMTP）。
- ✅ **管理系统**: 响应式管理后台，支持用户管理、邀请码、轮播图及邮件服务配置。
- ✅ **安全防护**: 内置速率限制 (Rate Limiting) 及安全防护机制。
- ✅ **灵活部署**: 支持 Docker 一键部署及子目录 (Sub-path) 部署。

---

## 快速开始

### 开发环境

#### 后端
```bash
cd skin-backend
python -m venv .venv
.venv\Scripts\activate  # Windows
# source .venv/bin/activate  # Linux/macOS
pip install -r requirements.txt
python gen_key.py  # 生成 RSA 密钥
uvicorn routes_reference:app --reload
```

#### 前端
```bash
cd element-skin
npm install
npm run dev
```
访问 http://localhost:5173

---

## Docker 部署

### 镜像地址 (GHCR)

本项目支持自动构建镜像，你可以直接从 GitHub Container Registry 获取：

- **后端**: `ghcr.io/water2004/element-skin-backend`
- **前端**: `ghcr.io/water2004/element-skin-frontend`

### 1. 准备配置

创建 `config.yaml`（参考 `skin-backend/config.yaml`）：
```yaml
jwt:
  secret: "随机字符串"
database:
  path: "/data/yggdrasil.db"
textures:
  directory: "/data/textures"
```

### 2. 启动容器

#### 默认方案：使用 GHCR 镜像（根目录部署）

创建 `docker-compose.yml`（或直接替换现有配置）：

```yaml
version: '3.8'
networks:
  element-skin:
    driver: bridge
    ipam:
      config:
        - subnet: 172.18.0.0/16
          gateway: 172.18.0.1

services:
  # 后端服务
  backend:
    image: ghcr.io/water2004/element-skin-backend:latest
    container_name: element-skin-backend
    restart: unless-stopped
    ports:
      - "8000:8000"
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./data:/data
    networks:
      - element-skin

  # 前端服务
  frontend:
    image: ghcr.io/water2004/element-skin-frontend:latest
    container_name: element-skin-frontend
    restart: unless-stopped
    ports:
      - "3000:80"
    networks:
      - element-skin
```

启动容器：

```bash
docker compose up -d

# 更新到最新镜像版本
docker compose pull
docker compose up -d
```

#### 子目录部署方案（必须本地构建）

保持原始 `docker-compose.yml` 配置，使用本地构建：

```bash
# 子目录部署示例1（前端在 /skin/，后端在 /skinapi）
VITE_BASE_PATH=/skin/ docker compose up -d --build

# 子目录部署示例2（前端在 /skin/，后端在 /skin/api/）
VITE_BASE_PATH=/skin/ VITE_API_BASE=/skin/api docker compose up -d --build

# 低内存环境部署（跳过前端类型检查，减少构建时内存占用）
BUILD_MODE=low-memory docker compose up -d --build
```

### 3. 配置主机 Nginx

根据部署方案选择相应的配置。

#### 默认方案 Nginx 配置（根目录部署）

```nginx
server {
    listen 80;
    server_name yourdomain.com;

    location / {
        proxy_pass http://localhost:3000;
    }

    # 后端 API（/skinapi 前缀）
    location /skinapi/ {
        proxy_pass http://localhost:8000/skinapi/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # 后端 API（/skinapi 本身）
    location = /skinapi {
        proxy_pass http://localhost:8000/skinapi;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

#### 子目录部署方案1 Nginx 配置（前端 /skin/，后端 /skinapi）

```nginx
server {
    listen 80;
    server_name yourdomain.com;

    # 前端子目录
    location /skin/ {
        proxy_pass http://localhost:3000/;
    }

    # 后端 API（/skinapi 前缀）
    location /skinapi/ {
        proxy_pass http://localhost:8000/skinapi/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # 后端 API（/skinapi 本身）
    location = /skinapi {
        proxy_pass http://localhost:8000/skinapi;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

#### 子目录部署方案2 Nginx 配置（前端 /skin/，后端 /skin/api/）

```nginx
server {
    listen 80;
    server_name yourdomain.com;

    # 前端子目录
    location /skin/ {
        proxy_pass http://localhost:3000/;
    }

    # 后端 API（/skin/api 前缀）
    location /skin/api/ {
        proxy_pass http://localhost:8000/skin/api/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }

    # 后端 API（/skin/api 本身）
    location = /skin/api {
        proxy_pass http://localhost:8000/skin/api;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

---

## 首次配置

1. **管理员账号**: 注册的第一个用户将自动获得管理员权限。
2. **站点配置**: 登录后进入「管理面板」→「站点设置」，配置 **站点 URL**（必须与实际访问地址一致，否则材质无法加载）。
3. **邮件服务**: 进入「管理面板」→「邮件服务」，配置 SMTP 信息并开启 **邮件验证开关**，即可启用注册验证码和找回密码功能。

---

## 部署方案对比

| 方案 | 构建方式 | 前端路径 | 后端路径 | 环境变量配置 | 命令 |
|-----|---------|---------|---------|---------|------|
| **默认方案** | GHCR 镜像 | `/` | `/skinapi` 等 | 无需配置 | `docker compose up -d` |
| **子目录方案1** | 本地构建 | `/skin/` | `/skinapi` 等 | `VITE_BASE_PATH=/skin/` | `VITE_BASE_PATH=/skin/ docker compose up -d --build` |
| **子目录方案2** | 本地构建 | `/skin/` | `/skin/api/` | `VITE_BASE_PATH=/skin/` `VITE_API_BASE=/skin/api` | `VITE_BASE_PATH=/skin/ VITE_API_BASE=/skin/api docker compose up -d --build` |

> **强烈推荐**：使用 **默认方案**（GHCR 镜像），无需本地构建，开箱即用。*子目录部署因需要环境变量定制，必须进行本地构建。*

---

## 项目结构

```
element-skin/
├── element-skin/       # 前端（Vue 3 + Element Plus）
├── skin-backend/       # 后端（FastAPI + SQLite）
├── config.yaml         # 配置文件（手动创建）
├── data/               # 数据目录（自动生成：数据库、材质、密钥）
├── docker-compose.yml  # Docker 编排
└── nginx-host.conf     # Nginx 参考配置
```

---

## 技术栈

- **Frontend**: Vue 3, Vite, Element Plus, SkinView3D
- **Backend**: FastAPI, aiosqlite, aiosmtplib, PyJWT
- **Database**: SQLite (WAL mode)

---

## 许可证

[MIT License](LICENSE)
