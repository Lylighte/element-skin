from ..core import BaseDB
import aiosqlite
import time
from typing import Optional, Tuple
import os
from PIL import Image
from io import BytesIO

# Import from utils, this assumes correct python path
from utils.image_utils import (
    validate_texture_dimensions,
    compute_texture_hash_from_image,
    normalize_png,
)
from config_loader import config

class TextureModule:
    def __init__(self, db: BaseDB):
        self.db = db
        self.textures_dir = config.get("textures.directory", "textures")
        os.makedirs(self.textures_dir, exist_ok=True)

    async def upload(
        self, user_id: str, file_bytes: bytes, texture_type: str, note: str = "", is_public: bool = False, model: str = "default"
    ) -> Tuple[str, str]:
        """
        验证、保存并记录材质
        """
        # 规范化图像
        normalized_bytes, img = normalize_png(file_bytes)

        # 验证尺寸
        is_cape = texture_type.lower() == "cape"
        if not validate_texture_dimensions(img, is_cape):
            raise ValueError("Invalid texture dimensions")

        # 计算哈希
        texture_hash = compute_texture_hash_from_image(img)

        # 保存文件
        file_path = os.path.join(self.textures_dir, f"{texture_hash}.png")
        with open(file_path, "wb") as f:
            f.write(normalized_bytes)

        await self.add_to_library(user_id, texture_hash, texture_type, note, is_public, model)

        return texture_hash, texture_type

    async def add_to_library(self, user_id: str, texture_hash: str, texture_type: str, note: str = "", is_public: bool = False, model: str = "default") -> bool:
        async with self.db.get_conn() as conn:
            created_at = int(time.time() * 1000)
            is_public_val = 1 if is_public else 0
            try:
                # 记录用户材质
                await conn.execute(
                    "INSERT OR IGNORE INTO user_textures (user_id, hash, texture_type, note, model, is_public, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
                    (user_id, texture_hash, texture_type, note, model, is_public_val, created_at),
                )
                
                # 记录到全局皮肤库
                await conn.execute(
                    "INSERT OR IGNORE INTO skin_library (skin_hash, texture_type, is_public, uploader, model, name, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
                    (texture_hash, texture_type, is_public_val, user_id, model, note, created_at),
                )
                
                await conn.commit()
                return True
            except aiosqlite.IntegrityError:
                return False

    async def delete_from_library(self, user_id: str, texture_hash: str, texture_type: str) -> bool:
        async with self.db.get_conn() as conn:
            cur = await conn.execute(
                "SELECT 1 FROM user_textures WHERE user_id=? AND hash=? AND texture_type=?",
                (user_id, texture_hash, texture_type),
            )
            if not await cur.fetchone():
                return False

            await conn.execute(
                "DELETE FROM user_textures WHERE user_id=? AND hash=? AND texture_type=?",
                (user_id, texture_hash, texture_type),
            )
            await conn.commit()
            return True

    async def get_for_user(self, user_id: str, texture_type: Optional[str] = None) -> list[tuple]:
        async with self.db.get_conn() as conn:
            if texture_type:
                query = "SELECT hash, texture_type, note, created_at, model, is_public FROM user_textures WHERE user_id=? AND texture_type=? ORDER BY created_at DESC"
                params = (user_id, texture_type)
            else:
                query = "SELECT hash, texture_type, note, created_at, model, is_public FROM user_textures WHERE user_id=? ORDER BY created_at DESC"
                params = (user_id,)
            
            async with conn.execute(query, params) as cur:
                rows = await cur.fetchall()
                # hash, texture_type, note, created_at, model, is_public
                return [(r[0], r[1], r[2], r[3], r[4], r[5]) for r in rows]

    async def verify_ownership(self, user_id: str, texture_hash: str, texture_type: str) -> bool:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT 1 FROM user_textures WHERE user_id=? AND hash=? AND texture_type=?",
                (user_id, texture_hash, texture_type),
            ) as cur:
                row = await cur.fetchone()
                return row is not None

    async def get_texture_info(self, user_id: str, texture_hash: str, texture_type: str) -> Optional[dict]:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT hash, texture_type, note, model, created_at, is_public FROM user_textures WHERE user_id=? AND hash=? AND texture_type=?",
                (user_id, texture_hash, texture_type),
            ) as cur:
                row = await cur.fetchone()
                if row:
                    return {
                        "hash": row[0],
                        "type": row[1],
                        "note": row[2],
                        "model": row[3],
                        "created_at": row[4],
                        "is_public": row[5]
                    }
                return None

    async def update_note(self, user_id: str, texture_hash: str, texture_type: str, note: str):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "UPDATE user_textures SET note=? WHERE user_id=? AND hash=? AND texture_type=?",
                (note, user_id, texture_hash, texture_type),
            )
            # 同时更新皮肤库中的名称 (如果是上传者)
            await conn.execute(
                "UPDATE skin_library SET name=? WHERE skin_hash=? AND uploader=?",
                (note, texture_hash, user_id),
            )
            await conn.commit()

    async def update_model(self, user_id: str, texture_hash: str, texture_type: str, model: str):
        async with self.db.get_conn() as conn:
            # Update user's wardrobe entry
            await conn.execute(
                "UPDATE user_textures SET model=? WHERE user_id=? AND hash=? AND texture_type=?",
                (model, user_id, texture_hash, texture_type),
            )
            # Update library entry if this user is the uploader
            await conn.execute(
                "UPDATE skin_library SET model=? WHERE skin_hash=? AND uploader=?",
                (model, texture_hash, user_id),
            )
            # If it's a skin, also update all profiles using this skin to match the new model
            if texture_type.lower() == "skin":
                await conn.execute(
                    "UPDATE profiles SET texture_model=? WHERE skin_hash=? AND user_id=?",
                    (model, texture_hash, user_id),
                )
            await conn.commit()

    async def update_is_public(self, user_id: str, texture_hash: str, texture_type: str, is_public: bool):
        async with self.db.get_conn() as conn:
            is_public_val = 1 if is_public else 0
            # 只有上传者才能修改公开状态 (is_public != 2)
            await conn.execute(
                "UPDATE user_textures SET is_public=? WHERE user_id=? AND hash=? AND texture_type=? AND is_public != 2",
                (is_public_val, user_id, texture_hash, texture_type),
            )
            # 同时更新皮肤库
            await conn.execute(
                "UPDATE skin_library SET is_public=? WHERE skin_hash=? AND uploader=?",
                (is_public_val, texture_hash, user_id),
            )
            await conn.commit()

    async def get_from_library(
        self,
        limit: int = 20,
        offset: int = 0,
        texture_type: Optional[str] = None,
        only_public: bool = True,
    ) -> list[tuple]:
        """
        获取皮肤库中的材质
        """
        async with self.db.get_conn() as conn:
            query = "SELECT skin_hash, texture_type, is_public, uploader, created_at, model, name FROM skin_library"
            conditions = []
            params = []

            if only_public:
                conditions.append("is_public = 1")

            if texture_type:
                conditions.append("texture_type = ?")
                params.append(texture_type)

            if conditions:
                query += " WHERE " + " AND ".join(conditions)

            query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
            params.extend([limit, offset])

            async with conn.execute(query, params) as cur:
                rows = await cur.fetchall()
                # skin_hash, texture_type, is_public, uploader, created_at, model, name
                return [(r[0], r[1], bool(r[2]), r[3], r[4], r[5], r[6]) for r in rows]

    async def count_library(self, texture_type: Optional[str] = None, only_public: bool = True) -> int:
        async with self.db.get_conn() as conn:
            query = "SELECT COUNT(*) FROM skin_library"
            conditions = []
            params = []
            if only_public:
                conditions.append("is_public = 1")
            if texture_type:
                conditions.append("texture_type = ?")
                params.append(texture_type)
            
            if conditions:
                query += " WHERE " + " AND ".join(conditions)
            
            async with conn.execute(query, params) as cur:
                row = await cur.fetchone()
                return row[0] if row else 0

    async def add_to_user_wardrobe(self, user_id: str, texture_hash: str) -> bool:
        """
        从公共库添加材质到用户衣柜
        """
        async with self.db.get_conn() as conn:
            # 获取材质信息
            async with conn.execute(
                "SELECT texture_type, model FROM skin_library WHERE skin_hash = ?", (texture_hash,)
            ) as cur:
                row = await cur.fetchone()
                if not row:
                    return False
                texture_type = row[0]
                model = row[1]
            
            created_at = int(time.time() * 1000)
            # 非上传者添加，状态设为 2
            await conn.execute(
                "INSERT OR IGNORE INTO user_textures (user_id, hash, texture_type, model, is_public, created_at) VALUES (?, ?, ?, ?, ?, ?)",
                (user_id, texture_hash, texture_type, model, 2, created_at),
            )
            await conn.commit()
            return True
