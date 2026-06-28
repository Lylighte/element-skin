<template>
  <UiDialog v-model="visible" destroy-on-close align-center variant="wide-form">
    <div v-if="user" class="p-6">
      <div
        class="mb-6 flex flex-col gap-4 rounded-xl border border-[var(--color-border)] bg-[var(--color-background-soft)] p-5 md:flex-row md:items-center"
      >
        <el-avatar
          :size="72"
          :shape="user.avatar_hash ? 'square' : 'circle'"
          :class="[
            user.avatar_hash ? 'has-custom' : 'bg-[var(--color-background-mute)]',
            'text-xl font-semibold text-[var(--color-text-light)]',
          ]"
          :src="userAvatars[user.avatar_hash || ''] || ''"
        >
          {{ !user.avatar_hash ? user.email.charAt(0).toUpperCase() : '' }}
        </el-avatar>
        <div class="min-w-0 flex-1">
          <div class="flex flex-wrap items-center gap-2">
            <h3 class="m-0 text-xl font-semibold text-[var(--color-heading)]">
              {{ user.display_name || '未设置显示名' }}
            </h3>
            <el-tag
              v-for="role in assignedRoleLabels"
              :key="role.id"
              :type="role.protected ? 'danger' : 'info'"
              size="small"
            >
              {{ role.name }}
            </el-tag>
          </div>
          <p class="mt-1 mb-0 text-sm text-[var(--color-text-light)]">{{ user.email }}</p>
          <p class="mt-1 mb-0 font-mono text-xs text-[var(--color-text-light)]">
            UID: {{ user.id }}
          </p>
        </div>
        <div class="md:text-right">
          <el-tag v-if="isBanned" type="warning" effect="dark">
            <el-icon><Warning /></el-icon>
            封禁中
          </el-tag>
          <el-tag v-else type="success" effect="dark">
            <el-icon><CircleCheck /></el-icon>
            状态正常
          </el-tag>
          <div v-if="isBanned" class="mt-1 text-xs text-[var(--el-color-warning)]">
            {{ banRemaining }} 后解封
          </div>
        </div>
      </div>

      <el-tabs type="border-card">
        <el-tab-pane label="角色列表">
          <el-table :data="profiles || []" size="small" max-height="320">
            <el-table-column prop="name" label="角色名称" />
            <el-table-column prop="model" label="模型" width="100">
              <template #default="{ row }">
                <el-tag size="small" :type="row.model === 'slim' ? 'success' : 'info'">
                  {{ row.model }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="id" label="角色 UUID" min-width="260" />
          </el-table>
          <el-empty v-if="!profiles?.length" description="该用户暂无角色" :image-size="60" />
          <div class="pagination-container mt-4">
            <CursorPager
              v-if="profiles.length > 0"
              :count="profiles.length"
              :loading="profilesLoading"
              :disabled-prev="profilesPrevDisabled"
              :disabled-next="profilesNextDisabled"
              @prev="$emit('profiles-prev')"
              @next="$emit('profiles-next')"
            />
          </div>
        </el-tab-pane>

        <el-tab-pane label="权限">
          <div v-loading="permissionsLoading" class="space-y-5">
            <section>
              <div class="mb-3 flex items-center justify-between gap-3">
                <h4 class="m-0 text-base font-semibold text-[var(--color-heading)]">角色授权</h4>
                <el-text size="small" type="info">角色提供批量权限，单项覆盖用于精细调整</el-text>
              </div>
              <div class="rounded-lg bg-[var(--color-background-soft)] p-4">
                <div class="mb-4 flex flex-wrap gap-2">
                  <el-tag
                    v-for="role in assignedRoleLabels"
                    :key="role.id"
                    :type="role.protected ? 'danger' : 'info'"
                    :closable="roleTagClosable(role)"
                    disable-transitions
                    @close="emit('revoke-role', role.id)"
                  >
                    {{ role.name }}
                  </el-tag>
                  <el-text v-if="!assignedRoleLabels.length" type="info" size="small">
                    暂无额外角色
                  </el-text>
                </div>
                <div class="flex flex-col gap-2 md:flex-row">
                  <el-select
                    v-model="selectedRoleId"
                    class="md:w-72"
                    placeholder="选择要授予的角色"
                    filterable
                    clearable
                  >
                    <el-option
                      v-for="role in grantableRoles"
                      :key="role.id"
                      :label="role.name"
                      :value="role.id"
                      :disabled="role.protected && !canManageProtected"
                    >
                      <div class="flex items-center justify-between gap-3">
                        <span>{{ role.name }}</span>
                        <el-tag v-if="role.protected" type="danger" size="small">受保护</el-tag>
                      </div>
                    </el-option>
                  </el-select>
                  <el-button
                    type="primary"
                    :icon="Plus"
                    :disabled="!selectedRoleId || !canGrantPermission"
                    @click="grantSelectedRole"
                  >
                    添加角色
                  </el-button>
                </div>
              </div>
            </section>

            <section>
              <div class="mb-3 flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                <h4 class="m-0 text-base font-semibold text-[var(--color-heading)]">单项权限</h4>
              </div>
              <div class="space-y-4">
                <div class="grid gap-4">
                  <div class="flex flex-col gap-2">
                    <div class="text-sm font-semibold text-[var(--color-heading)]">继承的权限</div>
                    <div v-if="inheritedPermissionGroups.length" class="space-y-3">
                      <div class="flex flex-wrap gap-2">
                        <PermissionToneTag
                          v-for="group in inheritedPermissionGroups"
                          :key="group.resource"
                          :label="group.resourceDescription"
                          :tone="group.tone"
                          :count="group.items.length"
                          :active="isSelectedPermissionGroup(group, 'inherited')"
                          variant="category"
                          clickable
                          @click="selectPermissionGroup(group.resource, 'inherited')"
                        />
                      </div>
                      <div class="min-h-10 rounded-lg bg-[var(--color-background-soft)] px-3 py-2">
                        <div class="mb-2 flex items-center gap-1.5">
                          <span class="text-sm font-medium text-[var(--color-heading)]">
                            {{ selectedInheritedPermissionGroup?.resourceDescription }}
                          </span>
                          <span class="text-xs text-[var(--color-text-light)]">
                            {{ selectedInheritedPermissionGroup?.items.length || 0 }} 项
                          </span>
                        </div>
                        <div class="flex flex-wrap gap-2">
                          <span
                            v-for="item in selectedInheritedPermissionGroup?.items || []"
                            :key="item.code"
                          >
                            <PermissionToneTag
                              :label="item.label"
                              :tone="selectedInheritedPermissionGroup?.tone || 'slate'"
                              :title="item.code"
                            />
                          </span>
                        </div>
                      </div>
                    </div>
                    <el-text v-else type="info" size="small">暂无继承权限</el-text>
                  </div>
                  <div class="flex flex-col gap-2">
                    <div class="text-sm font-semibold text-[var(--color-heading)]">覆盖</div>
                    <div v-if="overridePermissionGroups.length" class="space-y-3">
                      <div class="flex flex-wrap gap-2">
                        <PermissionToneTag
                          v-for="group in overridePermissionGroups"
                          :key="group.resource"
                          :label="group.resourceDescription"
                          :tone="group.tone"
                          :count="group.items.length"
                          :active="isSelectedPermissionGroup(group, 'override')"
                          variant="category"
                          clickable
                          @click="selectPermissionGroup(group.resource, 'override')"
                        />
                      </div>
                      <div class="min-h-10 rounded-lg bg-[var(--color-background-soft)] px-3 py-2">
                        <div class="mb-2 flex items-center gap-1.5">
                          <span class="text-sm font-medium text-[var(--color-heading)]">
                            {{ selectedOverridePermissionGroup?.resourceDescription }}
                          </span>
                          <span class="text-xs text-[var(--color-text-light)]">
                            {{ selectedOverridePermissionGroup?.items.length || 0 }} 项
                          </span>
                        </div>
                        <div class="flex flex-wrap gap-2">
                          <span
                            v-for="item in selectedOverridePermissionGroup?.items || []"
                            :key="item.code"
                          >
                            <PermissionToneTag
                              :label="item.label"
                              :tone="selectedOverridePermissionGroup?.tone || 'slate'"
                              :title="item.code"
                              :badge-label="item.effect === 'allow' ? '允许' : '拒绝'"
                              :badge-tone="item.effect"
                              removable
                              @remove="emit('clear-permission', item.code)"
                            />
                          </span>
                        </div>
                      </div>
                    </div>
                    <el-text v-else type="info" size="small">暂无单项权限覆盖</el-text>
                  </div>
                </div>
                <div class="grid gap-2 pt-1 md:grid-cols-[minmax(0,1fr)_120px_auto]">
                  <el-select
                    v-model="selectedPermissionCode"
                    placeholder="选择要覆盖的权限"
                    filterable
                    clearable
                  >
                    <el-option
                      v-for="item in grantablePermissionOptions"
                      :key="item.code"
                      :label="`${item.code} · ${item.description}`"
                      :value="item.code"
                      :disabled="permissionControlDisabled(item)"
                    >
                      <div class="flex flex-col">
                        <span class="font-mono text-xs">{{ item.code }}</span>
                        <span class="text-xs text-[var(--color-text-light)]">
                          {{ item.description }}
                        </span>
                      </div>
                    </el-option>
                  </el-select>
                  <el-select v-model="selectedPermissionEffect">
                    <el-option label="允许" value="allow" :disabled="!canGrantPermission" />
                    <el-option label="拒绝" value="deny" :disabled="!canRevokePermission" />
                  </el-select>
                  <el-button
                    type="primary"
                    :icon="Plus"
                    :disabled="!canAddSelectedPermission"
                    @click="setSelectedPermission"
                  >
                    添加覆盖
                  </el-button>
                </div>
              </div>
            </section>
          </div>
        </el-tab-pane>

        <el-tab-pane label="账号操作">
          <div class="grid gap-4 md:grid-cols-2">
            <div
              class="flex items-center justify-between gap-4 rounded-lg border border-[var(--color-border)] p-4"
            >
              <div>
                <div class="font-semibold text-[var(--color-heading)]">账号封禁</div>
                <div class="mt-1 text-sm text-[var(--color-text-light)]">
                  禁止该用户加入 Minecraft 服务器。
                </div>
              </div>
              <el-button
                v-if="!isBanned"
                type="warning"
                :disabled="isProtectedUser || isSelf"
                @click="$emit('show-ban')"
              >
                执行封禁
              </el-button>
              <el-button v-else type="success" @click="$emit('unban', user)">解除封禁</el-button>
            </div>
            <div
              class="flex items-center justify-between gap-4 rounded-lg border border-[var(--color-border)] p-4"
            >
              <div>
                <div class="font-semibold text-[var(--color-heading)]">强制重置密码</div>
                <div class="mt-1 text-sm text-[var(--color-text-light)]">
                  手动为该用户设置新密码。
                </div>
              </div>
              <el-button @click="$emit('show-reset-password')">重置密码</el-button>
            </div>
            <div
              class="flex items-center justify-between gap-4 rounded-lg border border-[var(--el-color-danger-light-7)] p-4 md:col-span-2"
            >
              <div>
                <div class="font-semibold text-[var(--el-color-danger)]">注销账号</div>
                <div class="mt-1 text-sm text-[var(--color-text-light)]">
                  永久删除该用户及其关联数据。
                </div>
              </div>
              <el-button
                type="danger"
                :disabled="isProtectedUser || isSelf"
                @click="$emit('delete-user', user)"
              >
                删除用户
              </el-button>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>
  </UiDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Warning, CircleCheck, Plus } from '@element-plus/icons-vue'
