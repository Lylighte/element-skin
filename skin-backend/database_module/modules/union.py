from ..core import BaseDB
import time


class UnionModule:
    def __init__(self, db: BaseDB):
        self.db = db
        self._cache = {}

    async def init(self):
        rows = await self.db.fetch("SELECT key, value FROM settings WHERE key LIKE 'union_%'")
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
        """Apply UUID remapping: updates profiles.id and cascades to tokens.profile_id.

        All updates run in a single transaction (all-or-nothing).
        Handles overlapping chains (A→B→C) via dependency-aware ordering.
        Rejects UUID collisions where new_uuid already exists and is not
        itself being remapped away.
        """
        if not remapped:
            return

        remapped = {k: v for k, v in remapped.items() if k != v}
        if not remapped:
            return

        old_uuids = set(remapped.keys())
        new_uuids = set(remapped.values())

        # ── pre-check: UUID collision ─────────────────────────────────
        for new_uuid in new_uuids:
            if new_uuid in old_uuids:
                continue  # being remapped away → safe
            exists = await self.db.fetchval(
                "SELECT 1 FROM profiles WHERE id = $1", new_uuid,
            )
            if exists:
                raise ValueError(
                    f"UUID collision: target {new_uuid} already exists in profiles"
                )

        # ── dependency graph (only edges where new is also an old) ───
        depends_on: dict[str, str] = {
            old: new for old, new in remapped.items() if new in old_uuids
        }

        # ── topological sort with cycle detection ────────────────────
        processed: set[str] = set()
        visiting: set[str] = set()
        ordered: list[str] = []

        def add_entry(old: str):
            if old in processed:
                return
            if old in visiting:
                raise ValueError(
                    f"UUID remap cycle detected involving {old}"
                )
            visiting.add(old)
            if old in depends_on:
                add_entry(depends_on[old])
            visiting.discard(old)
            processed.add(old)
            ordered.append(old)

        for old in remapped:
            add_entry(old)

        # ── single DB transaction ────────────────────────────────────
        async with self.db.get_conn() as conn:
            async with conn.transaction():
                for old_uuid in ordered:
                    new_uuid = remapped[old_uuid]
                    await conn.execute(
                        "UPDATE profiles SET id = $1 WHERE id = $2",
                        new_uuid, old_uuid,
                    )
                    await conn.execute(
                        "UPDATE tokens SET profile_id = $1 WHERE profile_id = $2",
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
