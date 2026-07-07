# union-svc

union-svc is a standalone sidecar for Element-Skin that communicates with the
main site through its public OAuth2 and API endpoints. It coordinates Union
network membership, profile synchronization, and remote character import.

## Run locally

```bash
cd union-svc
cp config.yaml.example config.yaml
# Edit config.yaml — all empty fields are required.
go build ./cmd/union-svc
./union-svc --config config.yaml
```

## Configuration

Configuration is loaded from the YAML file given by `--config` and can be
overridden with environment variables prefixed by `UNION_`. Required fields
that are empty at startup will cause a validation error listing all missing
values.

### Server

- `UNION_SERVER_ADDR`
- `UNION_SERVER_PORT` (default `8001`)

### Element-Skin (all required)

- `UNION_ELEMENTSKIN_BASE_URL`
- `UNION_ELEMENTSKIN_OAUTH_CLIENT_ID`
- `UNION_ELEMENTSKIN_OAUTH_CLIENT_SECRET`
- `UNION_ELEMENTSKIN_OAUTH_REDIRECT_URI`
- `UNION_ELEMENTSKIN_SERVICE_ACCOUNT_CLIENT_ID`
- `UNION_ELEMENTSKIN_SERVICE_ACCOUNT_CLIENT_SECRET`
- `UNION_ELEMENTSKIN_SERVICE_ACCOUNT_SCOPE` (default `profile.read.any`)

### Storage

- `UNION_STORAGE_PATH` (default `./union-svc.db`)

### Union (HubURL and MemberKey required)

- `UNION_UNION_HUB_URL`
- `UNION_UNION_MEMBER_KEY`
- `UNION_UNION_CORS_ALLOW_ORIGIN` (optional; when empty, no CORS header is emitted)
- `UNION_UNION_TIMEOUT_SECONDS` (default `30`)

### TLS (optional, not yet wired into HTTP clients)

- `UNION_TLS_INSECURE_SKIP_VERIFY`
- `UNION_TLS_CA_FILE`

### Logging

- `UNION_LOG_LEVEL` (default `info`)
