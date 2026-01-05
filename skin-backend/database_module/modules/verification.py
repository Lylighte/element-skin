from ..core import BaseDB
import time

class VerificationModule:
    def __init__(self, db: BaseDB):
        self.db = db

    async def create_code(self, email: str, code: str, type: str, ttl: int):
        created_at = int(time.time() * 1000)
        expires_at = created_at + (ttl * 1000)
        async with self.db.get_conn() as conn:
            # Upsert code
            await conn.execute(
                "INSERT OR REPLACE INTO verification_codes (email, code, type, created_at, expires_at) VALUES (?, ?, ?, ?, ?)",
                (email, code, type, created_at, expires_at),
            )
            await conn.commit()

    async def get_code(self, email: str, type: str):
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT code, expires_at FROM verification_codes WHERE email=? AND type=?",
                (email, type),
            ) as cur:
                return await cur.fetchone()

    async def delete_code(self, email: str, type: str):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "DELETE FROM verification_codes WHERE email=? AND type=?",
                (email, type),
            )
            await conn.commit()
