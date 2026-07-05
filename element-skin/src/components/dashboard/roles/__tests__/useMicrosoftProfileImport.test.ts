import { afterEach, describe, expect, it, vi } from 'vitest'
import type { MicrosoftProfileResponse } from '@/api/types'
import {
  microsoftCallbackParams,
  microsoftDialogProfile,
  useMicrosoftProfileImport,
} from '../useMicrosoftProfileImport'

function microsoftProfileResponse(
  overrides: Partial<MicrosoftProfileResponse> = {},
): MicrosoftProfileResponse {
  return {
    profile: {
      id: '00000000111122223333444444444444',
      name: 'Steve',
      has_game: false,
    },
    has_game: true,
    import_token: 'import-token',
    ...overrides,
  }
}

describe('useMicrosoftProfileImport helpers', () => {
  it('extracts exact Microsoft callback query params', () => {
    expect(microsoftCallbackParams('?ms_token=profile-token&state=abc')).toEqual({
      msToken: 'profile-token',
      error: null,
    })
    expect(microsoftCallbackParams(new URLSearchParams('error=access_denied'))).toEqual({
      msToken: null,
      error: 'access_denied',
    })
  })

  it('uses top-level has_game from server response for dialog profile', () => {
    expect(
      microsoftDialogProfile(
        microsoftProfileResponse({
          profile: {
            id: 'profile-id',
            name: 'Alex',
            has_game: false,
          },
          has_game: true,
        }),
      ),
    ).toEqual({
      id: 'profile-id',
      name: 'Alex',
      has_game: true,
    })
  })
})

