import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import {
  getMicrosoftAuthUrl,
  getMicrosoftProfile,
  importMicrosoftProfile as apiImportMicrosoftProfile,
} from '@/api/microsoft'
import type { MicrosoftGameProfile } from '@/api/types'
import { getErrorMessage } from '@/utils/error'

export interface MicrosoftProfileImportDeps {
  getAuthUrl?: typeof getMicrosoftAuthUrl
  getProfile?: typeof getMicrosoftProfile
  importProfile?: typeof apiImportMicrosoftProfile
  redirectTo?: (url: string) => void
  clearQuery?: () => Promise<void> | void
  onImported?: () => Promise<void> | void
  success?: (message: string) => void
  error?: (message: string) => void
  resetDelayMs?: number
}

export function microsoftCallbackParams(search: string | URLSearchParams) {
  const params = typeof search === 'string' ? new URLSearchParams(search) : search
  return {
    msToken: params.get('ms_token'),
    error: params.get('error'),
  }
}

export function microsoftDialogProfile(data: { profile: MicrosoftGameProfile; has_game: boolean }) {
  return {
    ...data.profile,
    has_game: data.has_game,
  }
}

export function useMicrosoftProfileImport(deps: MicrosoftProfileImportDeps = {}) {
  const showMicrosoftLoginDialog = ref(false)
  const microsoftProfile = ref<MicrosoftGameProfile | null>(null)
  const microsoftImportToken = ref<string | null>(null)
  const importing = ref(false)

  const getAuthUrl = deps.getAuthUrl ?? getMicrosoftAuthUrl
  const getProfile = deps.getProfile ?? getMicrosoftProfile
  const importProfile = deps.importProfile ?? apiImportMicrosoftProfile
  const redirectTo =
    deps.redirectTo ??
    ((url: string) => {
      window.location.href = url
    })
  const notifySuccess = deps.success ?? ElMessage.success
  const notifyError = deps.error ?? ElMessage.error
  const resetDelayMs = deps.resetDelayMs ?? 300

  async function startMicrosoftAuth() {
    try {
      const response = await getAuthUrl()
      redirectTo(response.data.auth_url)
    } catch (error: unknown) {
      notifyError('启动微软登录失败: ' + getErrorMessage(error, '启动微软登录失败'))
    }
  }

  async function handleMicrosoftCallback(search: string | URLSearchParams) {
    const { msToken, error } = microsoftCallbackParams(search)
    if (error) {
      notifyError('微软登录失败: ' + error)
      await deps.clearQuery?.()
      return
    }
    if (!msToken) return

    try {
      const response = await getProfile({ ms_token: msToken })
      microsoftProfile.value = microsoftDialogProfile(response.data)
      microsoftImportToken.value = response.data.import_token
      showMicrosoftLoginDialog.value = true
      notifySuccess('授权成功！')
    } catch (e: unknown) {
      notifyError('获取角色信息失败: ' + getErrorMessage(e, '获取角色信息失败'))
    }
    await deps.clearQuery?.()
  }

  async function importMicrosoftProfile() {
    if (!microsoftProfile.value) return
    if (!microsoftImportToken.value) {
      notifyError('导入凭证已失效，请重新授权')
      return
    }

    try {
      importing.value = true
      await importProfile({ ms_token: microsoftImportToken.value })
      notifySuccess('正版角色导入成功！')
      showMicrosoftLoginDialog.value = false
      window.setTimeout(() => {
        microsoftProfile.value = null
        microsoftImportToken.value = null
      }, resetDelayMs)
      await deps.onImported?.()
    } catch (error: unknown) {
      notifyError('导入失败: ' + getErrorMessage(error, '导入失败'))
    } finally {
      importing.value = false
    }
  }

  function cancelMicrosoftLogin() {
    showMicrosoftLoginDialog.value = false
    microsoftProfile.value = null
    microsoftImportToken.value = null
    importing.value = false
  }

  return {
    showMicrosoftLoginDialog,
    microsoftProfile,
    microsoftImportToken,
    importing,
    startMicrosoftAuth,
    handleMicrosoftCallback,
    importMicrosoftProfile,
    cancelMicrosoftLogin,
  }
}
