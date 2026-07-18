<template>
  <UiDialog v-model="visible" title="重设邮箱" :close-on-click-modal="false" @closed="reset">
    <p class="mt-0 mb-6 text-sm leading-6 text-[var(--color-text-light)]">
      验证码将发送至新邮箱，验证成功后将立即替换当前账号邮箱。
    </p>

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
            class="min-w-[120px]"
            @click="sendCode"
          >
            {{ countdown > 0 ? `${countdown}s` : '发送验证码' }}
          </el-button>
        </div>
      </el-form-item>
    </el-form>

    <template #footer>
      <el-button :disabled="loading" @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="loading" @click="submit">确认重设</el-button>
    </template>
  </UiDialog>
</template>

<script setup lang="ts">
import { onBeforeUnmount, reactive, ref } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { Message, Ticket } from '@element-plus/icons-vue'
import { changeEmail, sendEmailChangeCode } from '@/api/me'
import UiDialog from '@/components/ui/UiDialog.vue'
import { getErrorMessage, isValidationError } from '@/utils/error'

const visible = defineModel<boolean>({ required: true })
const emit = defineEmits<{
  changed: []
}>()

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
    visible.value = false
    emit('changed')
  } catch (error: unknown) {
    if (!isValidationError(error)) {
      ElMessage.error('重设失败: ' + getErrorMessage(error, '请稍后再试'))
    }
  } finally {
    loading.value = false
  }
}

function startCountdown(seconds: number) {
  stopCountdown()
  countdown.value = seconds
  timer = setInterval(() => {
    countdown.value--
    if (countdown.value <= 0) stopCountdown()
  }, 1000)
}

function stopCountdown() {
  if (timer) clearInterval(timer)
  timer = null
}

function reset() {
  stopCountdown()
  countdown.value = 0
  form.email = ''
  form.code = ''
  formRef.value?.clearValidate()
}

onBeforeUnmount(stopCountdown)
</script>