describe('useMicrosoftProfileImport', () => {
  afterEach(() => {
    vi.useRealTimers()
  })

  it('starts Microsoft auth and redirects to exact auth url', async () => {
    const getAuthUrl = vi.fn().mockResolvedValue({ data: { auth_url: 'https://login.example' } })
    const redirectTo = vi.fn()
    const state = useMicrosoftProfileImport({ getAuthUrl, redirectTo })

    await state.startMicrosoftAuth()

    expect(getAuthUrl).toHaveBeenCalledTimes(1)
    expect(redirectTo).toHaveBeenCalledTimes(1)
    expect(redirectTo).toHaveBeenCalledWith('https://login.example')
  })

  it('reports exact auth start errors', async () => {
    const error = vi.fn()
    const state = useMicrosoftProfileImport({
      error,
      getAuthUrl: vi
        .fn()
        .mockRejectedValue({ response: { data: { detail: 'auth endpoint unavailable' } } }),
    })

    await state.startMicrosoftAuth()

    expect(error).toHaveBeenCalledTimes(1)
    expect(error).toHaveBeenCalledWith('启动微软登录失败: auth endpoint unavailable')
  })

  it('handles callback error and clears query without loading profile', async () => {
    const error = vi.fn()
    const clearQuery = vi.fn()
    const getProfile = vi.fn()
    const state = useMicrosoftProfileImport({ error, clearQuery, getProfile })

    await state.handleMicrosoftCallback('?error=access_denied')

    expect(error).toHaveBeenCalledWith('微软登录失败: access_denied')
    expect(clearQuery).toHaveBeenCalledTimes(1)
    expect(getProfile).not.toHaveBeenCalled()
    expect(state.showMicrosoftLoginDialog.value).toBe(false)
  })

  it('ignores callback without token or error', async () => {
    const clearQuery = vi.fn()
    const getProfile = vi.fn()
    const state = useMicrosoftProfileImport({ clearQuery, getProfile })

    await state.handleMicrosoftCallback('?state=only')

    expect(getProfile).not.toHaveBeenCalled()
    expect(clearQuery).not.toHaveBeenCalled()
    expect(state.microsoftProfile.value).toBeNull()
  })

  it('loads callback profile, stores import token and opens confirm dialog exactly', async () => {
    const getProfile = vi.fn().mockResolvedValue({
      data: microsoftProfileResponse({
        import_token: 'server-import-token',
        profile: { id: 'profile-id', name: 'GBwater', has_game: false },
        has_game: true,
      }),
    })
    const success = vi.fn()
    const clearQuery = vi.fn()
    const state = useMicrosoftProfileImport({ getProfile, success, clearQuery })

    await state.handleMicrosoftCallback('?ms_token=profile-token')

    expect(getProfile).toHaveBeenCalledTimes(1)
    expect(getProfile).toHaveBeenCalledWith({ ms_token: 'profile-token' })
    expect(state.microsoftProfile.value).toEqual({
      id: 'profile-id',
      name: 'GBwater',
      has_game: true,
    })
    expect(state.microsoftImportToken.value).toBe('server-import-token')
    expect(state.showMicrosoftLoginDialog.value).toBe(true)
    expect(success).toHaveBeenCalledWith('授权成功！')
    expect(clearQuery).toHaveBeenCalledTimes(1)
  })

  it('reports callback profile errors and still clears query', async () => {
    const error = vi.fn()
    const clearQuery = vi.fn()
    const state = useMicrosoftProfileImport({
      error,
      clearQuery,
      getProfile: vi.fn().mockRejectedValue({ message: 'profile lookup failed' }),
    })

    await state.handleMicrosoftCallback('?ms_token=bad-token')

    expect(error).toHaveBeenCalledWith('获取角色信息失败: profile lookup failed')
    expect(clearQuery).toHaveBeenCalledTimes(1)
    expect(state.showMicrosoftLoginDialog.value).toBe(false)
  })

  it('rejects import when profile or import token is missing', async () => {
    const error = vi.fn()
    const importProfile = vi.fn()
    const state = useMicrosoftProfileImport({ error, importProfile })

    await state.importMicrosoftProfile()
    expect(importProfile).not.toHaveBeenCalled()
    expect(error).not.toHaveBeenCalled()

    state.microsoftProfile.value = { id: 'profile-id', name: 'Steve', has_game: true }
    await state.importMicrosoftProfile()
    expect(importProfile).not.toHaveBeenCalled()
    expect(error).toHaveBeenCalledWith('导入凭证已失效，请重新授权')
  })

  it('imports profile, refreshes dependencies, hides dialog and clears token after delay', async () => {
    vi.useFakeTimers()
    const importProfile = vi.fn().mockResolvedValue({ data: { ok: true } })
    const success = vi.fn()
    const onImported = vi.fn()
    const state = useMicrosoftProfileImport({
      importProfile,
      success,
      onImported,
      resetDelayMs: 25,
    })
    state.showMicrosoftLoginDialog.value = true
    state.microsoftProfile.value = { id: 'profile-id', name: 'Steve', has_game: true }
    state.microsoftImportToken.value = 'import-token'

    await state.importMicrosoftProfile()

    expect(importProfile).toHaveBeenCalledTimes(1)
    expect(importProfile).toHaveBeenCalledWith({ ms_token: 'import-token' })
    expect(success).toHaveBeenCalledWith('正版角色导入成功！')
    expect(onImported).toHaveBeenCalledTimes(1)
    expect(state.showMicrosoftLoginDialog.value).toBe(false)
    expect(state.importing.value).toBe(false)
    expect(state.microsoftProfile.value).toEqual({
      id: 'profile-id',
      name: 'Steve',
      has_game: true,
    })
    expect(state.microsoftImportToken.value).toBe('import-token')

    await vi.advanceTimersByTimeAsync(25)

    expect(state.microsoftProfile.value).toBeNull()
    expect(state.microsoftImportToken.value).toBeNull()
  })

  it('keeps dialog state and reports exact import errors', async () => {
    const error = vi.fn()
    const state = useMicrosoftProfileImport({
      error,
      importProfile: vi
        .fn()
        .mockRejectedValue({ response: { data: { detail: 'import rejected' } } }),
    })
    state.showMicrosoftLoginDialog.value = true
    state.microsoftProfile.value = { id: 'profile-id', name: 'Steve', has_game: true }
    state.microsoftImportToken.value = 'import-token'

    await state.importMicrosoftProfile()

    expect(error).toHaveBeenCalledWith('导入失败: import rejected')
    expect(state.showMicrosoftLoginDialog.value).toBe(true)
    expect(state.microsoftProfile.value).toEqual({
      id: 'profile-id',
      name: 'Steve',
      has_game: true,
    })
    expect(state.microsoftImportToken.value).toBe('import-token')
    expect(state.importing.value).toBe(false)
  })

  it('cancels Microsoft login and clears all transient state', () => {
    const state = useMicrosoftProfileImport()
    state.showMicrosoftLoginDialog.value = true
    state.microsoftProfile.value = { id: 'profile-id', name: 'Steve', has_game: true }
    state.microsoftImportToken.value = 'import-token'
    state.importing.value = true

    state.cancelMicrosoftLogin()

    expect(state.showMicrosoftLoginDialog.value).toBe(false)
    expect(state.microsoftProfile.value).toBeNull()
    expect(state.microsoftImportToken.value).toBeNull()
    expect(state.importing.value).toBe(false)
  })
})