import CursorPager from '@/components/common/CursorPager.vue'
import PermissionToneTag from '@/components/admin/users/PermissionToneTag.vue'
import type {
  PermissionDefinition,
  PermissionOverrideEffect,
  PermissionRole,
  Profile,
  User,
  UserPermissionsResponse,
} from '@/api/types'
import UiDialog from '@/components/ui/UiDialog.vue'

const visible = defineModel<boolean>('visible', { required: true })

const props = defineProps<{
  user: User | null
  profiles: Profile[]
  userAvatars: Record<string, string>
  profilesLoading: boolean
  profilesPrevDisabled: boolean
  profilesNextDisabled: boolean
  isBanned: boolean
  banRemaining: string
  isSelf: boolean
  permissionState: UserPermissionsResponse | null
  permissionsLoading: boolean
  currentPermissions: string[]
}>()

const emit = defineEmits<{
  'profiles-prev': []
  'profiles-next': []
  'grant-role': [roleId: string]
  'revoke-role': [roleId: string]
  'set-permission': [permissionCode: string, effect: PermissionOverrideEffect]
  'clear-permission': [permissionCode: string]
  'show-ban': []
  unban: [user: User]
  'show-reset-password': []
  'delete-user': [user: User]
}>()

const selectedRoleId = ref('')
const selectedPermissionCode = ref('')
const selectedPermissionEffect = ref<PermissionOverrideEffect>('allow')
const selectedInheritedResource = ref('')
const selectedOverrideResource = ref('')

