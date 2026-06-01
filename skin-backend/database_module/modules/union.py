from ..core import BaseDB
import time


class UnionModule:
    def __init__(self, db: BaseDB):
        self.db = db
        self._cache = {}

    async def init(self):
        rows = await self.db.fetch("SELECT key, value FROM settings WHERE key LIKE 'union_%' OR key = 'ygg_restore_api' OR key = 'ygg_private_key'")
        self._cache = {row[0]: row[1] for row in rows}

    async def get(self, key: str, default: str = None) -> str:
        return self._cache.get(key, default)

    async def set(self, key: str, value: str):
        await self.db.execute(
            "INSERT INTO settings (key, value) VALUES ($1, $2) ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value",
            key, value,
        )
        self._cache[key] = value

    async def get_all_settings(self) -> dict:
        return self._cache.copy()

    async def remap_uuids(self, remapped: dict[str, str]):
        """Apply UUID remapping: updates profiles.id to new UUID values."""
        for old_uuid, new_uuid in remapped.items():
            await self.db.execute(
                "UPDATE profiles SET id = $1 WHERE id = $2",
                new_uuid, old_uuid,
            )

    async def get_email_by_username(self, username: str) -> str | None:
        """Get user email by player/character name (for blacklist lookup)."""
        row = await self.db.fetchrow(
            "SELECT u.email FROM users u INNER JOIN profiles p ON p.user_id = u.id WHERE p.name = $1 LIMIT 1",
            username,
        )
        return row[0] if row else None

    async def get_all_profiles_sync_data(self) -> dict[str, str]:
        """Build {uuid: name} mapping for Union sync (matching reference format)."""
        rows = await self.db.fetch("SELECT id, name FROM profiles")
        return {row[0]: row[1] for row in rows}

    async def log_nonce(self, nonce: str, ttl_seconds: int = 60):
        now = int(time.time())
        await self.db.execute(
            "INSERT INTO union_nonces (nonce, created_at) VALUES ($1, $2) ON CONFLICT (nonce) DO NOTHING",
            nonce, now,
        )
        # Clean expired nonces periodically
        await self.db.execute(
            "DELETE FROM union_nonces WHERE created_at < $1",
            now - ttl_seconds,
        )

    async def is_nonce_used(self, nonce: str) -> bool:
        row = await self.db.fetchrow(
            "SELECT 1 FROM union_nonces WHERE nonce = $1",
            nonce,
        )
        return row is not None
