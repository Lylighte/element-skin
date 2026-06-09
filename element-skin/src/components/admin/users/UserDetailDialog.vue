<template>
  <el-dialog
    v-model="visible"
    title=""
    class="dialog-viewer"
    destroy-on-close
    align-center
    append-to-body
  >
    <div v-if="user" class="user-detail-container">
      <div class="identity-panel mb-6">
        <el-avatar
          :size="80"
          :shape="user.avatar_hash ? 'square' : 'circle'"
          :class="user.avatar_hash ? 'has-custom' : 'avatar-fallback panel-avatar-base'"
          :src="userAvatars[user.avatar_hash || ''] || ''"
        >
          {{ !user.avatar_hash ? user.email.charAt(0).toUpperCase() : '' }}
        </el-avatar>
        <div class="panel-info">
          <div class="panel-name">
            <h3>{{ user.display_name || '未设置显示名' }}</h3>
            <el-tag v-if="user.is_admin" type="danger" size="small" class="ml-2">管理员</el-tag>
          </div>
          <div class="panel-email">{{ user.email }}</div>
          <div class="panel-id">UID: {{ user.id }}</div>
        </div>
        <div class="panel-status">
          <div v-if="isBanned" class="ban-info">
            <el-tag type="warning" effect="dark">
              <el-icon><Warning /></el-icon>
              封禁中
            </el-tag>
            <div class="ban-timer">{{ banRemaining }} 后解封</div>
          </div>
          <el-tag v-else type="success" effect="dark">
            <el-icon><CircleCheck /></el-icon>
            状态正常
          </el-tag>
        </div>
      </div>

      <el-tabs type="border-card" class="detail-tabs">
        <el-tab-pane label="角色列表">
          <el-table :data="profiles || []" size="small" max-height="300">
            <el-table-column prop="name" label="角色名称" />
            <el-table-column prop="model" label="模型" width="100">
              <template #default="{ row }">
                <el-tag size="small" :type="row.model === 'slim' ? 'success' : 'info'">{{ row.model }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="id" label="角色 UUID" width="300" />
          </el-table>
          <el-empty v-if="!profiles?.length" description="该用户暂无角色" :image-size="60" />
          <div class="pagination-container profile-pagination">
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

        <el-tab-pane label="危险操作">
          <div class="actions-grid">
            <div class="action-card-item">
              <div class="action-text-box">
                <div class="a-title">管理权限</div>
                <div class="a-desc">授予或撤销该用户的管理员权限。</div>
              </div>
              <el-button
                :type="user.is_admin ? 'warning' : 'primary'"
                :disabled="isSelf"
                class="hover-lift"
                @click="$emit('toggle-admin', user)"
              >
                {{ user.is_admin ? '撤销管理' : '设为管理' }}
              </el-button>
            </div>

            <div class="action-card-item">
              <div class="action-text-box">
                <div class="a-title">账号封禁</div>
                <div class="a-desc">暂时禁止该用户登录 Minecraft 客户端。</div>
              </div>
              <el-button
                v-if="!isBanned"
                type="warning"
                :disabled="user.is_admin || isSelf"
                class="hover-lift"
                @click="$emit('show-ban')"
              >
                执行封禁
              </el-button>
              <el-button v-else type="success" class="hover-lift" @click="$emit('unban', user)">
                解除封禁
              </el-button>
            </div>

            <div class="action-card-item">
              <div class="action-text-box">
                <div class="a-title">强制重置密码</div>
                <div class="a-desc">系统管理员手动为该用户设置新密码。</div>
              </div>
              <el-button class="hover-lift" @click="$emit('show-reset-password')">
                重置密码
              </el-button>
            </div>

            <div class="action-card-item dangerous">
              <div class="action-text-box">
                <div class="a-title">注销账号</div>
                <div class="a-desc">永久删除该用户及其所有关联的角色、皮肤。</div>
              </div>
              <el-button
                type="danger"
                :disabled="user.is_admin || isSelf"
                class="hover-lift"
                @click="$emit('delete-user', user)"
              >
                删除用户
              </el-button>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>
  </el-dialog>
</template>

<script setup lang="ts">
import { Warning, CircleCheck } from '@element-plus/icons-vue'
import CursorPager from '@/components/common/CursorPager.vue'
import type { Profile, User } from '@/api/types'

const visible = defineModel<boolean>('visible', { required: true })

defineProps<{
  user: User | null
  profiles: Profile[]
  userAvatars: Record<string, string>
  profilesLoading: boolean
  profilesPrevDisabled: boolean
  profilesNextDisabled: boolean
  isBanned: boolean
  banRemaining: string
  isSelf: boolean
}>()

defineEmits<{
  'profiles-prev': []
  'profiles-next': []
  'toggle-admin': [user: User]
  'show-ban': []
  unban: [user: User]
  'show-reset-password': []
  'delete-user': [user: User]
}>()
</script>

<style scoped>
.user-detail-container {
  padding: 24px;
}

.identity-panel {
  display: flex;
  align-items: center;
  gap: 24px;
  padding: 20px;
  background: var(--color-background-soft);
  border-radius: 12px;
}

.has-custom,
.el-avatar.has-custom {
  background: transparent !important;
  border: none !important;
  box-shadow: none !important;
}

.has-custom :deep(img) {
  object-fit: contain;
}

.avatar-fallback {
  background-color: var(--color-background-mute) !important;
  color: var(--color-text-light) !important;
}

.panel-avatar-base {
  font-weight: bold;
  border: 2px solid #fff;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.panel-info {
  flex: 1;
}

.panel-name {
  display: flex;
  align-items: center;
  gap: 8px;
}

.panel-name h3 {
  margin: 0;
  font-size: 20px;
  color: var(--color-heading);
}

.panel-email {
  color: var(--color-text-light);
  margin-top: 4px;
}

.panel-id {
  font-size: 11px;
  font-family: monospace;
  color: var(--color-text-light);
  margin-top: 4px;
}

.panel-status {
  text-align: right;
}

.ban-timer {
  font-size: 12px;
  color: var(--el-color-warning);
  margin-top: 4px;
}

.profile-pagination {
  margin-top: 10px;
}

.actions-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  padding: 10px 0;
}

.action-card-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  background: var(--color-background-soft);
  border-radius: 10px;
  border: 1px solid var(--color-border);
}

.action-card-item.dangerous {
  border-color: rgba(245, 108, 108, 0.3);
}

.action-text-box {
  flex: 1;
  margin-right: 12px;
}

.a-title {
  font-weight: 600;
  font-size: 14px;
  color: var(--color-heading);
}

.a-desc {
  font-size: 12px;
  color: var(--color-text-light);
  margin-top: 2px;
}

@media (max-width: 768px) {
  .identity-panel {
    align-items: flex-start;
    flex-direction: column;
  }

  .panel-status {
    text-align: left;
  }

  .actions-grid {
    grid-template-columns: 1fr;
  }
}
</style>
