# Backend Concurrency Load Test Report

- Generated at: `2026-06-09T00:14:39+08:00`
- Harness: `go test ./cmd/loadtest -run TestRealBackendLoad -count=1 -v`
- Data set: 100 users, 300 profiles, 500 texture rows, 50 invites
- Fixed concurrency: `500`
- Duration per level: `1s`
- Backend database pool used by harness: `20` max connections
- Test database: isolated `elementskin_go_test_*`, dropped by test cleanup

## Scenario Coverage

| Area | Scenario | Method | Path |
| --- | --- | --- | --- |
| Public home | `public-settings` | `GET` | `/public/settings` |
| Public home | `public-carousel` | `GET` | `/public/carousel` |
| Public library | `public-library-search` | `GET` | `/public/skin-library?limit=20&q=Load` |
| Authentication | `site-login` | `POST` | `/site-login` |
| User center | `me` | `GET` | `/me` |
| User center | `my-profiles` | `GET` | `/me/profiles?limit=20` |
| User center | `my-textures` | `GET` | `/me/textures?limit=20` |
| User center | `texture-detail` | `GET` | `/me/textures/load_texture_001_000/skin` |
| Admin console | `admin-users` | `GET` | `/admin/users?limit=20&q=Load` |
| Admin console | `admin-user-detail` | `GET` | `/admin/users/47c22b1728a14b1981a47023b589584a` |
| Admin console | `admin-user-profiles` | `GET` | `/admin/users/47c22b1728a14b1981a47023b589584a/profiles?limit=20` |
| Admin console | `admin-profiles` | `GET` | `/admin/profiles?limit=20` |
| Admin console | `admin-textures` | `GET` | `/admin/textures?limit=20` |
| Admin console | `admin-invites` | `GET` | `/admin/invites?limit=20` |
| Admin console | `admin-settings-site` | `GET` | `/admin/settings/site` |

## Fixed-500 One-Second Results

