# API 客户端

`ElementSkinAPI` 是常用 Element Skin `/v1` 接口的同步封装。

## 构造客户端

使用 access token：

```python
from element_skin_sdk import ElementSkinAPI

api = ElementSkinAPI(
    "https://skin.example.com",
    access_token="access-token",
)
```

使用 `TokenSet`：

```python
api = ElementSkinAPI("https://skin.example.com", token=tokens)
```

显式传入权限：

```python
api = ElementSkinAPI(
    "https://skin.example.com",
    access_token="access-token",
    permissions=("account.read.self",),
)
```

## 当前用户

```python
me = api.me()
```

接口：

```text
GET /v1/users/me
```

所需权限：

```text
account.read.self
```

## 角色

```python
profiles = api.list_profiles(cursor=None, page_size=20)
created = api.create_profile("Steve", model="default")
updated = api.update_profile("profile-id", name="Alex", model="slim")
api.delete_profile("profile-id")
```

接口：

```text
GET    /v1/users/me/profiles
POST   /v1/users/me/profiles
PATCH  /v1/users/me/profiles/{profile_id}
DELETE /v1/users/me/profiles/{profile_id}
```

## 材质

```python
textures = api.list_textures(texture_type="skin", page_size=20)
texture = api.get_texture("texture-hash", "skin")
updated = api.update_texture("texture-hash", "skin", note="Main skin")
api.delete_texture("texture-hash", "skin")
```

接口：

```text
GET    /v1/users/me/textures
GET    /v1/users/me/textures/{hash}/{texture_type}
PATCH  /v1/users/me/textures/{hash}/{texture_type}
DELETE /v1/users/me/textures/{hash}/{texture_type}
```

## 衣柜操作

```python
api.add_texture_to_wardrobe("texture-hash", texture_type="skin")
api.apply_texture("texture-hash", profile_id="profile-id", texture_type="skin")
```

接口：

```text
POST /v1/users/me/textures/{hash}/wardrobe
POST /v1/users/me/textures/{hash}/apply
```

## Minecraft 能力 API

这些接口是 `/v1/minecraft` 下的站点能力 API，不是 Yggdrasil 协议端点。

```python
profile = api.minecraft_profile("Steve")
profiles = api.minecraft_profiles(["Steve", "Alex"])
joined = api.minecraft_has_joined(
    username="Steve",
    server_id="server-hash",
    ip="127.0.0.1",
)
```

接口：

```text
GET  /v1/minecraft/profiles/by-name/{name}
POST /v1/minecraft/profiles/by-names
POST /v1/minecraft/session/has-joined
```

`minecraft_has_joined` 需要应用自身 token 具备：

```text
minecraft_session.hasjoined.server
```
