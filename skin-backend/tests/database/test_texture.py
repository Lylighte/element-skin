import pytest
import os
from io import BytesIO
from PIL import Image
from utils.uuid_utils import generate_random_uuid
from utils.pagination import CursorEncoder

def create_test_image(width=64, height=64):
    """创建一个测试用的 PNG 字节流"""
    file = BytesIO()
    image = Image.new('RGBA', size=(width, height), color=(255, 0, 0, 255))
    image.save(file, 'png')
    file.name = 'test.png'
    file.seek(0)
    return file.read()

@pytest.mark.asyncio
async def test_texture_upload_and_library(db_session, user_factory):
    """测试材质上传及皮肤库接口"""
    user = await user_factory()
    image_bytes = create_test_image(64, 64) # 标准 64x64 皮肤
    
    # 1. Upload
    tex_hash, tex_type = await db_session.texture.upload(
        user.id, image_bytes, "skin", note="MySkin", is_public=True, model="default"
    )
    assert tex_hash is not None
    assert tex_type == "skin"
    
    # 验证文件是否保存
    assert os.path.exists(os.path.join(db_session.texture.textures_dir, f"{tex_hash}.png"))
    
    # 2. Get for user
    user_textures_page = await db_session.texture.get_for_user_cursor(user.id, limit=10)
    assert len(user_textures_page["items"]) == 1
    assert user_textures_page["items"][0]["hash"] == tex_hash
    
    count = await db_session.texture.count_for_user(user.id)
    assert count == 1
    
    # 3. Get texture info
    info = await db_session.texture.get_texture_info(user.id, tex_hash, "skin")
    assert info["note"] == "MySkin"
    assert info["is_public"] == 1
    
    # 4. Verify ownership
    assert await db_session.texture.verify_ownership(user.id, tex_hash, "skin") is True
    
    # 5. Library actions
    lib_page = await db_session.texture.get_from_library_cursor(only_public=True)
    assert len(lib_page["items"]) == 1
    assert lib_page["items"][0]["hash"] == tex_hash
    
    count = await db_session.texture.count_library(only_public=True)
    assert count == 1
    
    # 6. Update actions
    await db_session.texture.update_note(user.id, tex_hash, "skin", "NewNote")
    await db_session.texture.update_model(user.id, tex_hash, "skin", "slim")
    await db_session.texture.update_is_public(user.id, tex_hash, "skin", False)
    
    updated_info = await db_session.texture.get_texture_info(user.id, tex_hash, "skin")
    assert updated_info["note"] == "NewNote"
    assert updated_info["model"] == "slim"
    assert updated_info["is_public"] == 0
    
    # 7. Add to wardrobe (from library)
    user2 = await user_factory()
    success = await db_session.texture.add_to_user_wardrobe(user2.id, tex_hash)
    assert success is True
    user2_textures_page = await db_session.texture.get_for_user_cursor(user2.id)
    assert len(user2_textures_page["items"]) == 1
    assert user2_textures_page["items"][0]["is_public"] == 2 # 状态 2 表示非上传者
    
    # 8. Delete
    await db_session.texture.delete_from_library(user.id, tex_hash, "skin")
    assert len((await db_session.texture.get_for_user_cursor(user.id))["items"]) == 0

@pytest.mark.asyncio
async def test_texture_model_cascade_update(db_session, user_factory):
    """测试更新皮肤模型时，自动同步更新所有使用该皮肤的角色的模型"""
    user = await user_factory()
    image_bytes = create_test_image(64, 64)
    
    # 1. 上传皮肤
    tex_hash, _ = await db_session.texture.upload(user.id, image_bytes, "skin", model="default")
    
    # 2. 创建角色并应用该皮肤
    from utils.typing import PlayerProfile
    pid = generate_random_uuid()
    await db_session.user.create_profile(PlayerProfile(pid, user.id, "ModelTester", "default", None, None))
    await db_session.user.update_profile_skin(pid, tex_hash)
    
    # 3. 更新材质模型为 slim
    await db_session.texture.update_model(user.id, tex_hash, "skin", "slim")
    
    # 4. 验证级联更新：角色的 texture_model 应该也变成了 slim
    profile = await db_session.user.get_profile_by_id(pid)
    assert profile.texture_model == "slim"
    
    # 5. 验证非上传者更新 (不应报错，但也不会影响全局库)
    user2 = await user_factory()
    await db_session.texture.add_to_user_wardrobe(user2.id, tex_hash)
    await db_session.texture.update_model(user2.id, tex_hash, "skin", "default")
    
    # 角色1的模型不应被 user2 的操作改变 (因为 user2 不是上传者)
    assert (await db_session.user.get_profile_by_id(pid)).texture_model == "slim"

