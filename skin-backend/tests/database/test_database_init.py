import pytest
import os
from database_module import Database

@pytest.mark.asyncio
async def test_database_init_scripts(db_session):
    """测试数据库初始化：验证表和默认设置"""
    # 由于 db_session 已经调用了 db.init()，我们直接验证结果
    
    async with db_session.get_conn() as conn:
        # PostgreSQL 使用 information_schema 验证表
        rows = await conn.fetch(
            "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'"
        )
        tables = [r['table_name'] for r in rows]
        assert "users" in tables
        assert "settings" in tables
        assert "fallback_endpoints" in tables
        
        # 验证默认设置
        val = await conn.fetchval("SELECT value FROM settings WHERE key='enable_skin_library'")
        assert val == "true"
        
        # 验证其他默认设置
        smtp_host = await conn.fetchval("SELECT value FROM settings WHERE key='smtp_host'")
        assert smtp_host == "smtp.example.com"
