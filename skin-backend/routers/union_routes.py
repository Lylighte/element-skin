"""Union 联合认证系统路由"""

import json
import os
import time
import logging

from fastapi import (
    APIRouter,
    Request,
    HTTPException,
    Depends,
    Body,
    Header,
    Query,
)
from fastapi.responses import JSONResponse, RedirectResponse
from typing import Optional

from config_loader import Config
from routers.deps import get_current_user as deps_get_current_user, admin_required as deps_admin_required
from utils.crypto import compute_key_fingerprint

logger = logging.getLogger("union")

router = APIRouter()


def setup_routes(union_backend, rate_limiter, config: Config):
    """设置 Union 路由（注入依赖）"""

    async def verify_union_request(request: Request):
        """UnionHostVerify: delegate to backend."""
        await union_backend.verify_union_request_inbound(request)
        return True

    # ========================================================================
    # GROUP A: Union Inbound API (protected by UnionHostVerify)
    # ========================================================================

    @router.post("/api/union/member/updatelist")
    async def union_update_list(_verified=Depends(verify_union_request)):
        """Union pushes updated server list."""
        if not await union_backend.is_update_enabled():
            return {"ok": True, "message": "Updates from Union are disabled"}
        success = await union_backend.fetch_server_list()
        if not success:
            raise HTTPException(status_code=502, detail="Failed to fetch server list from Union")
        return {"ok": True}

    @router.post("/api/union/member/updateprivatekey")
    async def union_update_private_key(_verified=Depends(verify_union_request)):
        """Union pushes updated private key."""
        if not await union_backend.is_update_enabled():
            return {"ok": True, "message": "Updates from Union are disabled"}
        success = await union_backend.fetch_private_key()
        if not success:
            raise HTTPException(status_code=502, detail="Failed to fetch private key from Union")
        return {"ok": True}

    @router.post("/api/union/member/updatebackendkey")
    async def union_update_backend_key(request: Request, _verified=Depends(verify_union_request)):
        """Union updates this member's authentication key."""
        if not await union_backend.is_update_enabled():
            return {"ok": True, "message": "Updates from Union are disabled"}
        body = json.loads(getattr(request.state, "union_body", "{}"))
        new_key = body.get("key")
        if not new_key:
            raise HTTPException(status_code=400, detail="key is required")
        await union_backend.update_settings({"union_member_key": new_key})
        logger.info("Union member key updated by Union server")
        return {"ok": True}

    @router.post("/api/union/member/sync")
    async def union_sync(request: Request, _verified=Depends(verify_union_request)):
        """Union triggers profile sync."""
        body = json.loads(getattr(request.state, "union_body", "{}"))
        profile_list = body.get("profileList", {})
        logger.info(f"Received sync trigger from Union with {len(profile_list)} profiles")
        success = await union_backend.sync_profiles()
        if not success:
            raise HTTPException(status_code=502, detail="Failed to sync profiles to Union")
        return {"ok": True}

    @router.post("/api/union/member/remapuuid")
    async def union_remap_uuid(request: Request, _verified=Depends(verify_union_request)):
        """Union pushes UUID remappings."""
        body = json.loads(getattr(request.state, "union_body", "{}"))
        remapped = body.get("remapped_uuid", {})
        if not remapped:
            raise HTTPException(status_code=400, detail="remapped_uuid is required")
        await union_backend.remap_uuids(remapped)
        logger.info(f"Applied {len(remapped)} UUID remappings from Union")
        return {"ok": True}

    @router.post("/api/union/member/diagnose")
    async def union_diagnose(request: Request, _verified=Depends(verify_union_request)):
        """Diagnostic echo."""
        body = json.loads(getattr(request.state, "union_body", "{}"))
        nonce = body.get("nonce", "")
        return {"nonce": nonce, "timestamp": time.time()}

    @router.get("/api/union/member/queryemail")
    async def union_query_email(
        username: str = Query(...),
        _verified=Depends(verify_union_request),
    ):
        """Union queries user email by player name (for blacklist)."""
        email = await union_backend.get_email_by_username(username)
        if email:
            return {"email": email}
        return JSONResponse(status_code=204)

    # ========================================================================
    # GROUP B: Union Public API (no auth)
    # ========================================================================

    @router.get("/api/union/member/")
    @router.get("/api/union/member")
    async def union_hello():
        """Public endpoint exposing this server's Union capabilities."""
        settings = await union_backend.get_settings()
        server_list_version = int(settings.get("union_server_list_version", "0"))
        private_key_version = int(settings.get("union_private_key_version", "0"))

        enabled_features = ["unionBlacklist"]
        if await union_backend.is_email_verify_enabled():
            enabled_features.append("emailVerification")
        if await union_backend.is_invitation_codes_for_union_enabled():
            enabled_features.append("invitationCodesForUnion")
        if await union_backend.is_oauth2_enabled():
            enabled_features.append("unionOAuth2")

        return {
            "yggdrasilApiVersion": "2.0.0",
            "serverListVersion": server_list_version,
            "privateKeyVersion": private_key_version,
            "enabledFeatures": enabled_features,
        }

    # ========================================================================
    # GROUP C: Union OAuth2
    # ========================================================================

    @router.get("/api/union/member/oauth2/")
    @router.get("/api/union/member/oauth2")
    async def union_oauth2_pubkey():
        """Expose this server's OAuth2 signature public key."""
        settings = await union_backend.get_settings()
        sig_pub_key = settings.get("union_oauth2_sig_public_key", "")
        if not sig_pub_key:
            raise HTTPException(status_code=503, detail="OAuth2 signature public key not configured")
        return {"signaturePublicKey": sig_pub_key}

    @router.get("/api/union/member/oauth2/grant")
    async def union_oauth2_grant(
        request: Request,
        payload: dict = Depends(deps_get_current_user),
    ):
        """OAuth2 grant: build encrypted user info token and redirect to Union."""
        if not await union_backend.is_oauth2_enabled():
            raise HTTPException(status_code=403, detail="Union OAuth2 is not enabled")
        user_id = payload.get("sub")
        user = await union_backend.get_user(user_id)
        if not user:
            raise HTTPException(status_code=404, detail="User not found")

        token = await union_backend.build_oauth2_token(user)
        if not token:
            raise HTTPException(status_code=500, detail="Failed to build OAuth2 token")

        settings = await union_backend.get_settings()
        api_root = settings.get("union_api_root", "")
        if not api_root:
            raise HTTPException(status_code=503, detail="Union API root not configured")

        # Build redirect URL with preserved query params
        query_params = dict(request.query_params)
        query_params["userInfoToken"] = token
        from urllib.parse import urlencode
        redirect_url = api_root.rstrip("/") + "/oauth2/continue?" + urlencode(query_params)

        return RedirectResponse(url=redirect_url)

    # ========================================================================
    # GROUP D: User-facing Union (JWT auth)
    # ========================================================================

    @router.get("/union/profiles")
    async def union_profiles_render(payload: dict = Depends(deps_get_current_user)):
        """Get profile binding info for all user's profiles.

        Matches reference project (UnionProfileController@render) exactly:
        1. Fire ALL requests concurrently (all dup + all self at once)
        2. First pass: populate dup_name for all profiles
        3. Second pass: populate self, then filter dup_name (exclude own + already-bound)
        """
        user_id = payload.get("sub")
        local_profiles = await union_backend.get_user_profiles(user_id)

        import asyncio

        # Step 1: Fire ALL requests concurrently (matching reference)
        n = len(local_profiles)
        dup_coros = [union_backend.get_profile_unmapped_byname(p.name) for p in local_profiles]
        self_coros = [union_backend.get_profile_detail(p.id) for p in local_profiles]
        all_results = await asyncio.gather(
            *(dup_coros + self_coros), return_exceptions=True,
        )
        dup_results = all_results[:n]
        self_results = all_results[n:]

        # Step 2: First pass — populate dup_name (matching reference)
        items = []
        for dup in dup_results:
            raw_dup = dup if not isinstance(dup, Exception) else None
            items.append({"dup_name": raw_dup})

        # Step 3: Second pass — populate self, then filter dup_name
        for i, profile in enumerate(local_profiles):
            detail = self_results[i]
            raw_detail = detail if not isinstance(detail, Exception) else None
            detail_data = raw_detail if raw_detail else None
            items[i]["self"] = detail_data

            # Filter dup_name: exclude own internal_id and already-bound internal_ids
            # Reference: collect(dup_name)->keyBy('internal_id')->except(self['internal_id'])->diffKeys(bind->keyBy('internal_id'))
            raw_dup = items[i]["dup_name"]
            if detail_data and raw_dup:
                self_int_id = detail_data.get("internal_id") if isinstance(detail_data, dict) else None
                bound_ids = set()
                bind_list = detail_data.get("bind", []) if isinstance(detail_data, dict) else []
                if isinstance(bind_list, list):
                    for b in bind_list:
                        bid = b.get("internal_id") if isinstance(b, dict) else None
                        if bid:
                            bound_ids.add(bid)

                dup_source = []
                if isinstance(raw_dup, list):
                    dup_source = raw_dup
                elif isinstance(raw_dup, dict) and "data" in raw_dup and isinstance(raw_dup["data"], list):
                    dup_source = raw_dup["data"]

                if dup_source:
                    filtered = []
                    for d in dup_source:
                        if not isinstance(d, dict):
                            continue
                        did = d.get("internal_id")
                        if did == self_int_id or did in bound_ids:
                            continue
                        filtered.append(d)
                    items[i]["dup_name"] = filtered

        # Step 4: Build response with local profile data for Vue frontend
        results = []
        for i, profile in enumerate(local_profiles):
            dup_val = items[i].get("dup_name", [])
            results.append({
                "id": profile.id,
                "name": profile.name,
                "self": items[i].get("self"),
                "dup_name": dup_val if isinstance(dup_val, list) else [],
            })

        return {"items": results}

    @router.post("/union/bind")
    async def union_bind(payload: dict = Depends(deps_get_current_user), body: dict = Body(...)):
        """Request a bind token from Union."""
        uuid = body.get("uuid")
        if not uuid:
            raise HTTPException(status_code=400, detail="uuid is required")

        # Verify profile ownership
        user_id = payload.get("sub")
        if not await union_backend.verify_profile_ownership(user_id, uuid):
            raise HTTPException(status_code=403, detail="Profile not owned by user")

        result = await union_backend.request_bind_token(uuid)
        if not result or "token" not in result:
            raise HTTPException(status_code=502, detail="Failed to get bind token from Union")

        return {"token": result["token"]}

    @router.post("/union/unbind")
    async def union_unbind(payload: dict = Depends(deps_get_current_user), body: dict = Body(...)):
        """Unbind a profile from Union."""
        uuid = body.get("uuid")
        if not uuid:
            raise HTTPException(status_code=400, detail="uuid is required")

        user_id = payload.get("sub")
        if not await union_backend.verify_profile_ownership(user_id, uuid):
            raise HTTPException(status_code=403, detail="Profile not owned by user")

        success = await union_backend.request_unbind(uuid)
        if not success:
            raise HTTPException(status_code=502, detail="Failed to unbind from Union")

        return {"ok": True}

    @router.post("/union/bindto")
    async def union_bind_to(payload: dict = Depends(deps_get_current_user), body: dict = Body(...)):
        """Bind a profile to another profile using a token."""
        uuid = body.get("uuid")
        token = body.get("token")
        if not uuid or not token:
            raise HTTPException(status_code=400, detail="uuid and token are required")

        user_id = payload.get("sub")
        if not await union_backend.verify_profile_ownership(user_id, uuid):
            raise HTTPException(status_code=403, detail="Profile not owned by user")

        success = await union_backend.request_bind_to(uuid, token)
        if not success:
            raise HTTPException(status_code=502, detail="Failed to bind to Union")

        return {"ok": True}

    @router.post("/union/remapuuid")
    async def union_remap_uuid_request(payload: dict = Depends(deps_get_current_user), body: dict = Body(...)):
        """Request UUID remapping across the federation."""
        me = body.get("me")
        target = body.get("target")
        if not me or not target:
            raise HTTPException(status_code=400, detail="me and target are required")

        user_id = payload.get("sub")
        if not await union_backend.verify_profile_ownership(user_id, me):
            raise HTTPException(status_code=403, detail="Profile not owned by user")

        success = await union_backend.request_remap_uuid(me, target)
        if not success:
            raise HTTPException(status_code=502, detail="Failed to request UUID remap")

        return {"ok": True}

    @router.get("/union/security/level")
    async def union_security_level(payload: dict = Depends(deps_get_current_user)):
        """Get this server's security level from Union.

        Note: Union API returns a bare integer for security level.
        The reference project (server-rendered Blade) passes this through directly.
        For our Vue SPA frontend, we wrap it in JSON for the frontend to consume.
        """
        level = await union_backend.get_security_level()
        if level is None:
            raise HTTPException(status_code=502, detail="Failed to get security level")
        return {"security_level": level}

    # ========================================================================
    # GROUP E: Admin Union (JWT + admin_required)
    # ========================================================================

    @router.get("/admin/union/settings")
    async def admin_get_union_settings(payload: dict = Depends(deps_admin_required)):
        """Get all Union configuration settings."""
        settings = await union_backend.get_settings()

        # Prefer file content if file exists, fallback to DB (migration phase)
        union_ygg_private_key = settings.get("union_ygg_private_key", "")
        union_key_path = "/app/data/union-ygg-private.pem"
        if os.path.exists(union_key_path):
            with open(union_key_path, "r") as f:
                union_ygg_private_key = f.read().strip()
        elif union_ygg_private_key:
            pass

        present = bool(union_ygg_private_key and "BEGIN" in union_ygg_private_key)
        fingerprint = compute_key_fingerprint(union_ygg_private_key) if present else ""

        return {
            "union_api_root": settings.get("union_api_root", ""),
            "union_member_key": settings.get("union_member_key", ""),
            "union_server_list_version": int(settings.get("union_server_list_version", "0")),
            "union_private_key_version": int(settings.get("union_private_key_version", "0")),
            "union_enable_update": settings.get("union_enable_update", "true"),
            "union_enable_oauth2": settings.get("union_enable_oauth2", "true"),
            "union_oauth2_sig_private_key": settings.get("union_oauth2_sig_private_key", ""),
            "union_oauth2_sig_public_key": settings.get("union_oauth2_sig_public_key", ""),
            "union_ygg_private_key_fingerprint": fingerprint,
            "union_ygg_private_key_present": present,
            "union_server_list": json.loads(settings.get("union_server_list", "[]")),
        }

    @router.post("/admin/union/settings")
    async def admin_save_union_settings(payload: dict = Depends(deps_admin_required), body: dict = Body(...)):
        """Save Union configuration settings."""
        allowed_keys = {
            "union_api_root", "union_member_key", "union_enable_update",
            "union_enable_oauth2", "union_oauth2_sig_private_key",
            "union_oauth2_sig_public_key",
        }
        bool_keys = {"union_enable_update", "union_enable_oauth2"}
        kv = {}
        for key, value in body.items():
            if key in allowed_keys:
                v = str(value).lower() if key in bool_keys else str(value)
                kv[key] = v
        await union_backend.update_settings(kv)

        return {"ok": True}

    @router.post("/admin/union/update-list")
    async def admin_update_list(payload: dict = Depends(deps_admin_required)):
        """Admin: manually trigger server list update."""
        success = await union_backend.fetch_server_list()
        if not success:
            raise HTTPException(status_code=502, detail="Failed to update server list")
        return {"ok": True}

    @router.post("/admin/union/update-key")
    async def admin_update_key(payload: dict = Depends(deps_admin_required)):
        """Admin: manually trigger private key update."""
        success = await union_backend.fetch_private_key()
        if not success:
            raise HTTPException(status_code=502, detail="Failed to update private key")
        return {"ok": True}

    @router.post("/admin/union/sync")
    async def admin_sync(payload: dict = Depends(deps_admin_required)):
        """Admin: manually trigger profile sync."""
        success = await union_backend.sync_profiles()
        if not success:
            raise HTTPException(status_code=502, detail="Failed to sync profiles")
        return {"ok": True}

    @router.post("/admin/union/diagnose")
    async def admin_diagnose(payload: dict = Depends(deps_admin_required)):
        """Admin: run connectivity diagnostic."""
        result = await union_backend.trigger_diagnose()
        return result

    @router.get("/admin/union/blacklist")
    async def admin_blacklist_list(
        q: Optional[str] = Query(None),
        page: int = Query(1),
        payload: dict = Depends(deps_admin_required),
    ):
        """Admin: query Union blacklist entries."""
        params = {}
        if q:
            params["q"] = q
        if page:
            params["page"] = page

        result = await union_backend.get_blacklist(params)
        if result is None:
            raise HTTPException(status_code=502, detail="Failed to query blacklist")
        return result

    @router.post("/admin/union/blacklist")
    async def admin_blacklist_create(payload: dict = Depends(deps_admin_required), body: dict = Body(...)):
        """Admin: create a new blacklist entry on Union."""
        email = body.get("email")
        reason = body.get("reason", "")
        if not email:
            raise HTTPException(status_code=400, detail="email is required")
        result = await union_backend.create_blacklist({"email": email, "reason": reason})
        if result is None:
            raise HTTPException(status_code=502, detail="Failed to create blacklist entry")
        return result

    @router.post("/admin/union/blacklist/{entry_id}/invalidate")
    async def admin_blacklist_invalidate(entry_id: str, payload: dict = Depends(deps_admin_required)):
        """Admin: invalidate/unban a blacklist entry."""
        success = await union_backend.invalidate_blacklist(entry_id)
        if not success:
            raise HTTPException(status_code=502, detail="Failed to invalidate blacklist entry")
        return {"ok": True}

    @router.delete("/admin/union/blacklist/{entry_id}")
    async def admin_blacklist_delete(entry_id: str, payload: dict = Depends(deps_admin_required)):
        """Admin: delete a blacklist entry."""
        success = await union_backend.delete_blacklist(entry_id)
        if not success:
            raise HTTPException(status_code=502, detail="Failed to delete blacklist entry")
        return {"ok": True}

    @router.post("/admin/union/generate-keypair")
    async def admin_generate_keypair(payload: dict = Depends(deps_admin_required)):
        """Admin: generate a new RSA keypair for OAuth2 signing."""
        keypair = union_backend.generate_rsa_keypair()
        return {"privateKey": keypair["private"], "publicKey": keypair["public"]}

    return router
