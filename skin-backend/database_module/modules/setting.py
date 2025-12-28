from ..core import BaseDB

class SettingModule:
    def __init__(self, db: BaseDB):
        self.db = db

    async def get(self, key: str, default: str = None) -> str:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT value FROM settings WHERE key=?", (key,)
            ) as cur:
                row = await cur.fetchone()
                return row[0] if row else default

    async def set(self, key: str, value: str):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)",
                (key, value),
            )
            await conn.commit()

    async def get_all(self) -> dict:
        async with self.db.get_conn() as conn:
            async with conn.execute("SELECT key, value FROM settings") as cur:
                rows = await cur.fetchall()
                return {row[0]: row[1] for row in rows}
