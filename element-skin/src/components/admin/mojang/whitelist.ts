import type { FallbackRow } from '@/components/admin/mojang/types'

export function hasWhitelistChanges(row: FallbackRow) {
  if (!row._loaded) return false
  const initial = row._initialWhitelist
    .map((u) => u.username.toLowerCase())
    .sort()
    .join(',')
  const current = row._whitelist
    .map((u) => u.username.toLowerCase())
    .sort()
    .join(',')
  return initial !== current
}