type PermissionGroupKind = 'inherited' | 'override'
type PermissionTone = 'emerald' | 'sky' | 'violet' | 'amber' | 'rose' | 'slate' | 'cyan'

interface PermissionDisplayItem {
  code: string
  label: string
  resource: string
  resourceDescription: string
  effect?: PermissionOverrideEffect
}

interface PermissionDisplayGroup {
  resource: string
  resourceDescription: string
  tone: PermissionTone
  items: PermissionDisplayItem[]
}

const roleIds = computed(() => new Set(props.permissionState?.roles || props.user?.roles || []))
const effectivePermissions = computed(
  () => new Set(props.permissionState?.effective_permissions || props.user?.permissions || []),
)
const overrideMap = computed(() => {
  const out = new Map<string, PermissionOverrideEffect>()
  for (const item of props.permissionState?.overrides || [])
    out.set(item.permission_code, item.effect)
  return out
})
const currentPermissionSet = computed(() => new Set(props.currentPermissions))
const canManageProtected = computed(() =>
  currentPermissionSet.value.has('permission_protected.manage.any'),
)
const canGrantPermission = computed(() => currentPermissionSet.value.has('permission.grant.any'))
const canRevokePermission = computed(() => currentPermissionSet.value.has('permission.revoke.any'))
const isProtectedUser = computed(() => {
  const protectedRoles = props.permissionState?.catalog.roles.filter((role) => role.protected) || []
  return protectedRoles.some((role) => roleIds.value.has(role.id))
})
const assignedRoleLabels = computed(() => {
  const roles = props.permissionState?.catalog.roles || []
  const selected = roles.filter((role) => roleIds.value.has(role.id))
  if (selected.length) return selected
  return (props.user?.roles || []).map((role) => ({
    id: role,
    name: role,
    description: '',
    system_role: true,
    protected: role === 'super_admin',
    permissions: [],
  }))
})
const grantableRoles = computed(() =>
  (props.permissionState?.catalog.roles || []).filter((role) => !roleIds.value.has(role.id)),
)
const permissionByCode = computed(() => {
  const out = new Map<string, PermissionDefinition>()
  for (const item of props.permissionState?.catalog.permissions || []) out.set(item.code, item)
  return out
})
const inheritedPermissionGroups = computed(() => {
  const inherited = new Map<string, PermissionDisplayItem>()
  for (const role of props.permissionState?.catalog.roles || []) {
    if (!roleIds.value.has(role.id)) continue
    for (const code of role.permissions) {
      if (overrideMap.value.has(code)) continue
      if (!effectivePermissions.value.has(code)) continue
      const definition = permissionByCode.value.get(code)
      inherited.set(code, createPermissionDisplayItem(code, definition))
    }
  }
  return groupPermissionItems([...inherited.values()])
})
const overridePermissionGroups = computed(() => {
  const items = (props.permissionState?.overrides || []).map((item) => ({
    ...createPermissionDisplayItem(
      item.permission_code,
      permissionByCode.value.get(item.permission_code),
    ),
    effect: item.effect,
  }))
  return groupPermissionItems(items)
})
const selectedInheritedPermissionGroup = computed(() =>
  selectedPermissionGroup(inheritedPermissionGroups.value, selectedInheritedResource.value),
)
const selectedOverridePermissionGroup = computed(() =>
  selectedPermissionGroup(overridePermissionGroups.value, selectedOverrideResource.value),
)
const grantablePermissionOptions = computed(() =>
  (props.permissionState?.catalog.permissions || []).filter(
    (item) => !overrideMap.value.has(item.code),
  ),
)
const selectedPermission = computed(() =>
  selectedPermissionCode.value ? permissionByCode.value.get(selectedPermissionCode.value) : null,
)
const canAddSelectedPermission = computed(() => {
  if (!selectedPermission.value) return false
  if (selectedPermissionEffect.value === 'allow' && !canGrantPermission.value) return false
  if (selectedPermissionEffect.value === 'deny' && !canRevokePermission.value) return false
  return !permissionControlDisabled(selectedPermission.value)
})

