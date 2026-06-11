"""Integration test: Union key fetch → file → Yggdrasil signature."""
import os
import json
import base64
import pytest

from cryptography.hazmat.primitives.asymmetric import rsa, padding as asym_padding
from cryptography.hazmat.primitives import serialization, hashes
from cryptography.hazmat.backends import default_backend

from utils.crypto import CryptoUtils, compute_key_fingerprint
from utils.typing import PlayerProfile, User
from utils.password_utils import hash_password
from utils.uuid_utils import generate_random_uuid
from routes_reference import crypto, union_backend, ygg_backend, config


def _generate_test_rsa_key():
    """Generate a 2048-bit RSA key pair for use as the test Union key."""
    private_key = rsa.generate_private_key(
        public_exponent=65537,
        key_size=2048,
        backend=default_backend(),
    )
    private_pem = private_key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption(),
    ).decode("utf-8")
    public_pem = (
        private_key.public_key()
        .public_bytes(
            encoding=serialization.Encoding.PEM,
            format=serialization.PublicFormat.SubjectPublicKeyInfo,
        )
        .decode("utf-8")
    )
    return private_key, private_pem, public_pem


@pytest.mark.asyncio
async def test_union_key_integration_chain(client, db_session, admin_headers, test_config, tmp_path):
    """Proves: Union key fetched → written to file → Yggdrasil signatures use it."""
    union_priv_key, union_priv_pem, union_pub_pem = _generate_test_rsa_key()
    union_key_path = tmp_path / "union-ygg-private.pem"
    await db_session.union.set("union_api_root", "https://fake-union.test")
    await db_session.union.set("union_member_key", "test-member-key")

    original_api_get = union_backend._api_get

    async def mock_api_get(path, params=None, timeout=5, raw=False):
        if path == "privatekey":
            return {"privateKey": union_priv_pem, "privateKeyVersion": 1}
        return await original_api_get(path, params=params, timeout=timeout, raw=raw)

    union_backend._api_get = mock_api_get

    # Patch fetch_private_key to write to tmp_path instead of the production path
    original_fetch = union_backend.fetch_private_key

    async def _fetch_to_tmp():
        result = await union_backend._api_get("privatekey")
        if result and "privateKey" in result and "privateKeyVersion" in result:
            try:
                union_key_path.parent.mkdir(parents=True, exist_ok=True)
                union_key_path.write_text(result["privateKey"])
                os.chmod(str(union_key_path), 0o600)
            except OSError:
                return False
            await union_backend.db.union.set("union_private_key_version", str(result["privateKeyVersion"]))
            await union_backend.db.union.set("union_ygg_private_key", "")
            if test_config.get("keys.use_union_key", False) and union_backend.crypto:
                try:
                    union_backend.crypto.reload_from_pem(result["privateKey"])
                except ValueError:
                    return False
            return True
        return False

    union_backend.fetch_private_key = _fetch_to_tmp

    try:
        # ── 1. Fetch key → file write ──────────────────────────────────
        result = await union_backend.fetch_private_key()
        assert result is True
        assert union_key_path.exists()
        assert union_key_path.read_text() == union_priv_pem
        stored_version = await db_session.union.get("union_private_key_version")
        assert stored_version == "1"

        # ── 2. Reload crypto with Union key ────────────────────────────
        crypto.reload_from_pem(union_priv_pem)
        assert crypto.get_public_key_pem().strip() == union_pub_pem.strip()

        # ── 3. Metadata: signaturePublickey matches Union key ──────────
        resp = await client.get("/")
        assert resp.status_code == 200
        metadata = resp.json()
        assert metadata["signaturePublickey"].strip() == union_pub_pem.strip()

        # ── 4. Create user + profile with skin ─────────────────────────
        uid = generate_random_uuid()
        user = User(
            uid,
            f"uniontest_{uid[:8]}@test.local",
            hash_password("UnionTest123!"),
            False,
            "zh_CN",
            f"UnionTest_{uid[:8]}",
        )
        await db_session.user.create(user)
        pid = "union_profile_" + uid[:8]
        await db_session.user.create_profile(PlayerProfile(pid, user.id, "UnionSkinPlayer"))
        await db_session.user.update_profile_skin(pid, "union_skin_hash")

        # ── 5. Signed profile → verify signature cryptographically ─────
        profile_resp = await client.get(
            f"/sessionserver/session/minecraft/profile/{pid}",
            params={"unsigned": "false"},
        )
        assert profile_resp.status_code == 200
        profile_data = profile_resp.json()

        textures_prop = next(
            p for p in profile_data["properties"] if p["name"] == "textures"
        )
        textures_b64 = textures_prop["value"]
        signature = textures_prop["signature"]

        textures_json = json.loads(base64.b64decode(textures_b64).decode("utf-8"))
        assert "union_skin_hash.png" in textures_json["textures"]["SKIN"]["url"]

        union_priv_key.public_key().verify(
            base64.b64decode(signature),
            textures_b64.encode("utf-8"),
            asym_padding.PKCS1v15(),
            hashes.SHA1(),
        )

        # ── 6. Default key rejects the Union signature ─────────────────
        default_crypto = CryptoUtils("private.pem")
        with pytest.raises(Exception):
            default_crypto.private_key.public_key().verify(
                base64.b64decode(signature),
                textures_b64.encode("utf-8"),
                asym_padding.PKCS1v15(),
                hashes.SHA1(),
            )

        # ── 7. Union key fingerprint differs from default ──────────────
        fp = compute_key_fingerprint(union_priv_pem)
        assert fp.startswith("sha256:") and len(fp) == len("sha256:") + 64
        with open("private.pem", "r") as f:
            default_fp = compute_key_fingerprint(f.read())
        assert fp != default_fp

    finally:
        union_backend._api_get = original_api_get
        union_backend.fetch_private_key = original_fetch
        if union_key_path.exists():
            union_key_path.unlink()
