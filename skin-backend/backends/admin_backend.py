from typing import Any, List, Dict
import time
import secrets
import os
import re
from fastapi import HTTPException

from utils.typing import InviteCode
from database_module import Database
from config_loader import Config

class AdminBackend:
    def __init__(self, db: Database, config: Config):
        self.db = db
        self.config = config

    async def get_admin_settings(self):
        settings = await self.db.setting.get_all()
        fallbacks = await self.db.fallback.list_endpoints()
        fallback_strategy = settings.get("fallback_strategy", "serial")
        primary_fallback = fallbacks[0] if fallbacks else None
        
        return {
            "site_name": settings.get("site_name", "皮肤站"),
            "site_url": settings.get("site_url", ""),
            "require_invite": settings.get("require_invite", "false") == "true",
            "allow_register": settings.get("allow_register", "true") == "true",
            "max_texture_size": int(settings.get("max_texture_size", "1024")),
            "rate_limit_enabled": settings.get("rate_limit_enabled", "true") == "true",
            "rate_limit_auth_attempts": int(settings.get("rate_limit_auth_attempts", "5")),
            "rate_limit_auth_window": int(settings.get("rate_limit_auth_window", "15")),
            "jwt_expire_days": int(settings.get("jwt_expire_days", "7")),
            "microsoft_client_id": settings.get("microsoft_client_id", ""),
            "microsoft_client_secret": settings.get("microsoft_client_secret", ""),
            "microsoft_redirect_uri": settings.get("microsoft_redirect_uri", "http://localhost:8000/microsoft/callback"),
            "fallbacks": fallbacks,
            "fallback_strategy": fallback_strategy,
            "enable_skin_library": settings.get("enable_skin_library", "true") == "true",
            "email_verify_enabled": settings.get("email_verify_enabled", "false") == "true",
            "email_verify_ttl": int(settings.get("email_verify_ttl", "300")),
            "enable_strong_password_check": settings.get("enable_strong_password_check", "false") == "true",
            "smtp_host": settings.get("smtp_host", ""),
            "smtp_port": settings.get("smtp_port", "465"),
            "smtp_user": settings.get("smtp_user", ""),
            "smtp_ssl": settings.get("smtp_ssl", "true") == "true",
            "smtp_sender": settings.get("smtp_sender", ""),
        }

    async def save_admin_settings(self, body: dict):
        if "fallbacks" in body:
            fallbacks = self._validate_fallback_services(body.get("fallbacks"))
            await self.db.fallback.save_endpoints(fallbacks)

        for key in [
            "site_name", "site_url", "require_invite", "allow_register", "max_texture_size",
            "rate_limit_enabled", "rate_limit_auth_attempts", "rate_limit_auth_window",
            "jwt_expire_days", "microsoft_client_id", "microsoft_client_secret",
            "microsoft_redirect_uri", "fallback_strategy", "enable_skin_library",
            "email_verify_enabled", "email_verify_ttl", "enable_strong_password_check",
            "smtp_host", "smtp_port", "smtp_user", "smtp_password", "smtp_ssl", "smtp_sender",
        ]:
            if key in body:
                val = body[key]
                value = "true" if isinstance(val, bool) and val else ("false" if isinstance(val, bool) else str(val))
                if key == "smtp_password" and not value:
                    continue
                await self.db.setting.set(key, value)

    def _validate_fallback_services(self, services: Any) -> list[dict]:
        if not isinstance(services, list):
            raise HTTPException(status_code=400, detail="fallbacks must be a list")

        normalized: list[dict] = []
        for idx, entry in enumerate(services, start=1):
            if not isinstance(entry, dict):
                raise HTTPException(status_code=400, detail="invalid fallback entry")

            endpoint_id = entry.get("id")
            if endpoint_id is not None:
                try:
                    endpoint_id = int(endpoint_id)
                except (TypeError, ValueError):
                    raise HTTPException(status_code=400, detail=f"fallback[{idx}] id invalid")
            
            session_url = str(entry.get("session_url", "")).strip()
            account_url = str(entry.get("account_url", "")).strip()
            services_url = str(entry.get("services_url", "")).strip()
            cache_ttl = entry.get("cache_ttl", 60)
            raw_domains = entry.get("skin_domains", "")
            
            if not session_url or not account_url or not services_url:
                raise HTTPException(status_code=400, detail=f"fallback[{idx}] urls are required")

            if isinstance(raw_domains, list):
                skin_domains = [str(item).strip() for item in raw_domains if str(item).strip()]
            else:
                skin_domains = [item.strip() for item in str(raw_domains).split(",") if item.strip()]
            
            try:
                cache_ttl = int(cache_ttl)
            except (TypeError, ValueError):
                raise HTTPException(status_code=400, detail=f"fallback[{idx}] cache_ttl invalid")
            
            if cache_ttl < 0:
                raise HTTPException(status_code=400, detail=f"fallback[{idx}] cache_ttl must be non-negative")

            normalized.append({
                "id": endpoint_id,
                "session_url": session_url,
                "account_url": account_url,
                "services_url": services_url,
                "cache_ttl": cache_ttl,
                "skin_domains": ",".join(skin_domains),
                "enable_profile": bool(entry.get("enable_profile", True)),
                "enable_hasjoined": bool(entry.get("enable_hasjoined", True)),
                "enable_whitelist": bool(entry.get("enable_whitelist", False)),
                "note": str(entry.get("note", "")).strip(),
            })
        return normalized

    async def get_admin_users(self):
        users = await self.db.user.list_users(limit=1000, offset=0)
        result = []
        for row in users:
            profile_count = await self.db.user.count_profiles_by_user(row.id)
            result.append({
                "id": row.id,
                "email": row.email,
                "display_name": row.display_name or "",
                "is_admin": bool(row.is_admin),
                "banned_until": row.banned_until,
                "profile_count": profile_count,
            })
        return result

    async def toggle_user_admin(self, user_id: str, actor_id: str):
        if actor_id == user_id:
            raise HTTPException(status_code=403, detail="cannot change own admin status")
        new_status = await self.db.user.toggle_admin(user_id)
        if new_status == -1:
            raise HTTPException(status_code=404, detail="user not found")

    async def ban_user(self, user_id, banned_until, actor_id):
        user_row = await self.db.user.get_by_id(user_id)
        if not user_row:
            raise HTTPException(status_code=404, detail="user not found")
        if user_row.is_admin:
            raise HTTPException(status_code=403, detail="cannot ban admin user")
        await self.db.user.ban(user_id, banned_until)
        return banned_until

    async def create_invite(self, code, total_uses, note: str = ""):
        if code:
            if not (6 <= len(code) <= 32) or not re.match(r"^[a-zA-Z0-9_-]+$", code):
                raise HTTPException(status_code=400, detail="Invalid code format")
        else:
            code = secrets.token_urlsafe(16)

        if await self.db.user.get_invite(code):
            raise HTTPException(status_code=400, detail="invite code already exists")

        await self.db.user.create_invite(InviteCode(code, int(time.time() * 1000), total_uses=total_uses, note=note))
        return code

    async def upload_carousel_image(self, filename: str, content: bytes):
        directory = self.config.get("carousel.directory", "carousel")
        os.makedirs(directory, exist_ok=True)
        with open(os.path.join(directory, filename), "wb") as f:
            f.write(content)
        return {"filename": filename}

    async def delete_carousel_image(self, filename: str):
        directory = self.config.get("carousel.directory", "carousel")
        file_path = os.path.join(directory, filename)
        if os.path.dirname(os.path.abspath(file_path)) != os.path.abspath(directory):
            raise HTTPException(status_code=400, detail="Invalid filename")
        if os.path.exists(file_path):
            os.remove(file_path)
            return {"ok": True}
        raise HTTPException(status_code=404, detail="File not found")

    async def get_official_whitelist(self, endpoint_id: int):
        return await self.db.fallback.list_whitelist_users(endpoint_id)

    async def add_official_whitelist_user(self, username: str, endpoint_id: int):
        if not username:
            raise HTTPException(status_code=400, detail="username required")
        await self.db.fallback.add_whitelist_user(username, endpoint_id)
        return {"ok": True}

    async def remove_official_whitelist_user(self, username: str, endpoint_id: int):
        if not username:
            raise HTTPException(status_code=400, detail="username required")
        await self.db.fallback.remove_whitelist_user(username, endpoint_id)
        return {"ok": True}
