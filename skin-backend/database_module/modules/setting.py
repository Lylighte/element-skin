from ..core import BaseDB

class SettingModule:
    def __init__(self, db: BaseDB):
        self.db = db
        self._cache = {}

    async def init(self):
        """Initialize cache from database"""
        self._cache = await self._load_all_from_db()

    async def _load_all_from_db(self) -> dict:
        async with self.db.get_conn() as conn:
            async with conn.execute("SELECT key, value FROM settings") as cur:
                rows = await cur.fetchall()
                return {row[0]: row[1] for row in rows}

    async def get(self, key: str, default: str = None) -> str:
        """Get from cache with fallback to default"""
        return self._cache.get(key, default)

    async def set(self, key: str, value: str):
        """Update both DB and cache"""
        async with self.db.get_conn() as conn:
            await conn.execute(
                "INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)",
                (key, value),
            )
            await conn.commit()
        self._cache[key] = value

    async def get_all(self) -> dict:
        """Return a copy of the cache"""
        return self._cache.copy()
