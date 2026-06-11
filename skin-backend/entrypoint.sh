#!/bin/bash
set -e

# 1. 确保静态资源子目录存在
# 这些目录位于挂载的 /app/frontend 卷内，会被持久化
mkdir -p /app/frontend/static/textures
mkdir -p /app/frontend/static/carousel

# --- 2. 释放前端编译产物 ---
echo "正在释放前端静态文件到 /app/frontend..."

# 若用户已自定义 favicon.ico，先备份到临时文件。
# 注意：cp -rf 会覆盖目标里的 favicon.ico，因此备份必须在任何复制动作之前完成。
USER_FAVICON=""
if [ -f "/app/frontend/favicon.ico" ]; then
    USER_FAVICON="$(mktemp)"
    cp -f /app/frontend/favicon.ico "$USER_FAVICON"
    echo "检测到自定义 favicon.ico，将在释放后保留。"
fi

# 保护 static 目录和 favicon.ico，仅清空其它的前端入口文件（index.html, assets 等）
if [ -d "/app/frontend" ]; then
    find /app/frontend -mindepth 1 -maxdepth 1 ! -name 'static' ! -name 'favicon.ico' -exec rm -rf {} +
fi

# 复制新前端产物
cp -rf /app/frontend_dist/* /app/frontend/

# 还原用户自定义 favicon（覆盖 dist 中自带的默认值）
if [ -n "$USER_FAVICON" ]; then
    cp -f "$USER_FAVICON" /app/frontend/favicon.ico
    rm -f "$USER_FAVICON"
fi

# --- 2.5 运行时路径替换 ---
# 获取环境变量并处理默认值
BASE_PATH=${VITE_BASE_PATH:-/}
API_BASE=${VITE_API_BASE:-/skinapi}

# 规范化 BASE_PATH：确保以 / 开头且以 / 结尾
[[ $BASE_PATH != /* ]] && BASE_PATH="/$BASE_PATH"
[[ $BASE_PATH != */ ]] && BASE_PATH="$BASE_PATH/"

echo "正在动态替换前端路径: BASE=$BASE_PATH, API=$API_BASE"

# 扫描并替换 index.html 和 JS 文件中的占位符
# 使用 | 作为 sed 分隔符以处理路径中的斜杠
find /app/frontend -type f \( -name "*.js" -o -name "*.html" \) -exec sed -i "s|/VITE_BASE_PATH_PLACEHOLDER/|$BASE_PATH|g" {} +
find /app/frontend -type f \( -name "*.js" -o -name "*.html" \) -exec sed -i "s|VITE_API_BASE_PLACEHOLDER|$API_BASE|g" {} +

echo "前端文件释放及路径配置完成。"

# --- 3. 密钥生成逻辑 ---
KEY_DIR="/app/data"
mkdir -p "$KEY_DIR"

# Check if Union key mode is enabled
USE_UNION_CFG=$(python3 -c "from utils.config_loader import config; print(config.get('keys.use_union_key', False))" 2>/dev/null || echo "False")
if [ "$USE_UNION_CFG" = "True" ]; then
    echo "Union key mode enabled. Skipping default key generation."
else
    if [ ! -f "$KEY_DIR/private.pem" ] || [ ! -f "$KEY_DIR/public.pem" ]; then
        echo "密钥文件不存在，正在生成到 $KEY_DIR..."
        python3 gen_key.py "$KEY_DIR"
        echo "密钥已生成。"
    else
        echo "密钥文件已存在，跳过生成。"
    fi
fi

# 启动应用
exec "$@"
