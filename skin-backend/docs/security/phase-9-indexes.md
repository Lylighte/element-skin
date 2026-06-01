# 阶段 9：补热路径索引

## 目标

主改 `database_module/initsql.py`：为两条会随数据量退化为顺序扫描的热路径查询补索引。全部 `CREATE INDEX IF NOT EXISTS`，对旧库幂等即迁移。

## 问题证据

交叉比对查询与现有索引（`initsql.py:145-171`）：

- **`users.display_name`**：`is_display_name_taken`（`user.py:67,70`）按 `WHERE display_name = $1` 查询，处于注册/改名热路径。`display_name` 既非唯一也无索引 → 每次校验全表扫。
- **`user_textures (hash, texture_type)`**：`delete_texture` 强删路径（`texture.py:388-391`）与剩余计数查询（`texture.py:401-404`）按 `hash`/`texture_type` 过滤，**不带前导 `user_id`**。现有索引为 PK `(user_id, hash, texture_type)` 与 `(user_id, created_at, hash)`，均以 `user_id` 打头，无法服务 `hash` 前导查询 → 顺序扫。

> 较低优先（本阶段不强制）：`search_users_cursor`（`user.py:135-155`）与 `list_all_profiles_cursor` 的 `ILIKE '%q%'` 前导通配无法用 btree，属管理员低频操作；如将来增长再考虑 `pg_trgm` GIN。

## 改造清单

`database_module/initsql.py` 索引区追加：

```sql
-- users.display_name：注册/改名时的唯一性校验热路径（is_display_name_taken）
CREATE INDEX IF NOT EXISTS idx_users_display_name ON users (display_name);

-- user_textures：按 hash + 类型的强删/剩余计数（无 user_id 前导，PK 无法服务）
CREATE INDEX IF NOT EXISTS idx_user_textures_hash_type ON user_textures (hash, texture_type);
```

> 列名以 `initsql.py` 现有表定义为准（实现时核对 `user_textures` 的实际列名 `hash` / `texture_type`）。

## 影响文件

- `database_module/initsql.py`（主）

## 验证

```bash
cd skin-backend
pytest tests/database/test_database_init.py -q
pytest -q
```

针对性验证：

- 初始化幂等：对已存在索引的库重复 `init()` 不报错。
- （可选）在有数据的本地库上 `EXPLAIN` 上述两类查询，确认走 Index Scan 而非 Seq Scan。
