<script setup lang="ts">
import { provide, ref, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { User } from '@element-plus/icons-vue'
import { getPublicSettings, getPublicHomepageMedia } from '@/api/public'
import { getMe } from '@/api/me'
import CanvasGlassButton from '@/components/common/CanvasGlassButton.vue'
import { createHeroScene, heroSceneKey } from '@/composables/useHeroScene'

const router = useRouter()
const siteName = ref(localStorage.getItem('site_name_cache') || '皮肤站')
const siteSubtitle = ref(localStorage.getItem('site_subtitle_cache') || '简洁、高效、现代的 Minecraft 皮肤 management 站')
const isLogged = ref(false)
const bgCanvasRef = ref<HTMLCanvasElement | null>(null)

// Single source-of-truth renderer for the hero background. The buttons sample
// blurred crops from its canvas, so they stay in lockstep with the crossfade.
const scene = createHeroScene()
provide(heroSceneKey, scene)

onMounted(async () => {
  scene.setTarget(bgCanvasRef.value)
  scene.start()

  // 加载站点配置
  try {
    const res = await getPublicSettings()
    if (res.data.site_name) {
      siteName.value = res.data.site_name
      localStorage.setItem('site_name_cache', res.data.site_name)
    }
    if (res.data.site_subtitle) {
      siteSubtitle.value = res.data.site_subtitle
      localStorage.setItem('site_subtitle_cache', res.data.site_subtitle)
    }
  } catch (e) {
    console.warn('Failed to load site settings:', e)
  }

  // 加载首页媒体
  try {
    const res = await getPublicHomepageMedia()
    scene.setMedia(res.data)
  } catch (e) {
    console.warn('Failed to load homepage media:', e)
  }

  // 检查登录状态（cookie 自动携带）
  try {
    await getMe()
    isLogged.value = true
  } catch {}
})

onBeforeUnmount(() => {
  scene.destroy()
})

function goDashboard() { router.push('/dashboard') }
function goLogin() { router.push('/login') }
function goRegister() { router.push('/register') }

</script>

<template>
  <div class="home-container">
    <!-- Background is FIXED and outside of main content flow -->
    <canvas ref="bgCanvasRef" class="hero-bg-fixed" aria-hidden="true"></canvas>

    <!-- Main Content -->
    <div class="hero-section">
      <div class="hero-content animate-fade-in">
        <h1 class="hero-title">{{ siteName }}</h1>
        <p class="hero-subtitle">{{ siteSubtitle }}</p>
        <div class="hero-actions">
          <CanvasGlassButton
            v-if="isLogged"
            class="hero-btn"
            variant="primary"
            @click="goDashboard"
          >
            <el-icon><User /></el-icon>
            <span>进入个人面板</span>
          </CanvasGlassButton>
          <template v-else>
            <CanvasGlassButton class="hero-btn" variant="primary" @click="goLogin">
              登录账号
            </CanvasGlassButton>
            <CanvasGlassButton class="hero-btn" variant="secondary" @click="goRegister">
              即刻注册
            </CanvasGlassButton>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.home-container {
  width: 100%;
  height: calc(100vh - var(--footer-height, 0px));
  display: flex;
  flex-direction: column;
  position: relative;
}

/* FIXED Background logic — single canvas, drawn by the hero scene */
.hero-bg-fixed {
  position: fixed; top: 0; left: 0;
  width: 100vw; height: 100vh;
  z-index: 0;
  display: block;
}

.hero-section {
  position: relative; z-index: 1; flex: 1; display: flex; align-items: center; justify-content: center; color: #fff; padding: 0 20px;
}

.hero-content { text-align: center; max-width: 800px; }
.hero-title { font-size: 56px; font-weight: 800; margin: 0 0 16px 0; letter-spacing: -1.5px; text-shadow: 0 2px 10px rgba(0,0,0,0.3); }
.hero-subtitle { font-size: 20px; margin: 0 0 32px 0; opacity: 0.95; font-weight: 400; }

.hero-actions { display: flex; gap: 16px; justify-content: center; }
.hero-btn { height: 52px; padding: 0 36px; font-size: 16px; font-weight: 600; border-radius: 14px; }

@media (max-width: 768px) {
  .hero-title { font-size: 36px; }
  .hero-actions { flex-direction: column; gap: 12px; }
  .hero-btn { width: 100%; }
}
</style>
