# 阶段 4：管理员侧账号操作——吊销会话 + 级联删 profile

## 目标

主改 `backends/admin_backend.py`，补齐管理员操作的安全副作用：

1. **管理员重置密码后吊销该用户全部 refresh**，与用户自助改密/重置对齐。
2. **删 profile 走阶段 1 的 `delete_profile_cascade`**，消除孤儿 token、保证原子。

## 问题证据

- `backends/admin_backend.py:149-157` `reset_user_password`：只 `update_password`，**不** `delete_refresh_tokens_by_user`。而用户自助路径都吊销（`site_backend.py` `change_password`/`reset_password` 均调 `delete_refresh_tokens_by_user`）。管理员重置往往正因账号被盗，攻击者的 refresh 却仍有效。
- `admin_backend.py:130-131` 删 profile：`delete_tokens_by_profile` + `delete_profile` 两条独立自动提交语句，非原子。

## 改造清单

### 4.1 重置密码吊销会话

```python
async def reset_user_password(self, user_id: str, new_password: str):
    from utils.password_utils import hash_password
    user_row = await self.db.user.get_by_id(user_id)
    if not user_row:
        raise HTTPException(status_code=404, detail="user not found")

    password_hash = hash_password(new_password)
    await self.db.user.update_password(user_id, password_hash)
    # 管理员重置后强制该用户全部会话失效（与自助改密一致）
    await self.db.user.delete_refresh_tokens_by_user(user_id)
    return {"ok": True}
```

> access token 仍无状态，最多残留 30 分钟（见阶段 5/10 关于 `token_epoch` 的权衡）。本阶段先吊销 refresh，使长效会话立即失效。

### 4.2 级联删 profile

将 `admin_backend.py:130-131` 的两条独立删除替换为阶段 1 的事务方法：

```python
    ok = await self.db.user.delete_profile_cascade(profile_id)
    if not ok:
        raise HTTPException(status_code=404, detail="profile not found")
```

> 同时审查 `site_backend.py:574` 的用户自助删 profile：若它也只调 `delete_profile`（漏删 token），一并切到 `delete_profile_cascade`。该改动属阶段 3 或本阶段均可，建议在本阶段统一收口「删 profile 必级联」。

## 影响文件

- `backends/admin_backend.py`（主）
- 必要时 `backends/site_backend.py`（用户自助删 profile 统一走级联）

## 验证

```bash
cd skin-backend
pytest tests/backends/test_admin_backend.py -q
pytest tests/backends/test_admin_profiles.py -q
pytest -q
```

新增针对性用例：

- 管理员重置密码后，该用户既有 refresh 全部失效（`get_refresh_token` 返回 None / 轮换 401）。
- 删 profile 后该 profile 的 token 同时消失；删不存在的 profile 返回 404。
