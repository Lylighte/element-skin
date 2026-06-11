"""阶段5：JWT 密钥启动 fail-fast 校验单测。

只测纯函数 `_validate_jwt_secret`，不触发模块级副作用，故与 config.yaml 的
出厂占位密钥无关，可在合规/弱口令两侧都断言。
"""

import pytest

from utils.jwt_utils import (
    _validate_jwt_secret,
    _DEFAULT_SECRET,
    _SHIPPED_PLACEHOLDER,
)


def test_validate_rejects_empty():
    with pytest.raises(RuntimeError):
        _validate_jwt_secret("")


def test_validate_rejects_none():
    with pytest.raises(RuntimeError):
        _validate_jwt_secret(None)


def test_validate_rejects_code_default():
    with pytest.raises(RuntimeError):
        _validate_jwt_secret(_DEFAULT_SECRET)


def test_validate_rejects_shipped_placeholder():
    """config.yaml 出厂占位密钥必须被拒——这是运营最常见的失配。"""
    with pytest.raises(RuntimeError):
        _validate_jwt_secret(_SHIPPED_PLACEHOLDER)


def test_validate_rejects_too_short():
    # 31 字节，差一个字节
    with pytest.raises(RuntimeError):
        _validate_jwt_secret("x" * 31)


def test_validate_accepts_strong_secret():
    # 32 字节随机串：合规，不抛错
    _validate_jwt_secret("a" * 32)
    _validate_jwt_secret("9f3c1b7e2d4a6f8091a2b3c4d5e6f70819aabbccddeeff00")


def test_validate_counts_bytes_not_chars():
    """长度判定按 UTF-8 字节而非字符：10 个三字节汉字 = 30 字节，应被拒。"""
    with pytest.raises(RuntimeError):
        _validate_jwt_secret("密" * 10)  # 30 bytes
    # 11 个汉字 = 33 字节，通过
    _validate_jwt_secret("密" * 11)