watch(
  [visible, inheritedPermissionGroups, overridePermissionGroups],
  ([open, inheritedGroups, overrideGroups]) => {
    if (!open) {
      selectedRoleId.value = ''
      selectedPermissionCode.value = ''
      selectedPermissionEffect.value = 'allow'
      selectedInheritedResource.value = ''
      selectedOverrideResource.value = ''
      return
    }

    selectedInheritedResource.value = normalizeSelectedResource(
      selectedInheritedResource.value,
      inheritedGroups,
    )
    selectedOverrideResource.value = normalizeSelectedResource(
      selectedOverrideResource.value,
      overrideGroups,
    )
  },
)

function createPermissionDisplayItem(
  code: string,
  definition?: PermissionDefinition,
): PermissionDisplayItem {
  return {
    code,
    label: definition?.description || code,
    resource: definition?.resource || code.split('.')[0] || 'other',
    resourceDescription: definition?.resource_description || definition?.resource || '其他权限',
  }
}

function groupPermissionItems(items: PermissionDisplayItem[]): PermissionDisplayGroup[] {
  const groups = new Map<string, PermissionDisplayGroup>()
  for (const item of [...items].sort((a, b) => a.code.localeCompare(b.code))) {
    const group = groups.get(item.resource)
    if (group) {
      group.items.push(item)
      continue
    }
    groups.set(item.resource, {
      resource: item.resource,
      resourceDescription: item.resourceDescription,
      tone: permissionTone(item.resource),
      items: [item],
    })
  }
  return [...groups.values()].sort((a, b) =>
    a.resourceDescription.localeCompare(b.resourceDescription),
  )
}

