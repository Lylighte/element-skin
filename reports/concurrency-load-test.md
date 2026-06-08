# Backend Concurrency Load Test Report

- Generated at: `2026-06-09T00:21:36+08:00`
- Harness: `go test ./cmd/loadtest -run TestRealBackendLoad -count=1 -v`
- Data set: 100 users, 300 profiles, 500 texture rows, 50 invites
- Fixed concurrency: `200`
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
| Admin console | `admin-user-detail` | `GET` | `/admin/users/6cdf33bc12fd40c7a57d0601847483bb` |
| Admin console | `admin-user-profiles` | `GET` | `/admin/users/6cdf33bc12fd40c7a57d0601847483bb/profiles?limit=20` |
| Admin console | `admin-profiles` | `GET` | `/admin/profiles?limit=20` |
| Admin console | `admin-textures` | `GET` | `/admin/textures?limit=20` |
| Admin console | `admin-invites` | `GET` | `/admin/invites?limit=20` |
| Admin console | `admin-settings-site` | `GET` | `/admin/settings/site` |

## Fixed-200 One-Second Results

| Area | Scenario | Concurrency | Requests | OK | Fail | Fail % | Successful req/s | Total req/s | Avg | P50 | P95 | P99 | Status | First Error |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | --- | --- |
| Public home | `public-settings` | 200 | 6811 | 6811 | 0 | 0.00 | 6747.7 | 6747.7 | 29.5ms | 23.8ms | 57.9ms | 161.8ms | `200:6811` | `` |
| Public home | `public-carousel` | 200 | 9729 | 9729 | 0 | 0.00 | 9627.1 | 9627.1 | 20.6ms | 14.1ms | 50.9ms | 116.1ms | `200:9729` | `` |
| Public library | `public-library-search` | 200 | 17012 | 17012 | 0 | 0.00 | 16881.4 | 16881.4 | 11.6ms | 11.3ms | 14.3ms | 18.7ms | `200:17012` | `` |
| Authentication | `site-login` | 200 | 465 | 465 | 0 | 0.00 | 310.5 | 310.5 | 598.8ms | 484.9ms | 1.04s | 1.41s | `200:465` | `` |
| User center | `me` | 200 | 24457 | 24457 | 0 | 0.00 | 24333.9 | 24333.9 | 8.1ms | 7.8ms | 10.3ms | 12.6ms | `200:24457` | `` |
| User center | `my-profiles` | 200 | 34352 | 34352 | 0 | 0.00 | 34239.2 | 34239.2 | 5.8ms | 5.3ms | 8.1ms | 9.3ms | `200:34352` | `` |
| User center | `my-textures` | 200 | 29738 | 29738 | 0 | 0.00 | 29564.1 | 29564.1 | 6.6ms | 6.2ms | 9.3ms | 20.6ms | `200:29738` | `` |
| User center | `texture-detail` | 200 | 36235 | 36235 | 0 | 0.00 | 36116.0 | 36116.0 | 5.4ms | 5.0ms | 7.9ms | 9.9ms | `200:36235` | `` |
| Admin console | `admin-users` | 200 | 18252 | 18252 | 0 | 0.00 | 18130.5 | 18130.5 | 10.8ms | 10.6ms | 13.3ms | 19.5ms | `200:18252` | `` |
| Admin console | `admin-user-detail` | 200 | 38754 | 38754 | 0 | 0.00 | 38631.6 | 38631.6 | 5.1ms | 4.7ms | 7.2ms | 9.3ms | `200:38754` | `` |
| Admin console | `admin-user-profiles` | 200 | 37240 | 37240 | 0 | 0.00 | 37150.2 | 37150.2 | 5.3ms | 5.0ms | 7.4ms | 9.0ms | `200:37240` | `` |
| Admin console | `admin-profiles` | 200 | 21336 | 21336 | 0 | 0.00 | 21164.2 | 21164.2 | 9.3ms | 9.1ms | 12.2ms | 16.3ms | `200:21336` | `` |
| Admin console | `admin-textures` | 200 | 19072 | 19072 | 0 | 0.00 | 18932.7 | 18932.7 | 10.4ms | 10.1ms | 13.2ms | 17.8ms | `200:19072` | `` |
| Admin console | `admin-invites` | 200 | 22659 | 22659 | 0 | 0.00 | 22551.1 | 22551.1 | 8.7ms | 8.7ms | 11.1ms | 14.7ms | `200:22659` | `` |
| Admin console | `admin-settings-site` | 200 | 8775 | 8775 | 0 | 0.00 | 8669.1 | 8669.1 | 22.9ms | 22.7ms | 26.5ms | 40.0ms | `200:8775` | `` |

## Notes

- Every scenario is measured once at the same fixed concurrency, default `200`, for a one-second window.
- `Successful req/s` is the useful per-second throughput under that fixed concurrency.
- This report focuses on realistic frontend page-load endpoints and login; destructive write endpoints are intentionally excluded from high-concurrency runs.
- A failure is any request with a transport error or non-2xx/3xx response.
- The test harness closes the in-process HTTP server and drops the temporary PostgreSQL database during cleanup.
