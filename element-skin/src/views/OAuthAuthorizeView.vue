<template>
  <div class="min-h-[calc(100vh-160px)] px-4 py-10">
    <UiCard class="mx-auto max-w-3xl p-8">
      <div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 class="m-0 text-2xl font-semibold text-[var(--color-heading)]">授权第三方应用</h1>
          <p class="mt-2 mb-0 text-sm text-[var(--color-text-light)]">
            请确认该应用请求的站点能力，授权后应用可代表你调用对应接口。
          </p>
        </div>
        <el-button :loading="loading" @click="loadDetails">刷新</el-button>
      </div>

      <el-alert
        v-if="message"
        class="mb-5"
        :type="messageType"
        :closable="false"
        :title="message"
      />

      <div
        v-if="loading && !details"
        class="py-12 text-center text-sm text-[var(--color-text-light)]"
      >
        正在读取授权请求...
      </div>

      <div v-else-if="details" class="space-y-6">
        <section class="rounded-lg border border-[var(--color-border)] p-5">
          <div class="flex flex-col gap-5 sm:flex-row sm:items-start sm:justify-between">
            <div class="min-w-0">
              <div class="flex items-center gap-3">
                <div
                  class="flex h-12 w-12 shrink-0 items-center justify-center rounded-xl bg-sky-50 text-sky-600 ring-1 ring-sky-100 dark:bg-sky-500/10 dark:text-sky-300 dark:ring-sky-500/20"
                >
                  <el-icon :size="24"><Connection /></el-icon>
                </div>
                <div class="min-w-0">
                  <h2 class="m-0 truncate text-xl font-semibold text-[var(--color-heading)]">
                    {{ details.client.name }}
                  </h2>
                  <p class="mt-1 mb-0 break-all text-xs text-[var(--color-text-light)]">
                    {{ details.client.client_id }}
                  </p>
                </div>
              </div>
              <p
                v-if="details.client.description"
                class="mt-4 mb-0 text-sm leading-6 text-[var(--color-text)]"
              >
                {{ details.client.description }}
              </p>
            </div>
            <a
              v-if="details.client.website_url"
              class="inline-flex items-center gap-1.5 text-sm text-[var(--color-primary)] hover:underline"
              :href="details.client.website_url"
              target="_blank"
              rel="noopener noreferrer"
            >
              <el-icon><Link /></el-icon>
              应用网站
            </a>
          </div>

          <div class="mt-5 grid gap-3 text-sm sm:grid-cols-2">
            <div class="rounded-md bg-[var(--color-background-soft)] px-3 py-2">
              <div class="text-xs text-[var(--color-text-light)]">回调地址</div>
              <div class="mt-1 break-all text-[var(--color-heading)]">
                {{ details.redirect_uri }}
              </div>
            </div>
            <div class="rounded-md bg-[var(--color-background-soft)] px-3 py-2">
              <div class="text-xs text-[var(--color-text-light)]">请求状态</div>
              <div class="mt-1 break-all text-[var(--color-heading)]">
                {{ details.state || '未提供' }}
              </div>
            </div>
          </div>
        </section>

        <section>
          <div class="mb-3 flex items-center justify-between gap-3">
            <h2 class="m-0 text-lg font-semibold text-[var(--color-heading)]">请求权限</h2>
            <el-tag>{{ details.scopes.length }} 项</el-tag>
          </div>
          <div class="space-y-3">
            <div
              v-for="scope in details.scopes"
              :key="scope.code"
              class="rounded-lg border border-[var(--color-border)] p-4"
            >
              <div class="flex flex-wrap items-center gap-2">
                <PermissionToneTag
                  :label="scope.resource_description"
                  tone="sky"
                  variant="category"
                />
                <PermissionToneTag :label="scope.code" tone="slate" :title="scope.description" />
              </div>
              <p class="mt-3 mb-0 text-sm text-[var(--color-text)]">
                {{ scope.description }}
              </p>
              <p class="mt-1 mb-0 text-xs text-[var(--color-text-light)]">
                {{ scope.action_description }} / {{ scope.scope_description }}
              </p>
            </div>
          </div>
        </section>

        <div class="flex flex-col-reverse gap-3 sm:flex-row sm:justify-end">
          <el-button :disabled="deciding" @click="deny">拒绝</el-button>
          <el-button type="primary" :loading="deciding" @click="approve">允许</el-button>
        </div>
      </div>

      <div v-else class="rounded-lg border border-[var(--color-border)] p-6 text-center">
        <el-icon class="text-[var(--color-text-light)]" :size="32"><Warning /></el-icon>
        <p class="mt-3 mb-0 text-sm text-[var(--color-text-light)]">
          授权请求不可用，请从第三方应用重新发起授权。
        </p>
      </div>
    </UiCard>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { Connection, Link, Warning } from '@element-plus/icons-vue'
import {
  approveOAuthAuthorization,
  getOAuthAuthorizationDetails,
  type OAuthAuthorizationDetails,
  type OAuthAuthorizationRequest,
} from '@/api/oauth'
import PermissionToneTag from '@/components/admin/users/PermissionToneTag.vue'
import UiCard from '@/components/ui/UiCard.vue'
import { getErrorMessage } from '@/utils/error'

const route = useRoute()
const details = ref<OAuthAuthorizationDetails | null>(null)
const loading = ref(false)
const deciding = ref(false)
const message = ref('')
const messageType = ref<'success' | 'warning' | 'info' | 'error'>('info')

const authorizationRequest = computed<OAuthAuthorizationRequest>(() => ({
  response_type: queryString('response_type'),
  client_id: queryString('client_id'),
  redirect_uri: queryString('redirect_uri'),
  scope: queryString('scope'),
  state: queryString('state') || undefined,
  code_challenge: queryString('code_challenge'),
  code_challenge_method: queryString('code_challenge_method'),
}))

onMounted(loadDetails)

async function loadDetails() {
  details.value = null
  message.value = ''
  loading.value = true
  try {
    const res = await getOAuthAuthorizationDetails(authorizationRequest.value)
    details.value = res.data
  } catch (error) {
    const fallback = '授权请求无效或已过期'
    const detail = getErrorMessage(error, fallback)
    messageType.value = 'error'
    message.value = detail === 'permission denied' ? '你无权授权此应用' : detail
  } finally {
    loading.value = false
  }
}

async function approve() {
  if (!details.value) return
  deciding.value = true
  message.value = ''
  try {
    const res = await approveOAuthAuthorization(authorizationRequest.value)
    window.location.assign(res.data.redirect_url)
  } catch (error) {
    const detail = getErrorMessage(error, '提交授权失败')
    messageType.value = 'error'
    message.value = detail === 'permission denied' ? '你无权授权此应用' : detail
  } finally {
    deciding.value = false
  }
}

function deny() {
  if (!details.value) return
  window.location.assign(deniedRedirectURL(details.value.redirect_uri, details.value.state))
}

function deniedRedirectURL(redirectURI: string, state?: string) {
  const url = new URL(redirectURI)
  url.searchParams.set('error', 'access_denied')
  if (state) url.searchParams.set('state', state)
  return url.toString()
}

function queryString(key: string) {
  const value = route.query[key]
  if (Array.isArray(value)) return value[0] ?? ''
  return typeof value === 'string' ? value : ''
}
</script>
