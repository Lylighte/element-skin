import { createApp, nextTick, ref } from 'vue'
import ElementPlus, { ElMessage } from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { User } from '@/api/types'
import fixtures from '../../../../../../testdata/password-policy.json'
import DashboardProfile from '../DashboardProfile.vue'

const mocks = vi.hoisted(() => ({
  changePassword: vi.fn(),
  patchMe: vi.fn(),
  deleteMe: vi.fn(),
  sendEmailChangeCode: vi.fn(),
  changeEmail: vi.fn(),
}))
vi.mock('@/api/me', () => mocks)
vi.mock('@/composables/useAvatar', () => ({
  useAvatar: () => ({ currentAvatarImg: ref('') }),
}))
vi.mock('@/easter-eggs', () => ({
  isEasterEggDisabled: () => false,
  setEasterEggDisabled: vi.fn(),
}))

beforeEach(() => {
  vi.clearAllMocks()
  mocks.changePassword.mockResolvedValue({ data: null })
  mocks.patchMe.mockResolvedValue({ data: null })
  mocks.deleteMe.mockResolvedValue({ data: null })
})

interface ProfileSetup {
  form: {
    display_name: string
    old_password: string
    new_password: string
    confirm_password: string
  }
  showDeleteDialog: boolean
  deleteConfirmText: string
  updateProfile: () => Promise<void>
  confirmDeleteAccount: () => Promise<void>
}

describe('profile password and account deletion', () => {
  it.each(fixtures.filter((fixture) => fixture.entrypoint_test && fixture.password !== ''))(
    'password: $name',
    async (fixture) => {
      const mounted = await mountPage()
      try {
        Object.assign(mounted.setup.form, {
          old_password: 'Password123',
          new_password: fixture.password,
          confirm_password: fixture.password,
        })
        await mounted.setup.updateProfile()
        const accepted = fixture.basic_errors.length === 0
        expect(mocks.changePassword).toHaveBeenCalledTimes(accepted ? 1 : 0)
        expect(mocks.patchMe).toHaveBeenCalledTimes(accepted ? 1 : 0)
        if (accepted) {
          expect(mocks.changePassword).toHaveBeenCalledWith({
            old_password: 'Password123',
            new_password: fixture.password,
          })
          expect(mocks.patchMe).toHaveBeenCalledWith({ display_name: 'ProfileUser' })
          expect(mounted.setup.form.new_password).toBe('')
        } else {
          expect(document.body.textContent).toContain('密码不符合安全要求，请调整后重试')
          expect(mounted.setup.form.new_password).toBe(fixture.password)
        }
      } finally {
        mounted.unmount()
      }
    },
  )

  it('clears account state before invoking the shared logout after successful deletion', async () => {
    const mounted = await mountPage()
    const events: string[] = []
    mocks.deleteMe.mockImplementation(async () => {
      events.push('delete')
      return { data: null }
    })
    mounted.logout.mockImplementation(async () => {
      events.push('logout')
      expect(mounted.user.value).toBeNull()
      expect(mounted.setup.showDeleteDialog).toBe(false)
      expect(mounted.setup.deleteConfirmText).toBe('')
    })
    try {
      mounted.setup.showDeleteDialog = true
      mounted.setup.deleteConfirmText = '注销账号'
      await mounted.setup.confirmDeleteAccount()
      expect(mocks.deleteMe).toHaveBeenCalledTimes(1)
      expect(mounted.logout).toHaveBeenCalledTimes(1)
      expect(events).toEqual(['delete', 'logout'])
    } finally {
      mounted.unmount()
    }
  })

  it('keeps account state and avoids logout when deletion is rejected', async () => {
    const mounted = await mountPage()
    mocks.deleteMe.mockRejectedValue({
      response: {
        data: { error: { object: 'protected_subject', operation: 'delete', reason: 'denied' } },
      },
    })
    try {
      mounted.setup.showDeleteDialog = true
      mounted.setup.deleteConfirmText = '注销账号'
      await mounted.setup.confirmDeleteAccount()
      expect(mocks.deleteMe).toHaveBeenCalledTimes(1)
      expect(mounted.logout).not.toHaveBeenCalled()
      expect(mounted.user.value?.id).toBe('profile-user')
      expect(mounted.setup.showDeleteDialog).toBe(true)
      expect(mounted.setup.deleteConfirmText).toBe('注销账号')
      expect(document.body.textContent).toContain('注销失败:')
    } finally {
      mounted.unmount()
    }
  })
})

async function mountPage() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/dashboard/profile', component: DashboardProfile },
      { path: '/', component: { template: '<div>home</div>' } },
    ],
  })
  await router.push('/dashboard/profile')
  const host = document.createElement('div')
  document.body.appendChild(host)
  const user = ref<User | null>({
    id: 'profile-user',
    email: 'profile@example.com',
    display_name: 'ProfileUser',
    permissions: ['account.update.self', 'account.delete.self'],
  } as User)
  const logout = vi.fn(async () => undefined)
  const app = createApp(DashboardProfile)
  app.use(router)
  app.use(ElementPlus)
  app.provide('user', user)
  app.provide('logout', logout)
  app.mount(host)
  await nextTick()
  return {
    user,
    logout,
    setup: app._instance?.setupState as unknown as ProfileSetup,
    unmount() {
      ElMessage.closeAll()
      app.unmount()
      host.remove()
    },
  }
}
