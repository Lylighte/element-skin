"""阶段6：download_texture 内存硬上限测试。

不依赖 aioresponses：用最小 fake 替换 aiohttp.ClientSession，验证
- Content-Length 超限 → 提前拒绝；
- 缺失 Content-Length 但实际体超限 → 流式累加到上限即中断；
- 正常小图 → 原样返回；
- max_bytes 缺省 → _HARD_CAP_BYTES 兜底生效。
"""

import pytest
from unittest.mock import patch

import utils.http as http_mod
from utils.http import download_texture, _HARD_CAP_BYTES


class _FakeContent:
    def __init__(self, chunks):
        self._chunks = chunks

    async def iter_chunked(self, n):
        for c in self._chunks:
            yield c


class _FakeResponse:
    def __init__(self, status=200, content_length=None, chunks=()):
        self.status = status
        self.content_length = content_length
        self.content = _FakeContent(chunks)

    async def __aenter__(self):
        return self

    async def __aexit__(self, *exc):
        return False


class _FakeSession:
    def __init__(self, resp):
        self._resp = resp

    async def __aenter__(self):
        return self

    async def __aexit__(self, *exc):
        return False

    def get(self, url, allow_redirects=False):
        return self._resp


def _patch_session(resp):
    """把 aiohttp.ClientSession(...) 替换成返回固定 fake 会话。"""
    return patch.object(http_mod.aiohttp, "ClientSession", lambda *a, **k: _FakeSession(resp))


@pytest.fixture(autouse=True)
def _skip_ssrf():
    """跳过真实 DNS 解析；本组只测尺寸上限逻辑。"""
    async def _noop(url):
        return None
    with patch.object(http_mod, "validate_outbound_url", _noop):
        yield


@pytest.mark.asyncio
async def test_content_length_over_cap_rejected_early():
    resp = _FakeResponse(status=200, content_length=10_000, chunks=[b"x" * 100])
    with _patch_session(resp):
        with pytest.raises(Exception) as exc:
            await download_texture("http://example.com/big.png", max_bytes=1000)
    assert "too large" in str(exc.value)


@pytest.mark.asyncio
async def test_streamed_body_over_cap_aborts():
    # 不声明 Content-Length，但实际体累加超过上限
    chunks = [b"a" * 400, b"b" * 400, b"c" * 400]  # 共 1200 > 1000
    resp = _FakeResponse(status=200, content_length=None, chunks=chunks)
    with _patch_session(resp):
        with pytest.raises(Exception) as exc:
            await download_texture("http://example.com/stream.png", max_bytes=1000)
    assert "too large" in str(exc.value)


@pytest.mark.asyncio
async def test_normal_small_image_returns_bytes():
    chunks = [b"abc", b"def"]
    resp = _FakeResponse(status=200, content_length=6, chunks=chunks)
    with _patch_session(resp):
        data = await download_texture("http://example.com/ok.png", max_bytes=1024)
    assert data == b"abcdef"


@pytest.mark.asyncio
async def test_non_200_raises():
    resp = _FakeResponse(status=404, content_length=None, chunks=[])
    with _patch_session(resp):
        with pytest.raises(Exception) as exc:
            await download_texture("http://example.com/missing.png", max_bytes=1024)
    assert "Failed to download" in str(exc.value)


@pytest.mark.asyncio
async def test_hard_cap_applies_when_max_bytes_omitted():
    # 不传 max_bytes：Content-Length 超过 _HARD_CAP_BYTES 即拒
    resp = _FakeResponse(status=200, content_length=_HARD_CAP_BYTES + 1, chunks=[b"x"])
    with _patch_session(resp):
        with pytest.raises(Exception) as exc:
            await download_texture("http://example.com/huge.png")
    assert "too large" in str(exc.value)


@pytest.mark.asyncio
async def test_zero_or_negative_max_bytes_falls_back_to_hard_cap():
    # max_bytes<=0 退化为硬顶：小体应通过
    chunks = [b"y" * 100]
    resp = _FakeResponse(status=200, content_length=100, chunks=chunks)
    with _patch_session(resp):
        data = await download_texture("http://example.com/ok.png", max_bytes=0)
    assert data == b"y" * 100
