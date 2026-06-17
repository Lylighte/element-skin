<template>
  <UiDialog v-model="visible" destroy-on-close variant="viewer">
    <UiViewerLayout v-if="profile">
      <template #stage>
        <SkinViewer
          v-if="profile.skin_hash"
          :skin-url="texturesUrl(profile.skin_hash)"
          :cape-url="profile.cape_hash ? texturesUrl(profile.cape_hash) : null"
          :model="profile.texture_model || profile.model || 'default'"
          :width="320"
          :height="430"
        />
        <el-empty v-else description="未设置皮肤" />
      </template>

      <div class="flex min-h-0 flex-1 flex-col">
        <section class="border-b border-[var(--color-border)] py-3.5">
          <el-input
            v-model="name"
            placeholder="角色名称"
            @blur="$emit('rename')"
            @keyup.enter="$emit('rename')"
          />
        </section>

        <section class="border-b border-[var(--color-border)] py-3.5">
          <div
            class="mb-2.5 text-xs font-bold uppercase tracking-[0.5px] text-[var(--color-text-light)]"
          >
            皮肤绑定
          </div>
          <el-input :model-value="profile.skin_hash || '未绑定'" disabled>
            <template #append>
              <el-button :disabled="!profile.skin_hash" @click="$emit('clear-skin')"
                >清除</el-button
              >
            </template>
          </el-input>
        </section>

        <section class="border-b border-[var(--color-border)] py-3.5">
          <div
            class="mb-2.5 text-xs font-bold uppercase tracking-[0.5px] text-[var(--color-text-light)]"
          >
            披风绑定
          </div>
          <el-input :model-value="profile.cape_hash || '未绑定'" disabled>
            <template #append>
              <el-button :disabled="!profile.cape_hash" @click="$emit('clear-cape')"
                >清除</el-button
              >
            </template>
          </el-input>
        </section>

        <section class="mt-auto py-3.5">
          <el-button type="danger" plain class="w-full rounded-lg" @click="$emit('delete')">
            删除角色
          </el-button>
        </section>
      </div>
    </UiViewerLayout>
  </UiDialog>
</template>

<script setup lang="ts">
import type { Profile } from '@/api/types'
import SkinViewer from '@/components/SkinViewer.vue'
import UiDialog from '@/components/ui/UiDialog.vue'
import UiViewerLayout from '@/components/ui/UiViewerLayout.vue'

const visible = defineModel<boolean>('visible', { required: true })
const name = defineModel<string>('name', { required: true })

defineProps<{
  profile: Profile | null
  texturesUrl: (hash: string | null | undefined) => string
}>()

defineEmits<{
  rename: []
  'clear-skin': []
  'clear-cape': []
  delete: []
}>()
</script>
