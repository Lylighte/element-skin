"""HTTP 工具：纹理下载"""

import aiohttp

from utils.url_guard import validate_outbound_url


# max_bytes 缺省时的保守硬顶（与设置无关的最后防线）
_HARD_CAP_BYTES = 8 * 1024 * 1024


async def download_texture(url: str, max_bytes: int | None = None) -> bytes:
    """下载皮肤或披风纹理，带 SSRF 校验、禁重定向、内存硬上限，失败抛 Exception。

    下载前对 URL 做 SSRF 校验（仅 http/https、禁止解析到内网/保留地址），
    并禁止跟随重定向，避免「先返回公网 URL 再 302 跳内网」的绕过。

    max_bytes：调用方传入的字节上限（通常 = max_texture_size 设置 × 1024）。
    缺省时退化为 _HARD_CAP_BYTES。先看 Content-Length 提前拒绝省流量，再边读边
    累加、超限即中断，防伪造/缺失 Content-Length 的超大响应把内存读爆（DoS）。
    """
    await validate_outbound_url(url)
    cap = max_bytes if (max_bytes and max_bytes > 0) else _HARD_CAP_BYTES

    timeout = aiohttp.ClientTimeout(total=15)
    async with aiohttp.ClientSession(timeout=timeout) as session:
        async with session.get(url, allow_redirects=False) as resp:
            if resp.status != 200:
                raise Exception(f"Failed to download texture from {url}")
            # 1) Content-Length 提前拒绝（若提供且超限，直接拒，省流量）
            if resp.content_length and resp.content_length > cap:
                raise Exception("texture too large")
            # 2) 流式累加，超限即中断（防伪造/缺失 Content-Length）
            buf = bytearray()
            async for chunk in resp.content.iter_chunked(64 * 1024):
                buf += chunk
                if len(buf) > cap:
                    raise Exception("texture too large")
            return bytes(buf)
