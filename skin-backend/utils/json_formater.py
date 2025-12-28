from utils.typing import User, PlayerProfile, Session


def format_user_json(user: User) -> dict:
    """格式化用户信息为 JSON"""
    return {
        "id": user.id,
        "username": user.username,
        "email": user.email,
        "is_admin": user.is_admin,
        "created_at": user.created_at.isoformat(),
    }
