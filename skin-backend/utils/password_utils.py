"""密码哈希和验证工具"""

import bcrypt


def hash_password(password: str) -> str:
    """
    使用 bcrypt 哈希密码

    Args:
        password: 明文密码

    Returns:
        str: 哈希后的密码字符串
    """
    return bcrypt.hashpw(password.encode("utf-8"), bcrypt.gensalt()).decode("utf-8")


def verify_password(password: str, hashed: str) -> bool:
    """
    验证密码

    Args:
        password: 明文密码
        hashed: 哈希后的密码

    Returns:
        bool: 密码是否正确
    """
    # bcrypt 密码验证
    if hashed.startswith("$2"):
        try:
            return bcrypt.checkpw(password.encode("utf-8"), hashed.encode("utf-8"))
        except Exception:
            return False
    # 兼容旧的明文密码
    return hashed == password


def needs_rehash(hashed: str) -> bool:
    """
    检查密码哈希是否需要升级

    Args:
        hashed: 哈希后的密码

    Returns:
        bool: 是否需要重新哈希（从明文升级到bcrypt）
    """
    return not hashed.startswith("$2")