@pytest.mark.asyncio
async def test_texture_edge_cases(db_session, user_factory):
    """测试材质模块的边界情况"""
    user = await user_factory()
    
    # 删除不存在的材质
    res = await db_session.texture.delete_from_library(user.id, "non-existent", "skin")
    assert res is False
    
    # 验证不存在材质的所有权
    assert await db_session.texture.verify_ownership(user.id, "none", "skin") is False
    
    # 获取不存在的材质信息
    assert await db_session.texture.get_texture_info(user.id, "none", "skin") is None

@pytest.mark.asyncio
async def test_texture_uploader_deletion_and_readd(db_session, user_factory):
    """测试上传者删除材质同步删除库记录，以及从库中恢复材质的逻辑"""
    user = await user_factory()
    image_bytes = create_test_image(64, 64)
    
    # 1. 上传材质
    tex_hash, _ = await db_session.texture.upload(
        user.id, image_bytes, "skin", note="PublicSkin", is_public=True
    )
    
    # 验证库中存在
    assert await db_session.texture.count_library(only_public=True) == 1
    
    # 2. 上传者删除材质
    await db_session.texture.delete_from_library(user.id, tex_hash, "skin")
    
    # 验证库中已删除 (修复验证)
    assert await db_session.texture.count_library(only_public=True) == 0
    
    # 3. 模拟遗留数据 (材质在库中，但不在用户衣柜中)
    # 手动插入到 skin_library
    created_at = 1234567890
    async with db_session.get_conn() as conn:
        await conn.execute(
            "INSERT INTO skin_library (skin_hash, texture_type, is_public, uploader, model, name, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
            tex_hash, "skin", 1, user.id, "default", "LegacySkin", created_at
        )
    
    # 4. 上传者重新添加 (验证兼容性修复：is_public=1)
    await db_session.texture.add_to_user_wardrobe(user.id, tex_hash)
    
    user_tex = await db_session.texture.get_texture_info(user.id, tex_hash, "skin")
    assert user_tex["is_public"] == 1
    
    # 5. 其他用户添加 (验证正常逻辑：is_public=2)
    user2 = await user_factory()
    await db_session.texture.add_to_user_wardrobe(user2.id, tex_hash)
    
    user2_tex = await db_session.texture.get_texture_info(user2.id, tex_hash, "skin")
    assert user2_tex["is_public"] == 2


@pytest.mark.asyncio
async def test_list_all_textures_cursor(db_session, user_factory):
    """测试管理端：游标分页列出所有材质（公共+私有），支持类型过滤和搜索"""
    user1 = await user_factory()
    user2 = await user_factory()

    # Create images with different colors so each gets a unique hash
    def _make_image(color):
        f = BytesIO()
        Image.new('RGBA', (64, 64), color).save(f, 'png')
        f.name = 'test.png'
        f.seek(0)
        return f.read()

    img1 = _make_image((255, 0, 0, 255))  # red
    img2 = _make_image((0, 255, 0, 255))  # green
    img3 = _make_image((0, 0, 255, 255))  # blue

    # Upload textures — must be public to appear in skin_library
    tex1_hash, _ = await db_session.texture.upload(
        user1.id, img1, "skin", note="Skin1", is_public=True, model="default"
    )
    tex2_hash, _ = await db_session.texture.upload(
        user1.id, img2, "cape", note="Cape1", is_public=True, model="default"
    )
    tex3_hash, _ = await db_session.texture.upload(
        user2.id, img3, "skin", note="Skin2", is_public=True, model="slim"
    )

    # 1. List all with ample limit
    page = await db_session.texture.list_all_textures_cursor(limit=10)
    assert len(page["items"]) == 3
    assert page["has_next"] is False
    assert page["next_cursor"] is None

    # 2. Type filter: skin
    skin_page = await db_session.texture.list_all_textures_cursor(limit=10, type_filter="skin")
    assert len(skin_page["items"]) == 2
    for item in skin_page["items"]:
        assert item["type"] == "skin"

    # 3. Type filter: cape
    cape_page = await db_session.texture.list_all_textures_cursor(limit=10, type_filter="cape")
    assert len(cape_page["items"]) == 1
    assert cape_page["items"][0]["type"] == "cape"
    assert cape_page["items"][0]["hash"] == tex2_hash

    # 4. Search by hash substring
    query_substring = tex1_hash[:8]
    search_page = await db_session.texture.list_all_textures_cursor(limit=10, query=query_substring)
    assert len(search_page["items"]) == 1
    assert search_page["items"][0]["hash"] == tex1_hash


