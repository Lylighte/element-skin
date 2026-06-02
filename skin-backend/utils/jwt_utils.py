"""站点鉴权令牌工具：短效 access token（无状态 JWT）+ 长效 refresh token（入库可撤销）"""

import jwt
import hashlib
import secrets
from datetime import datetime, timedelta, timezone
from typing import Dict, Optional, Tuple
from config_loader import config


_DEFAULT_SECRET = "dev-secret-default-key-at-least-32-chars-long"
# config.yaml 出厂占位密钥；运营若忘改即命中，与代码内 fallback 一并视为「未配置」。
_SHIPPED_PLACEHOLDER = "dev-secret-please-change-to-a-very-long-string-in-production"
_KNOWN_WEAK_SECRETS = frozenset({_DEFAULT_SECRET, _SHIPPED_PLACEHOLDER})

JWT_SECRET = config.get("jwt.secret", _DEFAULT_SECRET)
JWT_ALGO = "HS256"


def _validate_jwt_secret(secret: str) -> None:
    """校验 JWT 密钥强度，命中任一弱口令条件即抛 RuntimeError（启动期 fail-fast）。

    HS256 下「知道密钥即可伪造任意 sub 的 access token → 账号/管理员接管」，
    故启动期必须拒绝缺失、默认/占位、过短（< 32 字节）的密钥，绝不拖到运行期变神秘 500。
    """
    if not secret:
        raise RuntimeError("jwt.secret 未配置：请在配置文件中设置高熵密钥后再启动")
    if secret in _KNOWN_WEAK_SECRETS:
        raise RuntimeError("jwt.secret 仍为默认/占位值：必须改为随机高熵密钥后再启动")
    if len(secret.encode("utf-8")) < 32:
        raise RuntimeError("jwt.secret 过短：至少 32 字节")


def assert_jwt_secret_ok() -> None:
    """应用启动期（lifespan）调用，对当前 JWT_SECRET 做 fail-fast 校验。

    放启动期而非模块加载期：测试与工具脚本 import 本模块时复用 config.yaml 的出厂占位
    密钥，若模块级抛错会令整个测试套件在收集期失败；而 ASGI lifespan 仅在真正起服务
    （uvicorn）时触发，故能在生产启动时拦截弱密钥，又不影响 import。
    """
    _validate_jwt_secret(JWT_SECRET)

# access token 有效期（分钟）。代码默认 30 分钟，可经配置覆盖，但 config.yaml 不预置该键。
ACCESS_EXPIRE_MINUTES = int(config.get("jwt.access_expire_minutes", 30))


def _secure_cookie() -> bool:
    """站点以 https 提供时才下发 Secure cookie。"""
    return config.get("server.site_url", "http://localhost").startswith("https://")


def get_access_cookie_settings() -> dict:
    """access token cookie 配置（短效，30 分钟）。"""
    return {
        "key": "access_token",
        "value": "",  # 由调用方设置
        "httponly": True,
        "secure": _secure_cookie(),
        "samesite": "lax",
        "max_age": ACCESS_EXPIRE_MINUTES * 60,
        "path": "/",
    }


def get_refresh_cookie_settings() -> dict:
    """refresh token cookie 配置（长效，会话时长 = jwt_expire_days）。"""
    expire_days = int(config.get("jwt.expire_days", "7"))
    return {
        "key": "refresh_token",
        "value": "",  # 由调用方设置
        "httponly": True,
        "secure": _secure_cookie(),
        "samesite": "lax",
        "max_age": expire_days * 24 * 3600,
        "path": "/",
    }


def create_access_token(user_id: str, is_admin: bool) -> str:
    """签发短效 access token（无状态 JWT）。

    Args:
        user_id: 用户 ID
        is_admin: 是否为管理员（仅作初值，deps 会以库内实时值为准）

    Returns:
        str: JWT 令牌
    """
    payload = {
        "sub": user_id,
        "is_admin": is_admin,
        "type": "access",
        "exp": datetime.now(timezone.utc) + timedelta(minutes=ACCESS_EXPIRE_MINUTES),
    }
    return jwt.encode(payload, JWT_SECRET, algorithm=JWT_ALGO)


def decode_access_token(token: str) -> Optional[Dict]:
    """解码并校验 access token（签名 + exp + type=="access"）。无效返回 None。"""
    try:
        payload = jwt.decode(token, JWT_SECRET, algorithms=[JWT_ALGO])
    except jwt.ExpiredSignatureError:
        return None
    except Exception:
        return None
    if payload.get("type") != "access":
        return None
    return payload


def hash_refresh_token(raw: str) -> str:
    """对 refresh token 原文计算 SHA-256 哈希（入库与校验均用哈希）。"""
    return hashlib.sha256(raw.encode("utf-8")).hexdigest()


def generate_refresh_token() -> Tuple[str, str]:
    """生成一枚 refresh token，返回 (原文, 哈希)。原文下发给客户端 cookie，哈希入库。"""
    raw = secrets.token_urlsafe(48)
    return raw, hash_refresh_token(raw)
