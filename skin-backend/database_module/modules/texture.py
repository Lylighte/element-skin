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
        self, user_id: str, file_bytes: bytes, texture_type: str, note: str = ""
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

        await self.add_to_library(user_id, texture_hash, texture_type, note)

        return texture_hash, texture_type

    async def add_to_library(self, user_id: str, texture_hash: str, texture_type: str, note: str = "") -> bool:
        async with self.db.get_conn() as conn:
            created_at = int(time.time() * 1000)
            try:
                await conn.execute(
                    "INSERT OR IGNORE INTO user_textures (user_id, hash, texture_type, note, created_at) VALUES (?, ?, ?, ?, ?)",
                    (user_id, texture_hash, texture_type, note, created_at),
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
                query = "SELECT hash, texture_type, note, created_at FROM user_textures WHERE user_id=? AND texture_type=? ORDER BY created_at DESC"
                params = (user_id, texture_type)
            else:
                query = "SELECT hash, texture_type, note, created_at FROM user_textures WHERE user_id=? ORDER BY created_at DESC"
                params = (user_id,)
            
            async with conn.execute(query, params) as cur:
                rows = await cur.fetchall()
                # hash, texture_type, note, created_at
                return [(r[0], r[1], r[2], r[3]) for r in rows]

    async def verify_ownership(self, user_id: str, texture_hash: str, texture_type: str) -> bool:
        async with self.db.get_conn() as conn:
            async with conn.execute(
                "SELECT 1 FROM user_textures WHERE user_id=? AND hash=? AND texture_type=?",
                (user_id, texture_hash, texture_type),
            ) as cur:
                row = await cur.fetchone()
                return row is not None

    async def update_note(self, user_id: str, texture_hash: str, texture_type: str, note: str):
        async with self.db.get_conn() as conn:
            await conn.execute(
                "UPDATE user_textures SET note=? WHERE user_id=? AND hash=? AND texture_type=?",
                (note, user_id, texture_hash, texture_type),
            )
            await conn.commit()