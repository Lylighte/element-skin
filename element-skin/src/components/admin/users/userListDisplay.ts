import type { User } from '@/api/types'

export interface UserQueryParams {
  cursor?: string | null
  limit?: number
  q?: string
}

export const banDurationPresets = [
  { label: '1小时', value: 1 },
  { label: '1天', value: 24 },
  { label: '3天', value: 72 },
  { label: '7天', value: 168 },
  { label: '30天', value: 720 },
]

export function buildUserSearchParams(
  activeSearchQuery: string,
  limit: number,
  extraParams: UserQueryParams = {},
): UserQueryParams {
  const params: UserQueryParams = { limit, ...extraParams }
  const q = activeSearchQuery.trim()
  if (q) params.q = q
  return params
}

export function hasUserRole(user: User, role: string) {
  return (user.roles || []).includes(role)
}

export function isUserBanned(user: User) {
  return user.banned_until != null && Date.now() < user.banned_until
}

export function userAvatarInitial(user: User) {
  return user.display_name?.charAt(0).toUpperCase() || user.email.charAt(0).toUpperCase()
}

export function formatBanRemaining(ts: number | null | undefined) {
  if (ts == null) return ''
  const minutes = Math.ceil((ts - Date.now()) / 60000)
  if (minutes > 1440) return Math.floor(minutes / 1440) + ' 天'
  if (minutes > 60) return Math.floor(minutes / 60) + ' 小时'
  return minutes + ' 分钟'
}

export function formatBanUntilTime(durationType: string, presetHours: number, customHours: number) {
  const hours = durationType === 'preset' ? presetHours : customHours
  return new Date(Date.now() + hours * 3600000).toLocaleString()
}
