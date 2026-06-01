"""Union 联合认证系统后端逻辑"""

import aiohttp
import asyncio
import json
import time
import base64
import hashlib
import hmac
import logging

from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding, rsa
from cryptography.hazmat.primitives.asymmetric.rsa import RSAPrivateKey, RSAPublicKey
from cryptography.hazmat.backends import default_backend

from fastapi import HTTPException
from database_module import Database
from config_loader import Config
from utils.typing import User

logger = logging.getLogger("union")


class UnionBackend:
    def __init__(self, db: Database, config: Config):
        self.db = db
        self.config = config
        self._union_public_key_cache: tuple[str, float] | None = None  # (key, fetch_time)

    # ========== Facade Methods (thin DB delegation for T5 router boundary fix) ==========

    # Feature flags (thin wrappers around db.union settings)
    async def is_update_enabled(self) -> bool:
        return (await self.db.union.get("union_enable_update", "true")).lower() == "true"

    async def is_oauth2_enabled(self) -> bool:
        return (await self.db.union.get("union_enable_oauth2", "true")).lower() == "true"

    async def is_restore_api_enabled(self) -> bool:
        return (await self.db.union.get("ygg_restore_api", "false")).lower() == "true"

    # Settings management
    async def get_settings(self) -> dict:
        return await self.db.union.get_all_settings()

    async def update_settings(self, kv: dict):
        """Save multiple union settings. Keys must be allowed keys."""
        allowed = {"union_api_root", "union_member_key", "union_enable_update",
                   "union_enable_oauth2", "union_oauth2_sig_private_key",
                   "union_oauth2_sig_public_key", "ygg_restore_api"}
        for key, value in kv.items():
            if key in allowed:
                await self.db.union.set(key, str(value))

    # User / profile helpers
    async def get_user(self, user_id: str):
        return await self.db.user.get_by_id(user_id)

    async def get_user_profiles(self, user_id: str) -> list:
        return await self.db.user.get_profiles_by_user(user_id)

    async def verify_profile_ownership(self, user_id: str, profile_id: str) -> bool:
        return await self.db.user.verify_profile_ownership(user_id, profile_id)

    # Email for blacklist
    async def get_email_by_username(self, username: str) -> str | None:
        return await self.db.union.get_email_by_username(username)

    # UUID remap delegation
    async def remap_uuids(self, remapped: dict):
        await self.db.union.remap_uuids(remapped)

    # UnionHostVerify: wraps nonce check + signature verification in one method
    async def verify_union_request_inbound(self, request) -> bool:
        """Complete UnionHostVerify: check nonce, verify RSA signature, log nonce."""
        signature = request.headers.get("X-Message-Signature")
        timestamp_str = request.headers.get("X-Message-Timestamp")
        nonce = request.headers.get("X-Message-Nonce")

        if not signature or not timestamp_str or not nonce:
            raise HTTPException(status_code=401, detail="Missing Union signature headers")

        if await self.db.union.is_nonce_used(nonce):
            raise HTTPException(status_code=401, detail="Nonce already used (replay detected)")

        try:
            ts = int(timestamp_str)
            now = int(time.time())
            if ts < now - 10 or ts > now + 30:
                raise HTTPException(status_code=401, detail="Timestamp out of acceptable window")
        except ValueError:
            raise HTTPException(status_code=401, detail="Invalid timestamp")

        union_pub_key = await self.get_union_public_key()
        if not union_pub_key:
            raise HTTPException(status_code=503, detail="Could not fetch Union public key")

        body_bytes = await request.body()
        body_str = body_bytes.decode("utf-8") if body_bytes else ""

        if not self.verify_union_signature(body_str, signature, timestamp_str, nonce, union_pub_key):
            raise HTTPException(status_code=401, detail="Invalid Union signature")

        await self.db.union.log_nonce(nonce)
        return True

    # Email verification / invitation codes feature flag
    async def is_email_verify_enabled(self) -> bool:
        val = await self.db.setting.get("email_verify_enabled", "false")
        return val == "true"

    async def is_invitation_codes_for_union_enabled(self) -> bool:
        val = await self.db.setting.get("invitation_codes_for_union_enabled", "false")
        return val == "true"

    # ========== Outbound HTTP helpers ==========

    async def _api_get(self, path: str, params: dict = None, timeout: int = 5, raw: bool = False):
        """Generic GET request to Union server."""
        api_root = await self.db.union.get("union_api_root", "")
        if not api_root:
            return None
        url = api_root.rstrip("/") + "/" + path.lstrip("/")
        member_key = await self.db.union.get("union_member_key", "")
        headers = {"X-Union-Member-Key": member_key}
        logger.debug(f"Union GET {url} headers={headers} params={params}")

        try:
            async with aiohttp.ClientSession() as session:
                async with session.get(url, headers=headers, params=params, timeout=timeout) as resp:
                    if raw:
                        return resp
                    if resp.status == 200:
                        text = await resp.text()
                        try:
                            return json.loads(text)
                        except json.JSONDecodeError:
                            logger.error(f"Union GET {path} 200: non-JSON response (content-type={resp.content_type}): {text[:300]}")
                            return None
                    logger.debug(f"Union GET {path} failed: status={resp.status}, body={(await resp.text())[:200]}")
                    return None
        except Exception as e:
            logger.error(f"Union GET {path} failed: {e}")
            return None

    async def _api_post(self, path: str, data: dict = None, timeout: int = 5, raw: bool = False):
        """Generic POST request to Union server."""
        api_root = await self.db.union.get("union_api_root", "")
        if not api_root:
            return None
        url = api_root.rstrip("/") + "/" + path.lstrip("/")
        member_key = await self.db.union.get("union_member_key", "")
        headers = {"X-Union-Member-Key": member_key}
        logger.debug(f"Union POST {url} headers={headers} body={json.dumps(data)[:500]}")

        try:
            async with aiohttp.ClientSession() as session:
                async with session.post(url, headers=headers, json=data, timeout=timeout) as resp:
                    if raw:
                        return resp
                    if resp.status == 200:
                        text = await resp.text()
                        try:
                            return json.loads(text)
                        except json.JSONDecodeError:
                            logger.debug(f"Union POST {path} 200: non-JSON (content-type={resp.content_type}), treating as success")
                            return {}
                    logger.debug(f"Union POST {path} failed: status={resp.status}, body={(await resp.text())[:200]}")
                    return None
        except Exception as e:
            logger.error(f"Union POST {path} failed: {e}")
            return None

    async def _api_put(self, path: str, data: dict = None, timeout: int = 5):
        """Generic PUT request to Union server."""
        api_root = await self.db.union.get("union_api_root", "")
        if not api_root:
            return None
        url = api_root.rstrip("/") + "/" + path.lstrip("/")
        member_key = await self.db.union.get("union_member_key", "")
        headers = {"X-Union-Member-Key": member_key}
        logger.debug(f"Union PUT {url} headers={headers} body={json.dumps(data)[:500]}")

        try:
            async with aiohttp.ClientSession() as session:
                async with session.put(url, headers=headers, json=data, timeout=timeout) as resp:
                    if resp.status == 200:
                        text = await resp.text()
                        try:
                            return json.loads(text)
                        except json.JSONDecodeError:
                            logger.error(f"Union PUT {path} 200: non-JSON response (content-type={resp.content_type}): {text[:300]}")
                            return None
                    logger.debug(f"Union PUT {path} failed: status={resp.status}, body={(await resp.text())[:200]}")
                    return None
        except Exception as e:
            logger.error(f"Union PUT {path} failed: {e}")
            return None

    async def _api_delete(self, path: str, timeout: int = 5):
        """Generic DELETE request to Union server."""
        api_root = await self.db.union.get("union_api_root", "")
        if not api_root:
            return None
        url = api_root.rstrip("/") + "/" + path.lstrip("/")
        member_key = await self.db.union.get("union_member_key", "")
        headers = {"X-Union-Member-Key": member_key}
        logger.debug(f"Union DELETE {url} headers={headers}")

        try:
            async with aiohttp.ClientSession() as session:
                async with session.delete(url, headers=headers, timeout=timeout) as resp:
                    if resp.status == 200:
                        text = await resp.text()
                        try:
                            return json.loads(text)
                        except json.JSONDecodeError:
                            logger.error(f"Union DELETE {path} 200: non-JSON response (content-type={resp.content_type}): {text[:300]}")
                            return None
                    logger.debug(f"Union DELETE {path} failed: status={resp.status}, body={(await resp.text())[:200]}")
                    return None
        except Exception as e:
            logger.error(f"Union DELETE {path} failed: {e}")
            return None

    # ========== Data Sync methods ==========

    async def fetch_server_list(self) -> bool:
        """Fetch server list from Union and cache locally."""
        result = await self._api_get("serverlist", raw=False)
        if result and "servers" in result and "version" in result:
            await self.db.union.set("union_server_list", json.dumps(result["servers"]))
            await self.db.union.set("union_server_list_version", str(result["version"]))
            logger.info(f"Updated server list (version {result['version']})")
            return True
        return False

    async def fetch_private_key(self) -> bool:
        """Fetch shared private key from Union and store locally."""
        result = await self._api_get("privatekey", raw=False)
        if result and "privateKey" in result and "privateKeyVersion" in result:
            await self.db.union.set("ygg_private_key", result["privateKey"])
            await self.db.union.set("union_private_key_version", str(result["privateKeyVersion"]))
            logger.info(f"Updated private key (version {result['privateKeyVersion']})")
            return True
        return False

    async def sync_profiles(self) -> bool:
        """Push all local player profiles to Union for sync."""
        profiles = await self.db.union.get_all_profiles_sync_data()
        result = await self._api_post("sync", {"profileList": profiles})
        if result is not None:
            logger.info(f"Synced {len(profiles)} profiles to Union")
            return True
        return False

    async def sync_profile_add(self, name: str, uuid: str):
        """Notify Union when a profile is created."""
        result = await self._api_post("profile", {"id": uuid, "name": name})
        if result is not None:
            logger.info(f"Synced profile add: {name} ({uuid})")
        else:
            logger.warning(f"Failed to sync profile add: {name} ({uuid})")

    async def sync_profile_update(self, uuid: str, name: str):
        """Notify Union when a profile is renamed."""
        result = await self._api_put(f"profile/{uuid}", {"name": name})
        if result is not None:
            logger.info(f"Synced profile rename: {uuid} -> {name}")
        else:
            logger.warning(f"Failed to sync profile rename: {uuid} -> {name}")

    async def sync_profile_delete(self, uuid: str):
        """Notify Union when a profile is deleted."""
        result = await self._api_delete(f"profile/{uuid}")
        if result is not None:
            logger.info(f"Synced profile delete: {uuid}")
        else:
            logger.warning(f"Failed to sync profile delete: {uuid}")

    # ========== Profile Binding methods ==========

    async def request_bind_token(self, uuid: str) -> dict | None:
        """Request a bind token from Union for the given profile UUID."""
        return await self._api_post("profile/bind", {"uuid": uuid})

    async def request_unbind(self, uuid: str) -> bool:
        """Request unbind of a profile from Union."""
        result = await self._api_post("profile/unbind", {"uuid": uuid})
        return result is not None

    async def request_bind_to(self, uuid: str, token: str) -> bool:
        """Bind this profile to another using a token."""
        result = await self._api_post("profile/bindto", {"uuid": uuid, "token": token})
        return result is not None

    async def request_remap_uuid(self, me: str, target: str) -> bool:
        """Request UUID remapping across the federation."""
        result = await self._api_post("profile/remapuuid", {"me": me, "target": target})
        return result is not None

    async def get_profile_detail(self, uuid: str) -> dict | None:
        """Get profile detail from Union."""
        return await self._api_get(f"profile/detail/{uuid}")

    async def get_profile_unmapped_byname(self, name: str) -> dict | None:
        """Get unmapped profile by name from Union (duplicate check)."""
        return await self._api_get(f"profile/unmapped/byname/{name}")

    # ========== Security Level ==========

    async def get_security_level(self) -> int | None:
        """Get this server's security level from Union (returns raw integer)."""
        api_root = await self.db.union.get("union_api_root", "")
        member_key = await self.db.union.get("union_member_key", "")
        if not api_root:
            return None

        # Step 1: get code
        code_url = api_root.rstrip("/") + "/code"
        try:
            async with aiohttp.ClientSession() as session:
                async with session.post(code_url, json={"token": member_key}, timeout=5) as resp:
                    if not resp.ok:
                        logger.warning(f"Union /code failed: status={resp.status}, body={(await resp.text())[:200]}")
                        return None
                    text = await resp.text()
                    try:
                        code_data = json.loads(text)
                    except json.JSONDecodeError:
                        logger.error(f"Union /code 200: non-JSON: {text[:300]}")
                        return None
                    code = code_data.get("code")
                    if not code:
                        return None

                # Step 2: get security level
                level_url = api_root.rstrip("/") + f"/backend/{code}/security/level"
                async with session.get(level_url, timeout=5) as resp:
                    if resp.ok:
                        text = await resp.text()
                        try:
                            return json.loads(text)
                        except json.JSONDecodeError:
                            logger.error(f"Union /security/level 200: non-JSON: {text[:300]}")
                            return None
        except Exception as e:
            logger.error(f"Failed to get security level: {e}")
        return None

    # ========== Blacklist methods ==========

    async def get_blacklist(self, params: dict = None) -> dict | None:
        """Query blacklist entries from Union."""
        return await self._api_get("blacklist/query", params=params)

    async def create_blacklist(self, data: dict) -> dict | None:
        """Create a new blacklist entry on Union."""
        return await self._api_post("blacklist/restful", data)

    async def invalidate_blacklist(self, entry_id) -> bool:
        """Invalidate/unban a blacklist entry."""
        result = await self._api_put(f"blacklist/invalidate/{entry_id}")
        return result is not None

    async def delete_blacklist(self, entry_id) -> bool:
        """Permanently delete a blacklist entry."""
        result = await self._api_delete(f"blacklist/restful/{entry_id}")
        return result is not None

    # ========== Diagnostics ==========

    async def trigger_diagnose(self) -> dict:
        """Trigger a full connectivity diagnostic test (matching reference project format)."""
        api_root = await self.db.union.get("union_api_root", "")
        if not api_root:
            return {"status": "error", "data": {"exception": "union_api_root 未配置"}}

        member_key = await self.db.union.get("union_member_key", "")
        headers = {"X-Union-Member-Key": member_key}
        url = api_root.rstrip("/") + "/diagnose"

        try:
            async with aiohttp.ClientSession() as session:
                start = time.time()
                async with session.post(url, headers=headers, timeout=10) as resp:
                    delay = time.time() - start
                    text = await resp.text()
                    try:
                        data = json.loads(text)
                    except json.JSONDecodeError:
                        data = None

                    if resp.ok and data:
                        return {"status": "ok", "data": data}
                    return {
                        "status": "error",
                        "data": {
                            "status_code": resp.status,
                            "headers": dict(resp.headers),
                            "body": text[:1000],
                        },
                    }
        except asyncio.TimeoutError:
            return {"status": "error", "data": {"exception": "连接超时（10秒）"}}
        except Exception as e:
            return {"status": "error", "data": {"exception": str(e)}}

    # ========== Union Public Key (cached) ==========

    async def get_union_public_key(self) -> str | None:
        """Fetch Union's signature verification public key."""
        # Check cache (60s TTL)
        now = time.time()
        if self._union_public_key_cache and (now - self._union_public_key_cache[1]) < 60:
            return self._union_public_key_cache[0]

        api_root = await self.db.union.get("union_api_root", "")
        if not api_root:
            return None

        try:
            async with aiohttp.ClientSession() as session:
                async with session.get(api_root, timeout=5) as resp:
                    if resp.ok:
                        text = await resp.text()
                        try:
                            data = json.loads(text)
                        except json.JSONDecodeError:
                            logger.error(f"Union public key 200: non-JSON: {text[:300]}")
                            return None
                        pub_key = data.get("union_host_signature_public_key")
                        if pub_key:
                            self._union_public_key_cache = (pub_key, now)
                            return pub_key
        except Exception as e:
            logger.error(f"Failed to fetch Union public key: {e}")
            return None

    async def get_union_oauth2_public_key(self) -> str | None:
        """Fetch Union's OAuth2 public key."""
        try:
            async with aiohttp.ClientSession() as session:
                api_root = await self.db.union.get("union_api_root", "")
                if not api_root:
                    return None
                async with session.get(api_root.rstrip("/") + "/oauth2/backend", timeout=5) as resp:
                    if resp.ok:
                        text = await resp.text()
                        try:
                            data = json.loads(text)
                        except json.JSONDecodeError:
                            logger.error(f"Union OAuth2 public key 200: non-JSON: {text[:300]}")
                            return None
                        return data.get("publicKey")
        except Exception as e:
            logger.error(f"Failed to fetch Union OAuth2 public key: {e}")
        return None

    # ========== Signature Verification (UnionHostVerify) ==========

    def verify_union_signature(
        self, body: str, signature_b64: str, timestamp_str: str, nonce: str, union_public_key_pem: str
    ) -> bool:
        """Verify RSA-SHA256 signature made by Union server.

        The signed data is: request_body + timestamp + nonce
        """
        try:
            public_key = serialization.load_pem_public_key(
                union_public_key_pem.encode("utf-8"),
                backend=default_backend(),
            )
            signature = base64.b64decode(signature_b64)
            signed_data = (body + timestamp_str + nonce).encode("utf-8")

            public_key.verify(
                signature,
                signed_data,
                padding.PKCS1v15(),
                hashes.SHA256(),
            )
            return True
        except Exception as e:
            logger.warning(f"Union signature verification failed: {e}")
            return False

    # ========== OAuth2 Token Building ==========

    def _rsa_encrypt_chunked(self, data: bytes, public_key_pem: str) -> bytes | None:
        """Encrypt data with RSA public key using chunked PKCS1 padding.

        Each chunk size = key_size // 8 - 11 (PKCS1 overhead).
        """
        try:
            public_key = serialization.load_pem_public_key(
                public_key_pem.encode("utf-8"), backend=default_backend()
            )
            key_size = public_key.key_size
            chunk_size = key_size // 8 - 11

            result = bytearray()
            for i in range(0, len(data), chunk_size):
                chunk = data[i : i + chunk_size]
                encrypted = public_key.encrypt(chunk, padding.PKCS1v15())
                result.extend(encrypted)
            return bytes(result)
        except Exception as e:
            logger.error(f"RSA encrypt failed: {e}")
            return None

    async def build_oauth2_token(self, user: User) -> str | None:
        """Build an encrypted OAuth2 user info token for Union OAuth2 grant flow.

        Token structure:
        1. user_info = base64(JSON: {uid, nickname, email, expires_at})
        2. mac = HMAC-SHA256(user_info, member_key)
        3. signature = RSA-SHA256(user_info + "." + mac) with oauth2_sig_private_key
        4. inner_token = JSON({userInfo, mac, signature})
        5. final = RSA-encrypt(inner_token) with Union's OAuth2 public key
        """
        # Get signing keys
        sig_private_pem = await self.db.union.get("union_oauth2_sig_private_key", "")
        sig_public_pem = await self.db.union.get("union_oauth2_sig_public_key", "")
        member_key = await self.db.union.get("union_member_key", "")

        if not sig_private_pem or not sig_public_pem:
            logger.error("OAuth2 signature keys not configured")
            return None

        # Build user info
        user_info = {
            "uid": user.id,
            "nickname": user.display_name,
            "email": user.email,
            "expires_at": int(time.time()) + 600,  # 10 min TTL
        }
        user_info_b64 = base64.b64encode(json.dumps(user_info).encode("utf-8")).decode("utf-8")

        # Compute HMAC
        mac = hmac.new(
            member_key.encode("utf-8"),
            user_info_b64.encode("utf-8"),
            hashlib.sha256,
        ).hexdigest()

        # Sign with server's private key
        try:
            sig_private = serialization.load_pem_private_key(
                sig_private_pem.encode("utf-8"), password=None, backend=default_backend()
            )
            signature = sig_private.sign(
                f"{user_info_b64}.{mac}".encode("utf-8"),
                padding.PKCS1v15(),
                hashes.SHA256(),
            )
            signature_b64 = base64.b64encode(signature).decode("utf-8")
        except Exception as e:
            logger.error(f"OAuth2 signing failed: {e}")
            return None

        # Build inner token
        inner_token = json.dumps({
            "userInfo": user_info_b64,
            "mac": mac,
            "signature": signature_b64,
        })

        # Encrypt with Union's OAuth2 public key
        union_pub_key = await self.get_union_oauth2_public_key()
        if not union_pub_key:
            logger.error("Could not fetch Union OAuth2 public key")
            return None

        encrypted = self._rsa_encrypt_chunked(inner_token.encode("utf-8"), union_pub_key)
        if encrypted is None:
            return None

        return base64.b64encode(encrypted).decode("utf-8")

    # ========== RSA Key Generation ==========

    @staticmethod
    def generate_rsa_keypair(key_size: int = 4096) -> dict[str, str]:
        """Generate an RSA key pair and return PEM strings."""
        private_key = rsa.generate_private_key(
            public_exponent=65537,
            key_size=key_size,
            backend=default_backend(),
        )
        private_pem = private_key.private_bytes(
            encoding=serialization.Encoding.PEM,
            format=serialization.PrivateFormat.PKCS8,
            encryption_algorithm=serialization.NoEncryption(),
        ).decode("utf-8")

        public_key = private_key.public_key()
        public_pem = public_key.public_bytes(
            encoding=serialization.Encoding.PEM,
            format=serialization.PublicFormat.SubjectPublicKeyInfo,
        ).decode("utf-8")

        return {"private": private_pem, "public": public_pem}

    # ========== Auto-initialization ==========

    async def ensure_oauth2_keys(self):
        """Auto-generate OAuth2 signing keys if not set (zero-config init)."""
        private = await self.db.union.get("union_oauth2_sig_private_key", "")
        public = await self.db.union.get("union_oauth2_sig_public_key", "")
        if not private or not public:
            keypair = self.generate_rsa_keypair()
            await self.db.union.set("union_oauth2_sig_private_key", keypair["private"])
            await self.db.union.set("union_oauth2_sig_public_key", keypair["public"])
            logger.info("Auto-generated OAuth2 signing key pair")

    # ========== Restore API ==========

    async def sign_profile_properties(self, profile: dict) -> dict:
        """Sign profile properties with Yggdrasil private key for multi-backend restore."""
        private_key_pem = await self.db.union.get("ygg_private_key", "")
        if not private_key_pem:
            return profile

        try:
            private_key = serialization.load_pem_private_key(
                private_key_pem.encode("utf-8"), password=None, backend=default_backend()
            )

            for prop in profile.get("properties", []):
                value = prop.get("value", "")
                signature = private_key.sign(
                    value.encode("utf-8"),
                    padding.PKCS1v15(),
                    hashes.SHA256(),
                )
                prop["signature"] = base64.b64encode(signature).decode("utf-8")

            return profile
        except Exception as e:
            logger.error(f"Failed to sign profile properties: {e}")
            return profile

    @staticmethod
    def load_pem_public_key(key_pem: str) -> RSAPublicKey | None:
        """Load a PEM-encoded RSA public key."""
        try:
            return serialization.load_pem_public_key(
                key_pem.encode("utf-8"), backend=default_backend()
            )
        except Exception:
            return None

    @staticmethod
    def load_pem_private_key(key_pem: str) -> RSAPrivateKey | None:
        """Load a PEM-encoded RSA private key."""
        try:
            return serialization.load_pem_private_key(
                key_pem.encode("utf-8"), password=None, backend=default_backend()
            )
        except Exception:
            return None
