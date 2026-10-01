import { createApp, nextTick } from 'vue'
import ElementPlus, { ElMessage } from 'element-plus'
import { createMemoryHistory, createRouter } from 'vue-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import RegisterView from '../RegisterView.vue'

const authMocks = vi.hoisted(() => ({
  register: vi.fn(),
  sendVerificationCode: vi.fn(),
}))

const publicMocks = vi.hoisted(() => ({
  getPublicSettings: vi.fn(),
}))

vi.mock('@/api/auth', () => authMocks)
vi.mock('@/api/public', () => publicMocks)

beforeEach(() => {
  vi.clearAllMocks()
  publicMocks.getPublicSettings.mockResolvedValue({
    data: {
      allow_register: true,
      require_invite: false,
      email_verify_enabled: false,
      enable_strong_password_check: true,
      email_suffix_policy: { mode: 'disabled', suffixes: [] },
    },
  })
})

describe('RegisterView API validation errors', () => {
  it('shows a generic strong-password validation message', async () => {
    authMocks.register.mockRejectedValue({
      response: {
        status: 400,
        data: {
          error: {
            object: 'password',
            operation: 'validate',
            reason: 'invalid',
          },
        },
      },
    })
    const mounted = await mountRegisterView()

    try {
      Object.assign(mounted.setup.form, {
        username: 'ValidUser',
        email: 'valid@example.com',
        password: 'GoodPass123',
        confirmPassword: 'GoodPass123',
      })

      await mounted.setup.register()
      await flushUI()

      expect(authMocks.register).toHaveBeenCalledTimes(1)
      expect(authMocks.register).toHaveBeenCalledWith({
        username: 'ValidUser',
        email: 'valid@example.com',
        password: 'GoodPass123',
        code: '',
      })
      expect(document.body.textContent).toContain(
        '注册失败: 密码不符合安全要求，请调整后重试',
      )
    } finally {
      mounted.unmount()
    }
  })
})

interface RegisterSetupState {
  form: {
    username: string
    email: string
    password: string
    confirmPassword: string
  }
  register: () => Promise<void>
}

async function mountRegisterView() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/register', component: RegisterView },
      { path: '/login', component: { template: '<div>login</div>' } },
    ],
  })
  await router.push('/register')

  const host = document.createElement('div')
  document.body.appendChild(host)
  const app = createApp(RegisterView)
  app.use(router)
  app.use(ElementPlus)
  app.mount(host)
  await flushUI()

  return {
    setup: app._instance?.setupState as unknown as RegisterSetupState,
    unmount() {
      ElMessage.closeAll()
      app.unmount()
      host.remove()
    },
  }
}

async function flushUI() {
  await Promise.resolve()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await nextTick()
}
