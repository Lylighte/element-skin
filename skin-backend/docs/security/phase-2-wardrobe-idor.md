# 阶段 2：衣柜越权（IDOR）——校验材质可见性

## 目标

修复 `add_to_user_wardrobe` 不校验材质可见性的越权读取：任何登录用户只要知道 `skin_hash`，就能把**他人的私有材质**（`is_public=0`）拷入自己衣柜，进而查看/应用。

## 问题证据

- `database_module/modules/texture.py:339-365` `add_to_user_wardrobe`：仅按 `WHERE skin_hash=$1` 查 `skin_library`，**不检查 `is_public`**，随后以 `is_public=2`（收藏）插入调用者的 `user_textures`。
- `skin_library` 确实存有私有行：`add_to_library`（`texture.py:10-26`）以 `is_public_val = 1 if is_public else 0` 写入，私有上传即 `is_public=0` 也进库。
- 入库后调用者可经 `/me/textures/{hash}/{type}` 查看并 `apply_texture_to_profile` 应用 → 私有材质泄露。
- 调用链：`routers/site_routes.py:257` → `backends/site_backend.py:139`（`add_to_wardrobe`）→ `texture.py:339`。

## 设计决策

衣柜「添加」的合法语义是：**从公共库收藏他人公开材质**，或**找回自己上传的材质**。因此放行条件为：

- `is_public = 1`（公开），任何人可收藏；**或**
- `uploader = user_id`（自己的材质，无论公私），可找回。

其余（他人的 `is_public=0`）一律拒绝，返回 `False`（路由层转 404，不泄露存在性）。

## 改造清单

`database_module/modules/texture.py:add_to_user_wardrobe`，在取到 `row` 后增加可见性判定：

```python
async def add_to_user_wardrobe(self, user_id: str, texture_hash: str) -> bool:
    async with self.db.get_conn() as conn:
        async with conn.transaction():
            row = await conn.fetchrow(
                "SELECT texture_type, model, uploader, name, is_public "
                "FROM skin_library WHERE skin_hash = $1",
                texture_hash,
            )
            if not row:
                return False

            texture_type, model, uploader, name, src_is_public = (
                row[0], row[1], row[2], row[3] or "", row[4],
            )

            # 仅允许：公开材质（任何人可收藏）或自己上传的材质（可找回）。
            # 拒绝他人的私有材质，避免越权读取。
            if src_is_public != 1 and uploader != user_id:
                return False

            created_at = int(time.time() * 1000)
            is_public = 1 if uploader == user_id else 2

            await conn.execute(
                "INSERT INTO user_textures (user_id, hash, texture_type, note, model, is_public, created_at) "
                "VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT DO NOTHING",
                user_id, texture_hash, texture_type, name, model, is_public, created_at,
            )
            return True
```

> 路由/后端层确认：`add_to_wardrobe` 返回 `False` 时应转 `404`（或 `403`），与「材质不存在」一致，避免通过响应差异探测私有材质是否存在。检查 `site_backend.py:139` 现有处理，如直接返回布尔需在 `site_routes.py:257` 转成 HTTP 404。

## 影响文件

- `database_module/modules/texture.py`（主）
- 必要时 `routers/site_routes.py` 或 `backends/site_backend.py`（仅在现状未把 `False` 转 404 时）

## 验证

```bash
cd skin-backend
pytest tests/database/test_texture.py -q
pytest tests/api -q
```

新增针对性用例：

- 用户 A 上传**私有**材质 → 用户 B `add_to_user_wardrobe` 返回 `False`，B 的 `user_textures` 无该行。
- 用户 A 上传**公开**材质 → 用户 B 可收藏，B 的行 `is_public=2`。
- 用户 A 找回自己的私有材质 → 成功，行 `is_public=1`。
- API 层：B 对 A 的私有 hash 调用收藏端点返回 404，且 `/me/textures/{hash}/{type}` 仍 404。
