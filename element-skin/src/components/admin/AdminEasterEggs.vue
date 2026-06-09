<template>
  <div class="easter-eggs-section animate-fade-in">
    <PageHeader title="彩蛋列表" subtitle="配置服务端允许启用的节日彩蛋">
      <template #icon><MagicStick /></template>
      <template #actions>
        <el-button :icon="Refresh" @click="loadSettings" class="hover-lift">
          重新加载
        </el-button>
        <el-button type="primary" :loading="saving" @click="saveSettings" class="hover-lift">
          保存
        </el-button>
      </template>
    </PageHeader>

    <el-alert
      class="mb-6"
      type="info"
      :closable="false"
      show-icon
      title="彩蛋启用规则"
      description="这里配置的是服务端允许启用的彩蛋。客户端还会结合本地日期和用户个人设置，三者都满足时才会 lazy import 对应效果。"
    />

    <div class="easter-egg-grid">
      <el-card
        v-for="egg in easterEggOptions"
        :key="egg.id"
        class="surface-card easter-egg-card"
        :class="{ active: enabledIds.includes(egg.id) }"
        shadow="never"
      >
        <div class="easter-egg-card-body">
          <div class="easter-egg-icon">
            <el-icon><MagicStick /></el-icon>
          </div>
          <div class="easter-egg-main">
            <div class="easter-egg-title-row">
              <h3>{{ egg.name }}</h3>
              <el-tag v-if="enabledIds.includes(egg.id)" type="success" effect="light">已启用</el-tag>
              <el-tag v-else type="info" effect="plain">未启用</el-tag>
            </div>
            <p>{{ egg.description }}</p>
            <div class="easter-egg-id">ID: {{ egg.id }}</div>
          </div>
          <el-switch
            :model-value="enabledIds.includes(egg.id)"
            @change="toggleEasterEgg(egg.id, Boolean($event))"
          />
        </div>
      </el-card>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { MagicStick, Refresh } from '@element-plus/icons-vue'
import { getAdminSettingsGroup, saveAdminSettingsGroup } from '@/api/admin/settings'
import PageHeader from '@/components/common/PageHeader.vue'
import { availableEasterEggs } from '@/easter-eggs'

const easterEggOptions = availableEasterEggs()
const enabledIds = ref<string[]>([])
const saving = ref(false)

async function loadSettings() {
  try {
    const res = await getAdminSettingsGroup('easter_eggs')
    const enabled = res.data.easter_eggs_enabled
    enabledIds.value = Array.isArray(enabled) ? enabled.filter((item): item is string => typeof item === 'string') : []
  } catch (e) {
    ElMessage.error('加载彩蛋设置失败')
  }
}

function toggleEasterEgg(id: string, enabled: boolean) {
  const exists = enabledIds.value.includes(id)
  if (enabled && !exists) {
    enabledIds.value = [...enabledIds.value, id]
  } else if (!enabled && exists) {
    enabledIds.value = enabledIds.value.filter((item) => item !== id)
  }
}

async function saveSettings() {
  saving.value = true
  try {
    await saveAdminSettingsGroup('easter_eggs', {
      easter_eggs_enabled: enabledIds.value,
    })
    ElMessage.success('彩蛋设置已更新')
  } catch (e) {
    ElMessage.error('保存彩蛋设置失败')
  } finally {
    saving.value = false
  }
}

onMounted(loadSettings)
</script>

<style scoped>
.easter-eggs-section {
  max-width: 1000px;
  margin: 0 auto;
  padding: 20px 0;
}

.easter-egg-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}

.easter-egg-card {
  transition: border-color 0.2s ease, box-shadow 0.2s ease, transform 0.2s ease;
}

.easter-egg-card.active {
  border-color: rgba(64, 158, 255, 0.45);
  box-shadow: 0 10px 26px rgba(64, 158, 255, 0.08);
}

.easter-egg-card:hover {
  transform: translateY(-2px);
}

.easter-egg-card-body {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  padding: 18px;
}

.easter-egg-icon {
  width: 42px;
  height: 42px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  background: linear-gradient(135deg, #409eff, #8553cf);
  flex-shrink: 0;
}

.easter-egg-main {
  flex: 1;
  min-width: 0;
}

.easter-egg-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 6px;
}

.easter-egg-title-row h3 {
  margin: 0;
  font-size: 17px;
  font-weight: 700;
  color: var(--color-heading);
}

.easter-egg-main p {
  margin: 0 0 10px;
  color: var(--color-text-light);
  line-height: 1.5;
}

.easter-egg-id {
  font-family: monospace;
  font-size: 12px;
  color: var(--color-text-light);
}

@media (max-width: 640px) {
  .easter-egg-grid {
    grid-template-columns: 1fr;
  }

  .easter-egg-card-body {
    align-items: center;
  }
}
</style>