function selectedPermissionGroup(groups: PermissionDisplayGroup[], selectedResource: string) {
  return groups.find((group) => group.resource === selectedResource) || groups[0] || null
}

function normalizeSelectedResource(selectedResource: string, groups: PermissionDisplayGroup[]) {
  if (groups.some((group) => group.resource === selectedResource)) return selectedResource
  return groups[0]?.resource || ''
}

function selectPermissionGroup(resource: string, kind: PermissionGroupKind) {
  if (kind === 'inherited') selectedInheritedResource.value = resource
  else selectedOverrideResource.value = resource
}

function isSelectedPermissionGroup(group: PermissionDisplayGroup, kind: PermissionGroupKind) {
  return kind === 'inherited'
    ? selectedInheritedResource.value === group.resource
    : selectedOverrideResource.value === group.resource
}

function permissionTone(resource: string): PermissionTone {
  const fixedTones: Record<string, PermissionTone> = {
    audit: 'sky',
    auth: 'violet',
    cache: 'cyan',
    invite: 'sky',
    media: 'sky',
    microsoft: 'violet',
    notification: 'amber',
    permission: 'rose',
    permission_audit: 'amber',
    permission_protected: 'rose',
    permission_role: 'emerald',
    profile: 'emerald',
    site: 'rose',
    texture: 'emerald',
    user: 'sky',
    wardrobe: 'emerald',
    wardrobe_item: 'emerald',
    yggdrasil: 'amber',
    yggdrasil_session: 'rose',
  }
  if (fixedTones[resource]) return fixedTones[resource]
  const fallbackTones: PermissionTone[] = ['emerald', 'sky', 'violet', 'amber', 'rose', 'cyan']
  const hash = [...resource].reduce((sum, char) => sum + char.charCodeAt(0), 0)
  return fallbackTones[hash % fallbackTones.length] || 'slate'
}

function roleTagClosable(role: PermissionRole) {
  if (role.id === 'user') return false
  if (props.isSelf && role.protected) return false
  if (role.protected && !canManageProtected.value) return false
  return canRevokePermission.value
}

function grantSelectedRole() {
  if (!selectedRoleId.value) return
  emit('grant-role', selectedRoleId.value)
  selectedRoleId.value = ''
}

function overrideEffect(code: string) {
  return overrideMap.value.get(code) || 'inherit'
}

function isProtectedPermission(row: PermissionDefinition) {
  return row.scope === 'system' || row.resource === 'permission_protected'
}

function permissionControlDisabled(row: PermissionDefinition) {
  if (props.isSelf && isProtectedPermission(row)) return true
  if (isProtectedPermission(row) && !canManageProtected.value) return true
  const current = overrideEffect(row.code)
  if (current === 'allow') return !canRevokePermission.value
  if (current === 'deny') return !canGrantPermission.value
  return !canGrantPermission.value && !canRevokePermission.value
}

function setSelectedPermission() {
  if (!selectedPermissionCode.value || !canAddSelectedPermission.value) return
  emit('set-permission', selectedPermissionCode.value, selectedPermissionEffect.value)
  selectedPermissionCode.value = ''
}
</script>

<style scoped>
.has-custom {
  background: transparent !important;
  border: none !important;
}

.has-custom :deep(img) {
  object-fit: contain;
}
</style>
