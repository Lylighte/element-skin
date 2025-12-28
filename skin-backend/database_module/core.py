import aiosqlite
import asyncio

class BaseDB:
    def __init__(self, db_path: str):
        self.db_path = db_path
        self.conn = None
        self.lock = asyncio.Lock()

    async def connect(self):
        """建立持久化连接并开启性能优化模式"""
        if self.conn is None:
            self.conn = await aiosqlite.connect(self.db_path)
            # 开启 WAL 模式
            await self.conn.execute("PRAGMA journal_mode=WAL;")
            await self.conn.commit()

    async def close(self):
        if self.conn:
            await self.conn.close()
            self.conn = None

    async def ensure_conn(self):
        if self.conn is None:
            await self.connect()

    def get_conn(self):
        """
        用于 context manager: async with db.get_conn() as conn:
        但鉴于我们持有 self.conn，这里返回一个 context manager wrapper
        或者直接暴露 conn。
        为保持兼容性或简单性，我们使用一个 helper。
        """
        # 由于我们是单例长连接，直接返回 conn 即可，但为了安全加锁
        # 这里返回一个 LockContext
        return LockContext(self)

class LockContext:
    def __init__(self, db: BaseDB):
        self.db = db

    async def __aenter__(self):
        await self.db.lock.acquire()
        await self.db.ensure_conn()
        return self.db.conn

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        self.db.lock.release()