@pytest.mark.asyncio
async def test_update_texture_public_admin(db_session, user_factory):
    """测试管理端：修改皮肤库材质的公开状态"""
    user = await user_factory()
    image_bytes = create_test_image(64, 64)

    # 1. Upload a public texture
    tex_hash, _ = await db_session.texture.upload(
        user.id, image_bytes, "skin", note="AdminTarget", is_public=True, model="default"
    )

    # 2. Set is_public=0
    result = await db_session.texture.update_texture_public_admin(tex_hash, is_public=0)
    assert result is True

    # Verify skin_library updated
    async with db_session.get_conn() as conn:
        lib_is_public = await conn.fetchval(
            "SELECT is_public FROM skin_library WHERE skin_hash = $1", tex_hash
        )
        assert lib_is_public == 0

    # Verify user_textures updated
    info = await db_session.texture.get_texture_info(user.id, tex_hash, "skin")
    assert info["is_public"] == 0

    # 3. Set is_public back to 1
    result = await db_session.texture.update_texture_public_admin(tex_hash, is_public=1)
    assert result is True
    async with db_session.get_conn() as conn:
        lib_is_public = await conn.fetchval(
            "SELECT is_public FROM skin_library WHERE skin_hash = $1", tex_hash
        )
        assert lib_is_public == 1

    # 4. Non-existent hash → False
    result = await db_session.texture.update_texture_public_admin("badhash", is_public=1)
    assert result is False

    # 5. Collected texture (is_public=2) — should NOT be affected
    user2 = await user_factory()
    await db_session.texture.add_to_user_wardrobe(user2.id, tex_hash)
    user2_info = await db_session.texture.get_texture_info(user2.id, tex_hash, "skin")
    assert user2_info["is_public"] == 2

    # Admin toggle to 0 — user2's is_public should stay 2 (guarded)
    await db_session.texture.update_texture_public_admin(tex_hash, is_public=0)
    user2_info = await db_session.texture.get_texture_info(user2.id, tex_hash, "skin")
    assert user2_info["is_public"] == 2


@pytest.mark.asyncio
async def test_delete_texture_admin(db_session, user_factory):
    """测试管理端：删除材质（按用户/强制全部）"""
    user1 = await user_factory()
    user2 = await user_factory()
    user3 = await user_factory()
    image_bytes = create_test_image(64, 64)

    # Upload same hash to user1 and user2
    tex_hash, tex_type = await db_session.texture.upload(
        user1.id, image_bytes, "skin", note="SharedSkin", is_public=True, model="default"
    )
    # Upload same image bytes as user2 → same hash, different user
    # The file already exists on disk, add_to_library will just add DB entries
    await db_session.texture.add_to_library(
        user2.id, tex_hash, tex_type, note="SharedSkin2", is_public=True, model="slim"
    )

    # Verify both users have it
    assert await db_session.texture.verify_ownership(user1.id, tex_hash, tex_type) is True
    assert await db_session.texture.verify_ownership(user2.id, tex_hash, tex_type) is True

    # 1. Per-user deletion: delete from user1
    result = await db_session.texture.delete_texture_admin(tex_hash, tex_type, user_id=user1.id)
    assert result is True
    assert await db_session.texture.verify_ownership(user1.id, tex_hash, tex_type) is False
    assert await db_session.texture.verify_ownership(user2.id, tex_hash, tex_type) is True

    # 2. Force deletion: upload as user3, then force-delete all
    tex2_hash, tex2_type = await db_session.texture.upload(
        user3.id, image_bytes, "skin", note="ForceTarget", is_public=True, model="default"
    )
    result = await db_session.texture.delete_texture_admin(
        tex2_hash, tex2_type, user_id=None, force=True
    )
    assert result is True
    assert await db_session.texture.verify_ownership(user3.id, tex2_hash, tex2_type) is False

    # Verify skin_library entry also gone
    async with db_session.get_conn() as conn:
        lib_val = await conn.fetchval(
            "SELECT 1 FROM skin_library WHERE skin_hash = $1", tex2_hash
        )
        assert lib_val is None

    # 3. Non-existent hash → False
    result = await db_session.texture.delete_texture_admin("badhash", "skin", user_id=user1.id)
    assert result is False

    # 4. Invalid: force=False, user_id=None → False
    result = await db_session.texture.delete_texture_admin("somehash", "skin", user_id=None, force=False)
    assert result is False
