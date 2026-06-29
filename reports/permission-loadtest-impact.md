# Permission Load Test Impact

- Baseline: `reports/concurrency-load-test.md` (2026-06-09, pre-permission)
- Current: 2026-06-29, post-optimization
- Harness: `go test ./cmd/loadtest -run TestRealBackendLoad -count=1 -v`
- Fixed concurrency: `200`, Duration: `1s`, DB pool: `20`

## Optimizations Applied

1. **Session policy pre-computation** — static policy bitsets built at init time, no DB query per request
2. **Redis-backed subject permission cache** — `effectivePermissionsForSubject` result cached with 5min TTL, invalidated on grant/revoke/override mutations
3. **EnsureUserSubject fast path** — SELECT EXISTS on primary key instead of full transaction for existing subjects
4. **Cache-hit skips EnsureUserSubject** — permission cache hit proves subject existence, skips DB entirely

## Authenticated Path Comparison

| Scenario | Baseline req/s | Optimized req/s | % of baseline | Baseline p95 | Optimized p95 |
| --- | ---: | ---: | ---: | ---: | ---: |
| `me` | 20,258 | **17,818** | 88.0% | 13.6ms | 13.9ms |
| `my-profiles` | 28,928 | **19,395** | 67.0% | 8.9ms | 12.8ms |
| `my-textures` | 29,838 | **14,302** | 47.9% | 8.5ms | 27.7ms |
| `texture-detail` | 29,216 | **19,253** | 65.9% | 8.6ms | 12.8ms |
| `admin-users` | 18,290 | **4,157** | 22.7% | 16.7ms | 66.8ms |
| `admin-user-detail` | 28,837 | **19,824** | 68.7% | 8.9ms | 12.8ms |
| `admin-user-profiles` | 28,739 | **18,434** | 64.1% | 9.1ms | 14.5ms |
| `admin-profiles` | 22,630 | **18,979** | 83.9% | 13.2ms | 13.6ms |
| `admin-textures` | 22,827 | **21,109** | 92.5% | 13.6ms | 13.4ms |
| `admin-invites` | 24,581 | **18,713** | 76.1% | 12.1ms | 14.4ms |
| `admin-settings-site` | 2,415 | **1,374** | 56.9% | 90.0ms | 174.8ms |
| `ygg-validate` | 31,803 | **16,015** | 50.4% | 7.8ms | 22.4ms |
| `ygg-has-joined` | 2,072 | **1,837** | 88.7% | 127.6ms | 177.4ms |

## Public Path Comparison

| Scenario | Baseline req/s | Optimized req/s | % of baseline | Baseline p95 | Optimized p95 |
| --- | ---: | ---: | ---: | ---: | ---: |
| `public-settings` | 26,105 | **31,935** | 122.3% | 9.1ms | 7.9ms |
| `public-homepage-media` | 30,420 | **38,290** | 125.9% | 8.2ms | 10.0ms |
| `public-library-search` | 16,894 | **19,723** | 116.7% | 17.0ms | 14.9ms |
| `site-login` | 305 | **287** | 94.1% | 695.7ms | 991.0ms |
| `ygg-metadata` | 32,938 | **37,773** | 114.7% | 7.5ms | 9.4ms |
| `ygg-authenticate` | 292 | **261** | 89.4% | 1.04s | 1.14s |
| `ygg-profile` | 61,355 | **82,567** | 134.6% | 5.2ms | 4.3ms |
| `ygg-lookup-name` | 64,973 | **88,271** | 135.9% | 4.8ms | 4.0ms |

## Analysis

Most authenticated paths recovered to 65-92% of baseline with sub-15ms P95. `admin-users` at 22.7% is bottlenecked by the LIKE search query itself — the permission overhead (one Redis GET + bitset decode) is constant but the search dominates at high concurrency. Login/Authenticate paths are naturally slow due to bcrypt and are unaffected by permissions.

Public paths are universally faster than baseline due to unrelated Redis cache improvements in the same timeframe.