| Area | Scenario | Concurrency | Requests | OK | Fail | Fail % | Successful req/s | Total req/s | Avg | P50 | P95 | P99 | Status | First Error |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- |
| Public home | `public-settings` | 500 | 33065 | 3541 | 29524 | 89.29 | 3444.9 | 32168.0 | 15.3ms | 4.0ms | 47.2ms | 424.0ms | `200:3541` | `Get "http://127.0.0.1:59938/public/settings": dial tcp 127.0.0.1:59938: connectex: No connection could be made because the target machine actively refused it.` |
| Public home | `public-carousel` | 500 | 40052 | 4867 | 35185 | 87.85 | 4629.5 | 38097.4 | 12.7ms | 5.3ms | 30.5ms | 77.9ms | `200:4867` | `Get "http://127.0.0.1:59938/public/carousel": dial tcp 127.0.0.1:59938: connectex: No connection could be made because the target machine actively refused it.` |
| Public library | `public-library-search` | 500 | 39929 | 4007 | 35922 | 89.96 | 3911.9 | 38981.8 | 12.6ms | 4.1ms | 52.6ms | 125.6ms | `200:4007` | `Get "http://127.0.0.1:59938/public/skin-library?limit=20&q=Load": dial tcp 127.0.0.1:59938: connectex: No connection could be made because the target machine actively refused it.` |
| Authentication | `site-login` | 500 | 16296 | 450 | 15846 | 97.24 | 262.2 | 9494.7 | 36.0ms | 11.8ms | 50.1ms | 1.10s | `200:450` | `Post "http://127.0.0.1:59938/site-login": dial tcp 127.0.0.1:59938: connectex: No connection could be made because the target machine actively refused it.` |
| User center | `me` | 500 | 44346 | 3716 | 40630 | 91.62 | 3654.7 | 43614.5 | 11.3ms | 4.0ms | 20.1ms | 122.1ms | `200:3716` | `Get "http://127.0.0.1:59938/me": dial tcp 127.0.0.1:59938: connectex: No connection could be made because the target machine actively refused it.` |
| User center | `my-profiles` | 500 | 46993 | 5457 | 41536 | 88.39 | 5410.4 | 46591.7 | 10.6ms | 4.5ms | 30.5ms | 62.2ms | `200:5457` | `Get "http://127.0.0.1:59938/me/profiles?limit=20": dial tcp 127.0.0.1:59938: connectex: No connection could be made because the target machine actively refused it.` |
| User center | `my-textures` | 500 | 43912 | 2620 | 41292 | 94.03 | 2563.6 | 42966.9 | 11.4ms | 4.8ms | 36.0ms | 92.2ms | `200:2620` | `Get "http://127.0.0.1:59938/me/textures?limit=20": dial tcp 127.0.0.1:59938: connectex: No connection could be made because the target machine actively refused it.` |
| User center | `texture-detail` | 500 | 47461 | 2408 | 45053 | 94.93 | 2363.0 | 46574.0 | 10.6ms | 4.5ms | 26.2ms | 64.1ms | `200:2408` | `Get "http://127.0.0.1:59938/me/textures/load_texture_001_000/skin": dial tcp 127.0.0.1:59938: connectex: No connection could be made because the target machine actively refused it.` |
| Admin console | `admin-users` | 500 | 43475 | 3267 | 40208 | 92.49 | 3197.0 | 42543.1 | 11.6ms | 4.2ms | 38.9ms | 80.0ms | `200:3267` | `Get "http://127.0.0.1:59938/admin/users?limit=20&q=Load": dial tcp 127.0.0.1:59938: connectex: No connection could be made because the target machine actively refused it.` |
| Admin console | `admin-user-detail` | 500 | 45943 | 2342 | 43601 | 94.90 | 2311.3 | 45341.3 | 10.8ms | 4.5ms | 25.0ms | 69.3ms | `200:2342` | `Get "http://127.0.0.1:59938/admin/users/47c22b1728a14b1981a47023b589584a": dial tcp 127.0.0.1:59938: connectex: No connection could be made because the target machine actively refused it.` |
| Admin console | `admin-user-profiles` | 500 | 45946 | 2527 | 43419 | 94.50 | 2494.8 | 45359.6 | 10.9ms | 4.5ms | 30.9ms | 70.4ms | `200:2527` | `Get "http://127.0.0.1:59938/admin/users/47c22b1728a14b1981a47023b589584a/profiles?limit=20": dial tcp 127.0.0.1:59938: connectex: No connection could be made because the target machine actively refused it.` |
| Admin console | `admin-profiles` | 500 | 41402 | 1920 | 39482 | 95.36 | 1883.7 | 40618.8 | 12.1ms | 5.0ms | 25.9ms | 80.1ms | `200:1920` | `Get "http://127.0.0.1:59938/admin/profiles?limit=20": dial tcp 127.0.0.1:59938: connectex: No connection could be made because the target machine actively refused it.` |
| Admin console | `admin-textures` | 500 | 44907 | 2236 | 42671 | 95.02 | 2194.4 | 44071.1 | 11.1ms | 5.0ms | 21.7ms | 75.7ms | `200:2236` | `Get "http://127.0.0.1:59938/admin/textures?limit=20": dial tcp 127.0.0.1:59938: connectex: No connection could be made because the target machine actively refused it.` |
| Admin console | `admin-invites` | 500 | 42825 | 4862 | 37963 | 88.65 | 4791.7 | 42205.6 | 11.7ms | 4.5ms | 38.0ms | 96.4ms | `200:4862` | `Get "http://127.0.0.1:59938/admin/invites?limit=20": dial tcp 127.0.0.1:59938: connectex: No connection could be made because the target machine actively refused it.` |
| Admin console | `admin-settings-site` | 500 | 40126 | 1021 | 39105 | 97.46 | 973.4 | 38255.8 | 12.8ms | 4.2ms | 11.3ms | 325.6ms | `200:1021` | `Get "http://127.0.0.1:59938/admin/settings/site": dial tcp 127.0.0.1:59938: connectex: No connection could be made because the target machine actively refused it.` |

## Notes

- Every scenario is measured once at the same fixed concurrency, default `500`, for a one-second window.
- `Successful req/s` is the useful per-second throughput under that fixed concurrency.
- This report focuses on realistic frontend page-load endpoints and login; destructive write endpoints are intentionally excluded from high-concurrency runs.
- A failure is any request with a transport error or non-2xx/3xx response.
- The test harness closes the in-process HTTP server and drops the temporary PostgreSQL database during cleanup.
