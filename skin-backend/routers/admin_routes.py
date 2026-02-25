"""管理员模块路由"""

from fastapi import (
    APIRouter,
    HTTPException,
    Depends,
    Body,
    UploadFile,
    File,
)
from fastapi.security import HTTPBearer, HTTPAuthorizationCredentials
import os
import uuid

from utils.jwt_utils import decode_jwt_token
from database_module import Database
from config_loader import Config

router = APIRouter()
security = HTTPBearer()


def setup_routes(db: Database, admin_backend, rate_limiter, config: Config):
    """设置路由（注入依赖）"""

    async def get_current_user(creds: HTTPAuthorizationCredentials = Depends(security)):
        token = creds.credentials
        payload = decode_jwt_token(token)
        if not payload:
            raise HTTPException(status_code=401, detail="invalid or expired token")
        return payload

    def admin_required(payload: dict = Depends(get_current_user)):
        if not payload.get("is_admin"):
            raise HTTPException(status_code=403, detail="admin required")
        return payload

    @router.get("/admin/settings")
    async def get_admin_settings(payload: dict = Depends(admin_required)):
        return await admin_backend.get_admin_settings()

    @router.post("/admin/settings")
    async def save_admin_settings(
        payload: dict = Depends(admin_required), body: dict = Body(...)
    ):
        await admin_backend.save_admin_settings(body)
        return {"ok": True}

    @router.get("/admin/users")
    async def get_admin_users(payload: dict = Depends(admin_required)):
        return await admin_backend.get_admin_users()

    @router.get("/admin/users/{user_id}")
    async def get_single_user_admin(user_id: str, payload: dict = Depends(admin_required)):
        return await admin_backend.get_user_info(user_id)

    @router.post("/admin/users/{user_id}/toggle-admin")
    async def toggle_user_admin(user_id: str, payload: dict = Depends(admin_required)):
        actor_id = payload.get("sub")
        await admin_backend.toggle_user_admin(user_id, actor_id)
        return {"ok": True}

    @router.delete("/admin/users/{user_id}")
    async def delete_user_admin(user_id: str, payload: dict = Depends(admin_required)):
        await admin_backend.delete_user(user_id, is_admin_action=True)
        return {"ok": True}

    @router.post("/admin/users/{user_id}/ban")
    async def ban_user(
        user_id: str, payload: dict = Depends(admin_required), body: dict = Body(...)
    ):
        banned_until = body.get("banned_until")
        if banned_until is None:
            raise HTTPException(status_code=400, detail="banned_until is required")
        res = await admin_backend.ban_user(user_id, banned_until, payload.get("sub"))
        return {"ok": True, "banned_until": res}

    @router.post("/admin/users/{user_id}/unban")
    async def unban_user(user_id: str, payload: dict = Depends(admin_required)):
        await db.user.unban(user_id)
        return {"ok": True}

    @router.get("/admin/invites")
    async def get_admin_invites(payload: dict = Depends(admin_required)):
        invites = await db.user.list_invites()
        return [
            {
                "code": row.code,
                "created_at": row.created_at,
                "used_by": row.used_by,
                "total_uses": row.total_uses,
                "used_count": row.used_count,
                "note": row.note,
            }
            for row in invites
        ]

    @router.post("/admin/invites")
    async def create_admin_invite(
        payload: dict = Depends(admin_required), body: dict = Body(None)
    ):
        code = body.get("code") if body else None
        total_uses = body.get("total_uses", 1) if body else 1
        note = body.get("note", "") if body else ""
        new_code = await admin_backend.create_invite(code, total_uses, note)
        return {"code": new_code, "total_uses": total_uses, "note": note}

    @router.delete("/admin/invites/{code}")
    async def delete_admin_invite(code: str, payload: dict = Depends(admin_required)):
        await db.user.delete_invite(code)
        return {"ok": True}

    @router.get("/admin/official-whitelist")
    async def get_official_whitelist(endpoint_id: int, payload: dict = Depends(admin_required)):
        return await admin_backend.get_official_whitelist(endpoint_id)

    @router.post("/admin/official-whitelist")
    async def add_official_whitelist(payload: dict = Depends(admin_required), body: dict = Body(...)):
        username = body.get("username")
        endpoint_id = body.get("endpoint_id")
        if endpoint_id is None:
            raise HTTPException(status_code=400, detail="endpoint_id is required")
        return await admin_backend.add_official_whitelist_user(username, endpoint_id)

    @router.delete("/admin/official-whitelist/{username}")
    async def remove_official_whitelist(username: str, endpoint_id: int, payload: dict = Depends(admin_required)):
        return await admin_backend.remove_official_whitelist_user(username, endpoint_id)

    @router.post("/admin/carousel")
    async def upload_carousel(
        file: UploadFile = File(...),
        payload: dict = Depends(admin_required)
    ):
        ext = os.path.splitext(file.filename)[1].lower()
        if ext not in [".png", ".jpg", ".jpeg", ".webp"]:
            raise HTTPException(status_code=400, detail="Unsupported file format")
        
        filename = f"{uuid.uuid4().hex}{ext}"
        content = await file.read()
        return await admin_backend.upload_carousel_image(filename, content)

    @router.delete("/admin/carousel/{filename}")
    async def delete_carousel(filename: str, payload: dict = Depends(admin_required)):
        return await admin_backend.delete_carousel_image(filename)

    return router
