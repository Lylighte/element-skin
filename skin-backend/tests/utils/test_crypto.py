"""CryptoUtils 单元测试"""

import tempfile

import pytest
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import rsa

from utils.crypto import compute_key_fingerprint, CryptoUtils


@pytest.fixture
def crypto_utils():
    """生成一个临时 RSA 密钥对，返回 (CryptoUtils 实例, 私钥 PEM 字符串)."""
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    pem_bytes = key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption(),
    )
    with tempfile.NamedTemporaryFile(suffix=".pem", delete=False) as f:
        f.write(pem_bytes)
        tmp_path = f.name
    cu = CryptoUtils(tmp_path)
    return cu, pem_bytes.decode("utf-8")


@pytest.fixture
def other_key_pem():
    """生成另一把不同的 RSA 私钥，返回 PEM 字符串."""
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    pem_bytes = key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption(),
    )
    return pem_bytes.decode("utf-8")


def test_reload_from_pem_happy(crypto_utils, other_key_pem):
    """reload 后 sign_data 应使用新密钥产生不同的签名."""
    cu, _ = crypto_utils
    data = "hello-world"

    sig_before = cu.sign_data(data)

    cu.reload_from_pem(other_key_pem)
    sig_after = cu.sign_data(data)

    assert sig_before != sig_after, "reload 后签名应不同"


def test_reload_from_pem_invalid(crypto_utils):
    """传入非法字符串应抛出 ValueError."""
    cu, _ = crypto_utils
    with pytest.raises(ValueError, match="Invalid PEM"):
        cu.reload_from_pem("this is not a pem at all")


def test_reload_from_pem_consistency(crypto_utils, other_key_pem):
    """reload 后 get_public_key_pem 应返回对应公钥."""
    cu, _ = crypto_utils

    cu.reload_from_pem(other_key_pem)

    reloaded_priv = serialization.load_pem_private_key(
        other_key_pem.encode("utf-8"), password=None
    )
    expected_pub = reloaded_priv.public_key().public_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PublicFormat.SubjectPublicKeyInfo,
    ).decode("utf-8")

    assert cu.get_public_key_pem() == expected_pub


def test_compute_key_fingerprint_consistent(other_key_pem):
    """同一密钥的指纹应始终一致."""
    fp1 = compute_key_fingerprint(other_key_pem)
    fp2 = compute_key_fingerprint(other_key_pem)
    assert fp1 == fp2


def test_compute_key_fingerprint_format(other_key_pem):
    """指纹格式应为 ``sha256:<64-小写十六进制>``."""
    fp = compute_key_fingerprint(other_key_pem)
    assert fp.startswith("sha256:")
    expected_length = len("sha256:") + 64
    assert len(fp) == expected_length
    hex_part = fp[7:]
    assert all(c in "0123456789abcdef" for c in hex_part)


def test_compute_key_fingerprint_invalid_pem():
    """无效的 PEM 字符串应抛出 ValueError."""
    with pytest.raises(ValueError, match="Invalid PEM"):
        compute_key_fingerprint("this is definitely not a pem key")


def test_compute_key_fingerprint_different_keys(other_key_pem, crypto_utils):
    """不同密钥的指纹应不同."""
    _, first_pem = crypto_utils
    fp1 = compute_key_fingerprint(first_pem)
    fp2 = compute_key_fingerprint(other_key_pem)
    assert fp1 != fp2
