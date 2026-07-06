# union-svc

union-svc is a standalone sidecar for Element-Skin that communicates with the
main site through its public OAuth2 and API endpoints. It coordinates Union
network membership, profile synchronization, and remote character import.

## Run locally

```bash
cd union-svc
cp config.yaml.example config.yaml
# Edit config.yaml with your OAuth2 client credentials.
go build ./cmd/union-svc
./union-svc --config config.yaml
```

## Configuration

Configuration is loaded from the YAML file given by `--config` and can be
overridden with environment variables prefixed by `UNION_`:

- `UNION_SERVER_ADDR`
- `UNION_SERVER_PORT`
- `UNION_ELEMENTSKIN_BASE_URL`
- `UNION_ELEMENTSKIN_OAUTH_CLIENT_ID`
- `UNION_ELEMENTSKIN_OAUTH_CLIENT_SECRET`
- `UNION_ELEMENTSKIN_OAUTH_REDIRECT_URI`
- `UNION_STORAGE_PATH`
- `UNION_LOG_LEVEL`
