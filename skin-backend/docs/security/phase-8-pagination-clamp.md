# 阶段 8：分页 `limit` 上下界 clamp

## 目标

主改 `utils/pagination.py`（新增一个共享 clamp 工具），并在各路由/后端入口应用，消除 `limit` 无界导致的三类问题：DoS、500、分页死循环。

## 问题证据

- 路由 `limit: int` 无任何上下界，直接流入 DB 的 `LIMIT $n`：
  - `routers/admin_routes.py:84,99,144,200,250`
  - `routers/site_routes.py:211,223,267`
- `?limit=100000000` → 巨量结果集（内存/DoS）。
- `?limit=-1` → 各游标方法 `actual_limit = limit + 1 = 0`，查询返回 `[]`，但 `has_next = len(rows) > limit` 即 `0 > -1 = True`，随后 `rows[limit-1] = rows[-2]` 对空列表 `IndexError` → 500（如 `texture.py:54` 一带、`user.py:88,100,116-117`）。
- `?limit=0` → `has_next=True` + 空 items + 游标，分页死循环。

## 设计决策

- **统一 clamp**：`limit = max(1, min(limit, MAX_LIMIT))`，`MAX_LIMIT` 取 100（管理列表/公共库浏览足够）。
- **落点选择**：最稳妥是在**路由入口**用 FastAPI 依赖统一处理，所有列表端点共享，无需逐个后端方法改。提供一个 `clamp_limit(limit: int) -> int` 工具放 `utils/pagination.py`，路由层调用；或写成 `Depends` 依赖。
- 不改 DB 游标方法内部逻辑（它们在 `limit >= 1` 时已正确），只保证传入值已被 clamp。

## 改造清单

### 8.1 clamp 工具

`utils/pagination.py` 新增：

```python
DEFAULT_LIMIT = 20
MAX_LIMIT = 100

def clamp_limit(limit: int | None, default: int = DEFAULT_LIMIT) -> int:
    """把分页 limit 收敛到 [1, MAX_LIMIT]。None 取 default。"""
    if limit is None:
        return default
    try:
        limit = int(limit)
    except (TypeError, ValueError):
        return default
    return max(1, min(limit, MAX_LIMIT))
```

### 8.2 路由应用

每个列表端点在把 `limit` 传给 backend 前 clamp：

```python
from utils.pagination import clamp_limit
# ...
return await admin_backend.list_users(cursor, clamp_limit(limit), q)
```

逐个改：`admin_routes.py` 5 处、`site_routes.py` 3 处。

> 可选更优雅：定义 `def paged_limit(limit: int = 20) -> int: return clamp_limit(limit)` 作为 `Depends`，端点签名 `limit: int = Depends(paged_limit)`，省去逐处包裹。但这会改端点签名顺序，按现状逐处 `clamp_limit(limit)` 最小侵入。

## 影响文件

- `utils/pagination.py`（主：新增工具）
- `routers/admin_routes.py`、`routers/site_routes.py`（应用 clamp）

## 验证

```bash
cd skin-backend
pytest tests/database/test_cursor_pagination.py -q
pytest tests/api -q
```

针对性用例：

- `clamp_limit(-1) == 1`、`clamp_limit(0) == 1`、`clamp_limit(10_000) == 100`、`clamp_limit(None) == 20`、`clamp_limit("abc") == 20`。
- API 层：`?limit=-1`、`?limit=0`、`?limit=99999999` 均返回 200 且结果量受控、无 500、无死循环游标。
