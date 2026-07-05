import { afterEach, describe, expect, it, vi } from 'vitest'
import type { User } from '@/api/types'
import {
  buildUserSearchParams,
  formatBanRemaining,
  formatBanUntilTime,
  hasUserRole,
  isUserBanned,
  userAvatarInitial,
} from '../userListDisplay'

function user(overrides: Partial<User> = {}): User {
  return {
    id: 'user-1',
    email: 'alpha@example.com',
    ...overrides,
  }
}

describe('userListDisplay', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('builds exact search params with trimmed query and extra cursor params', () => {
    expect(buildUserSearchParams('  alex  ', 15, { cursor: 'cursor-1', limit: 8 })).toEqual({
      limit: 8,
      cursor: 'cursor-1',
      q: 'alex',
    })
  })

  it('omits blank search query from list params', () => {
    expect(buildUserSearchParams('   ', 15)).toEqual({ limit: 15 })
  })

  it('checks user roles exactly', () => {
    const row = user({ roles: ['user', 'admin'] })

    expect(hasUserRole(row, 'admin')).toBe(true)
    expect(hasUserRole(row, 'moderator')).toBe(false)
    expect(hasUserRole(user({ roles: undefined }), 'admin')).toBe(false)
  })

  it('detects active bans against current time', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-01-01T00:00:00.000Z'))

    expect(isUserBanned(user({ banned_until: Date.now() + 1 }))).toBe(true)
    expect(isUserBanned(user({ banned_until: Date.now() }))).toBe(false)
    expect(isUserBanned(user({ banned_until: null }))).toBe(false)
    expect(isUserBanned(user({ banned_until: undefined }))).toBe(false)
  })

  it('uses display name initials before email initials', () => {
    expect(userAvatarInitial(user({ display_name: 'beta', email: 'alpha@example.com' }))).toBe('B')
    expect(userAvatarInitial(user({ display_name: '', email: 'gamma@example.com' }))).toBe('G')
  })

  it('formats ban remaining with exact minute hour and day buckets', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-01-01T00:00:00.000Z'))

    expect(formatBanRemaining(null)).toBe('')
    expect(formatBanRemaining(Date.now() + 20 * 60_000)).toBe('20 分钟')
    expect(formatBanRemaining(Date.now() + 2 * 60 * 60_000 + 30 * 60_000)).toBe('2 小时')
    expect(formatBanRemaining(Date.now() + 3 * 24 * 60 * 60_000 + 1)).toBe('3 天')
  })

  it('formats ban until time from preset or custom hours', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-01-01T00:00:00.000Z'))

    expect(formatBanUntilTime('preset', 24, 2)).toBe(
      new Date(Date.now() + 24 * 3600000).toLocaleString(),
    )
    expect(formatBanUntilTime('custom', 24, 2)).toBe(
      new Date(Date.now() + 2 * 3600000).toLocaleString(),
    )
  })
})
