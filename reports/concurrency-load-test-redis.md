# Backend Concurrency Load Test Report

- Generated at: `2026-06-09T01:17:35+08:00`
- Harness: `go test ./cmd/loadtest -run TestRealBackendLoad -count=1 -v`
- Data set: 100 users, 300 profiles, 500 texture rows, 50 invites
- Fixed concurrency: `200`
- Duration per level: `1s`
- Backend database pool used by harness: `20` max connections
- Test database: isolated `elementskin_go_test_*`, dropped by test cleanup

- Redis: real test Redis with isolated `elementskin:test:*` key prefix, cleaned by test cleanup
- Auth rate limiting: disabled for load-test login scenario to measure login throughput instead of 429 policy

## Scenario Coverage

| Area | Scenario | Method | Path |
| --- | --- | --- | --- |
| Public home | `public-settings` | `GET` | `/public/settings` |
| Public home | `public-homepage-media` | `GET` | `/public/homepage-media` |
| Public library | `public-library-search` | `GET` | `/public/skin-library?limit=20&q=Load` |
| Authentication | `site-login` | `POST` | `/site-login` |
| User center | `me` | `GET` | `/me` |
| User center | `my-profiles` | `GET` | `/me/profiles?limit=20` |
| User center | `my-textures` | `GET` | `/me/textures?limit=20` |
| User center | `texture-detail` | `GET` | `/me/textures/load_texture_001_000/skin` |
| Admin console | `admin-users` | `GET` | `/admin/users?limit=20&q=Load` |
| Admin console | `admin-user-detail` | `GET` | `/admin/users/1a456ea0eab84cabbf37b2821e8bbfa0` |
| Admin console | `admin-user-profiles` | `GET` | `/admin/users/1a456ea0eab84cabbf37b2821e8bbfa0/profiles?limit=20` |
| Admin console | `admin-profiles` | `GET` | `/admin/profiles?limit=20` |
| Admin console | `admin-textures` | `GET` | `/admin/textures?limit=20` |
| Admin console | `admin-invites` | `GET` | `/admin/invites?limit=20` |
| Admin console | `admin-settings-site` | `GET` | `/admin/settings/site` |

## Fixed-200 One-Second Results

| Area | Scenario | Concurrency | Requests | OK | Fail | Fail % | Successful req/s | Total req/s | Avg | P50 | P95 | P99 | Status | First Error |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- |
| Public home | `public-settings` | 200 | 28097 | 28097 | 0 | 0.00 | 27913.0 | 27913.0 | 7.1ms | 6.4ms | 8.9ms | 12.4ms | `200:28097` | `` |
| Public home | `public-homepage-media` | 200 | 29607 | 29607 | 0 | 0.00 | 29457.6 | 29457.6 | 6.7ms | 6.4ms | 8.5ms | 9.7ms | `200:29607` | `` |
| Public library | `public-library-search` | 200 | 16360 | 16360 | 0 | 0.00 | 16203.9 | 16203.9 | 12.1ms | 11.5ms | 17.2ms | 29.0ms | `200:16360` | `` |
| Authentication | `site-login` | 200 | 350 | 350 | 0 | 0.00 | 303.8 | 303.8 | 621.7ms | 587.9ms | 1.10s | 1.11s | `200:350` | `` |
| User center | `me` | 200 | 21698 | 21698 | 0 | 0.00 | 21564.5 | 21564.5 | 9.2ms | 8.9ms | 12.4ms | 16.7ms | `200:21698` | `` |
| User center | `my-profiles` | 200 | 27402 | 27402 | 0 | 0.00 | 27246.4 | 27246.4 | 7.3ms | 7.0ms | 9.4ms | 11.3ms | `200:27402` | `` |
| User center | `my-textures` | 200 | 28454 | 28454 | 0 | 0.00 | 28289.5 | 28289.5 | 7.0ms | 6.7ms | 9.2ms | 11.4ms | `200:28454` | `` |
| User center | `texture-detail` | 200 | 29077 | 29077 | 0 | 0.00 | 28903.5 | 28903.5 | 6.9ms | 6.6ms | 8.7ms | 10.0ms | `200:29077` | `` |
| Admin console | `admin-users` | 200 | 18194 | 18194 | 0 | 0.00 | 18019.4 | 18019.4 | 10.8ms | 10.5ms | 17.4ms | 22.3ms | `200:18194` | `` |
| Admin console | `admin-user-detail` | 200 | 29539 | 29539 | 0 | 0.00 | 29386.0 | 29386.0 | 6.8ms | 6.4ms | 8.8ms | 11.1ms | `200:29539` | `` |
| Admin console | `admin-user-profiles` | 200 | 28708 | 28708 | 0 | 0.00 | 28555.1 | 28555.1 | 6.9ms | 6.7ms | 9.2ms | 11.3ms | `200:28708` | `` |
| Admin console | `admin-profiles` | 200 | 23570 | 23570 | 0 | 0.00 | 23472.8 | 23472.8 | 8.4ms | 8.2ms | 12.7ms | 15.5ms | `200:23570` | `` |
| Admin console | `admin-textures` | 200 | 24791 | 24791 | 0 | 0.00 | 23197.3 | 23197.3 | 8.0ms | 7.7ms | 12.5ms | 15.7ms | `200:24791` | `` |
| Admin console | `admin-invites` | 200 | 24853 | 24853 | 0 | 0.00 | 24715.8 | 24715.8 | 8.0ms | 7.8ms | 11.9ms | 14.2ms | `200:24853` | `` |
| Admin console | `admin-settings-site` | 200 | 9343 | 9343 | 0 | 0.00 | 9205.6 | 9205.6 | 21.5ms | 21.2ms | 30.1ms | 36.3ms | `200:9343` | `` |

## Notes

- Every scenario is measured once at the same fixed concurrency, default `200`, for a one-second window.
- `Successful req/s` is the useful per-second throughput under that fixed concurrency.
- This report focuses on realistic frontend page-load endpoints and login; destructive write endpoints are intentionally excluded from high-concurrency runs.
- A failure is any request with a transport error or non-2xx/3xx response.
- The test harness closes the in-process HTTP server and drops the temporary PostgreSQL database during cleanup.
