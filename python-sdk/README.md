# Element Skin Python SDK

`element-skin-sdk` provides Python helpers for Element Skin OAuth 2.1 flows and `/v1` API access.

## Install for local development

```bash
pip install -e .[test]
```

## Authorization Code with PKCE

```python
from element_skin_sdk import OAuthClient
from element_skin_sdk.permissions import ProfileScopes, TextureScopes

oauth = OAuthClient(
    base_url="https://skin.example.com",
    client_id="app_id",
    redirect_uri="https://app.example.com/callback",
)

session = oauth.authorization_url([
    ProfileScopes.READ_OWNED,
    TextureScopes.READ_OWNED,
])

print(session.authorization_url)

# After the browser redirects back:
tokens = oauth.exchange_code(code="returned-code", code_verifier=session.code_verifier)
```

## Device Code Flow

```python
device = oauth.start_device_flow([ProfileScopes.READ_OWNED])
print(device.verification_uri_complete)
tokens = oauth.poll_device_token(device.device_code)
```

## Client Credentials

Client credentials are for app-only capabilities approved by administrators.

```python
from element_skin_sdk.permissions import MinecraftScopes

oauth = OAuthClient(
    base_url="https://skin.example.com",
    client_id="server_app",
    client_secret="secret",
)

tokens = oauth.client_credentials([MinecraftScopes.SESSION_HASJOINED_SERVER])
```

## API Access

```python
from element_skin_sdk import ElementSkinAPI

api = ElementSkinAPI("https://skin.example.com", access_token=tokens.access_token)
me = api.me()
```
