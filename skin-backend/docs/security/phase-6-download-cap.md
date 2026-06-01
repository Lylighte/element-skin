# 阶段 6：纹理下载内存硬上限

## 目标

主改 `utils/http.py`：让 `download_texture` 在下载阶段就施加字节上限，**先拒绝再读完**，消除「用户可控 URL 指向超大文件 → 全量读入内存 → OOM」的 DoS 面。

## 问题证据

- `utils/http.py:8-20` `download_texture`：`return await resp.read()` 无上限地把整个响应体读入内存。15s 超时只限时间、不限内存。
- 大小校验 `assert_texture_size`（`services/texture_storage.py:20-33`）在**字节已进内存之后**才执行（见 `microsoft_backend.py` 导入路径、`profile_import_backend.py:66,77`）。
- 触发面：Microsoft 导入的 `skin_url`/`cape_url` 完全由用户控制（`microsoft_routes.py:122-124`），Yggdrasil 导入的 URL 由用户指定的远端服务器控制。并发导入可叠加放大。

## 设计决策

- **上限来源**：与 `assert_texture_size` 一致，取 `db.setting` 的 `max_texture_size`（KB，默认 1024）。但 `download_texture` 当前不持有 `db`，为最小改动，**给函数加可选 `max_bytes` 参数**，由调用方把已解析的上限传入；调用方本就在调 `assert_texture_size`，顺手把同一上限传进来即可。无 `max_bytes` 时退化为一个保守硬顶（如 8MB）兜底。
- **双重防线**：先看 `Content-Length`（若提供且超限直接拒，省流量），再边读边累加、超限即中断（防伪造/缺失 `Content-Length`）。
- 保留现有 `validate_outbound_url` + `allow_redirects=False`。

## 改造清单

`utils/http.py`：

```python
import aiohttp
from utils.url_guard import validate_outbound_url

# max_bytes 缺省时的保守硬顶（与设置无关的最后防线）
_HARD_CAP_BYTES = 8 * 1024 * 1024


async def download_texture(url: str, max_bytes: int | None = None) -> bytes:
    """下载皮肤/披风纹理，带 SSRF 校验、禁重定向、内存硬上限。

    max_bytes：调用方传入的字节上限（通常 = max_texture_size 设置 × 1024）。
    缺省时退化为 _HARD_CAP_BYTES。超限在读取阶段即中断并抛 Exception。
    """
    await validate_outbound_url(url)
    cap = max_bytes if (max_bytes and max_bytes > 0) else _HARD_CAP_BYTES

    timeout = aiohttp.ClientTimeout(total=15)
    async with aiohttp.ClientSession(timeout=timeout) as session:
        async with session.get(url, allow_redirects=False) as resp:
            if resp.status != 200:
                raise Exception(f"Failed to download texture from {url}")
            # 1) Content-Length 提前拒绝
            if resp.content_length and resp.content_length > cap:
                raise Exception("texture too large")
            # 2) 流式累加，超限即断
            buf = bytearray()
            async for chunk in resp.content.iter_chunked(64 * 1024):
                buf += chunk
                if len(buf) > cap:
                    raise Exception("texture too large")
            return bytes(buf)
```

调用方（`backends/microsoft_backend.py`、`backends/profile_import_backend.py`）传入上限：

```python
# 调用方已能拿到 db；解析一次上限传进去
max_kb = int(await db.setting.get("max_texture_size", "1024"))
data = await download_texture(url, max_bytes=max_kb * 1024)
await assert_texture_size(db, data)   # 仍保留，作为落盘前的统一校验
```

> 若某调用方不便拿到 `db`，可不传 `max_bytes`，靠 `_HARD_CAP_BYTES` 兜底——仍优于现状（无上限）。

## 影响文件

- `utils/http.py`（主）
- `backends/microsoft_backend.py`、`backends/profile_import_backend.py`（传入 `max_bytes`，可选但推荐）

## 验证

```bash
cd skin-backend
pytest tests/services/test_texture_storage.py -q
pytest -q
```

针对性用例（可用本地 mock server / aioresponses 模拟）：

- 响应声明超大 `Content-Length` → 立即抛错，未读全量。
- 无 `Content-Length` 但实际体超限 → 累加到上限即中断抛错。
- 正常小图 → 正常返回字节。
- `max_bytes` 缺省时 `_HARD_CAP_BYTES` 生效。
