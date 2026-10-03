import { createApp, nextTick, type Component } from 'vue'
import ElementPlus, { ElMessage } from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import fixtures from '../../../../testdata/password-policy.json'
import RegisterView from '../RegisterView.vue'
import ResetPassword from '../ResetPassword.vue'

const authMocks = vi.hoisted(() => ({
  register: vi.fn(),
  resetPassword: vi.fn(),
  sendVerificationCode: vi.fn(),
}))
const publicMocks = vi.hoisted(() => ({ getPublicSettings: vi.fn() }))
vi.mock('@/api/auth', () => authMocks)
vi.mock('@/api/public', () => publicMocks)

beforeEach(() => {
  vi.clearAllMocks()
  // Request validation is the subject; rejecting the request avoids a delayed success redirect.
  const failure = {
    response: { data: { error: { object: 'server', operation: 'handle', reason: 'failed' } } },
  }
  authMocks.register.mockRejectedValue(failure)
  authMocks.resetPassword.mockRejectedValue(failure)
})

const entrypointFixtures = fixtures.filter((fixture) => fixture.entrypoint_test)

describe.each([false, true])('password forms with strong check=%s', (strong) => {
  it.each(entrypointFixtures)('register: $name', async (fixture) => {
    const mounted = await mountPage(RegisterView, '/register', strong)
    try {
      Object.assign(mounted.setup.form, {
        username: 'PasswordFormUser',
        email: 'password-form@example.com',
        password: fixture.password,
        confirmPassword: fixture.password,
        code: 'POLICY12',
      })
      await nextTick()
      await mounted.setup.register()
      const accepted = (strong ? fixture.strong_errors : fixture.basic_errors).length === 0
      expect(authMocks.register).toHaveBeenCalledTimes(accepted ? 1 : 0)
      expect(authMocks.resetPassword).not.toHaveBeenCalled()
      if (accepted) {
        expect(authMocks.register).toHaveBeenCalledWith({
          username: 'PasswordFormUser',
          email: 'password-form@example.com',
          password: fixture.password,
          code: 'POLICY12',
        })
      }
    } finally {
      mounted.unmount()
    }
  })

  it.each(entrypointFixtures)('email reset: $name', async (fixture) => {
    const mounted = await mountPage(ResetPassword, '/reset-password', strong)
    try {
      Object.assign(mounted.setup.form, {
        email: 'password-form@example.com',
        password: fixture.password,
        confirmPassword: fixture.password,
        code: 'POLICY12',
      })
      await nextTick()
      await mounted.setup.resetPassword()
      const accepted = (strong ? fixture.strong_errors : fixture.basic_errors).length === 0
      expect(authMocks.resetPassword).toHaveBeenCalledTimes(accepted ? 1 : 0)
      expect(authMocks.register).not.toHaveBeenCalled()
      if (accepted) {
        expect(authMocks.resetPassword).toHaveBeenCalledWith({
          email: 'password-form@example.com',
          password: fixture.password,
          code: 'POLICY12',
        })
      }
    } finally {
      mounted.unmount()
    }
  })
})

interface PasswordFormSetup {
  form: Record<string, string>
  register: () => Promise<void>
  resetPassword: () => Promise<void>
}

async function mountPage(component: Component, path: string, strong: boolean) {
  publicMocks.getPublicSettings.mockResolvedValue({
    data: {
      allow_register: true,
      require_invite: false,
      email_verify_enabled: true,
      enable_strong_password_check: strong,
      email_suffix_policy: { mode: 'disabled', suffixes: [] },
    },
  })
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path, component },
      { path: '/login', component: { template: '<div>login</div>' } },
    ],
  })
  await router.push(path)
  const host = document.createElement('div')
  document.body.appendChild(host)
  const app = createApp(component)
  app.use(router)
  app.use(ElementPlus)
  app.mount(host)
  await Promise.resolve()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await nextTick()
  return {
    setup: app._instance?.setupState as unknown as PasswordFormSetup,
    unmount() {
      ElMessage.closeAll()
      app.unmount()
      host.remove()
    },
  }
}
