<template>
  <div
    class="flex min-h-screen items-center justify-center bg-[var(--color-background-hero-light)] p-5 transition-[background] duration-300 dark:bg-[var(--color-background-hero-dark)]"
  >
    <div
      class="w-full max-w-[440px] rounded-[16px] border border-[var(--color-border)] bg-[var(--color-card-background)] p-10 shadow-[0_8px_32px_rgba(0,0,0,0.1)] animate-slide-up"
    >
      <div class="mb-8 text-center">
        <h1 class="m-0 mb-2 text-[28px] font-semibold text-[var(--color-heading)]">重设邮箱</h1>
        <p class="m-0 text-sm text-[var(--color-text-light)]">验证新邮箱后，它将替换当前账号邮箱</p>
      </div>

      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" size="large">
        <el-form-item label="新邮箱地址" prop="email">
          <el-input
            v-model="form.email"
            :maxlength="254"
            placeholder="请输入新邮箱地址"
            :prefix-icon="Message"
          />
        </el-form-item>

        <el-form-item label="验证码" prop="code">
          <div class="flex w-full gap-3">
            <el-input
              v-model="form.code"
              :maxlength="8"
              placeholder="请输入验证码"
              :prefix-icon="Ticket"
            />
            <el-button
              type="primary"
              plain
              :disabled="countdown > 0"
              :loading="codeLoading"
              class="h-12 min-w-[120px]"
              @click="sendCode"
            >
              {{ countdown > 0 ? `${countdown}s` : '发送验证码' }}
            </el-button>
          </div>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="loading" class="w-full" @click="submit">
            {{ loading ? '提交中...' : '确认重设邮箱' }}
          </el-button>
        </el-form-item>
      </el-form>

      <div class="mt-6 text-center">
        <el-button link type="primary" @click="router.push('/dashboard/profile')">
          返回个人资料
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { Message, Ticket } from '@element-plus/icons-vue'
import { changeEmail, sendEmailChangeCode } from '@/api/me'
import { getErrorMessage, isValidationError } from '@/utils/error'

const router = useRouter()
const formRef = ref<FormInstance | null>(null)
const loading = ref(false)
const codeLoading = ref(false)
const countdown = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

const form = reactive({
  email: '',
  code: '',
})

const rules: FormRules = {
  email: [
    { required: true, message: '请输入新邮箱地址', trigger: 'blur' },
    { type: 'email', message: '请输入有效的邮箱地址', trigger: 'blur' },
  ],
  code: [{ required: true, message: '请输入验证码', trigger: 'blur' }],
}

async function sendCode() {
  try {
    if (!formRef.value) return
    await formRef.value.validateField('email')
  } catch {
    ElMessage.warning('请先输入有效的新邮箱地址')
    return
  }

  try {
    codeLoading.value = true
    const response = await sendEmailChangeCode({ email: form.email })
    ElMessage.success('验证码已发送到新邮箱')
    startCountdown(Math.min(response.data.ttl, 60))
  } catch (error: unknown) {
    ElMessage.error('发送失败: ' + getErrorMessage(error, '请稍后再试'))
  } finally {
    codeLoading.value = false
  }
}

async function submit() {
  try {
    if (!formRef.value) return
    await formRef.value.validate()
    loading.value = true
    await changeEmail({ email: form.email, code: form.code })
    ElMessage.success('邮箱重设成功')
    await router.push('/dashboard/profile')
  } catch (error: unknown) {
    if (!isValidationError(error)) {
      ElMessage.error('重设失败: ' + getErrorMessage(error, '请稍后再试'))
    }
  } finally {
    loading.value = false
  }
}

function startCountdown(seconds: number) {
  if (timer) clearInterval(timer)
  countdown.value = seconds
  timer = setInterval(() => {
    countdown.value--
    if (countdown.value <= 0 && timer) {
      clearInterval(timer)
      timer = null
    }
  }, 1000)
}

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})
</script>

<style scoped>
:deep(.el-form-item__label) {
  font-weight: 500;
  color: var(--color-text);
}

:deep(.el-input__wrapper) {
  height: 48px;
}
</style>
