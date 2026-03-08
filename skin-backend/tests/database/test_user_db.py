import pytest
from utils.typing import User

@pytest.mark.asyncio
async def test_create_and_get_user(db_session):
    """验证用户创建和查询的基本功能"""
    user_id = "test-uid-123"
    email = "test@db.com"
    user = User(
        id=user_id,
        email=email,
        password="hashed_secret",
        display_name="DBTester"
    )
    
    # 写入
    await db_session.user.create(user)
    
    # 读取验证
    fetched = await db_session.user.get_by_email(email)
    assert fetched is not None
    assert fetched.id == user_id
    assert fetched.display_name == "DBTester"

@pytest.mark.asyncio
async def test_display_name_collision(db_session):
    """验证用户名重复检测"""
    # 先插入一个
    user1 = User("uid1", "u1@t.com", "pw", display_name="UniqueName")
    await db_session.user.create(user1)
    
    # 检测重复
    is_taken = await db_session.user.is_display_name_taken("UniqueName")
    assert is_taken is True
    
    # 检测非重复
    is_taken_2 = await db_session.user.is_display_name_taken("AnotherName")
    assert is_taken_2 is False

@pytest.mark.asyncio
async def test_user_factory_helper(db_session, user_factory):
    """验证 User Factory 是否好用"""
    # 快速创建3个用户
    for i in range(3):
        await user_factory(username=f"User_{i}")
        
    count = await db_session.user.count()
    assert count == 3
