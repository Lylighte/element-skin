<template>
  <div ref="container" class="skin-viewer-container" :class="{ 'is-static': isStatic }">
    <!-- 静态模式下显示图片，非静态模式下由 JS 挂载 Canvas -->
    <img v-if="isStatic && snapshotUrl" :src="snapshotUrl" :style="{ width: width + 'px', height: height + 'px' }" class="skin-snapshot" />
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import * as skinview3d from 'skinview3d'

const props = defineProps({
  skinUrl: { type: String, required: true },
  capeUrl: { type: String, default: null },
  model: { type: String, default: 'default' },
  width: { type: Number, default: 300 },
  height: { type: Number, default: 400 },
  isStatic: { type: Boolean, default: false }
})

const container = ref(null)
const snapshotUrl = ref(null)
let viewer = null

async function initViewer() {
  if (viewer) {
    viewer.dispose()
    viewer = null
  }

  // 创建 Viewer 配置
  const config = {
    width: props.width,
    height: props.height,
    skin: props.skinUrl,
    cape: props.capeUrl,
    model: props.model === 'slim' ? 'slim' : 'steve',
    // 关键：为了导出图片，必须开启此选项
    preserveDrawingBuffer: true
  }

  if (props.isStatic) {
    // 1. 创建离屏或临时 Canvas 进行单次渲染
    const tempCanvas = document.createElement('canvas')
    viewer = new skinview3d.SkinViewer({
      canvas: tempCanvas,
      ...config
    })

    // 设置静态视角：完全正面 A-Pose，配合长焦远摄消除畸变
    viewer.autoRotate = false
    viewer.animation = null
    viewer.camera.position.set(0, 10, 500)
    viewer.camera.lookAt(0, 15, 0)
    viewer.zoom = 0.8

    // 手动调整 Pose 为 A-Pose (手臂微张)
    viewer.playerObject.skin.leftArm.rotation.z = 0.05
    viewer.playerObject.skin.rightArm.rotation.z = -0.05
    viewer.playerObject.skin.leftLeg.rotation.z = 0
    viewer.playerObject.skin.rightLeg.rotation.z = 0

    // 等待皮肤加载完成
    try {
      await viewer.loadSkin(props.skinUrl, { model: props.model === 'slim' ? 'slim' : 'steve' })
      if (props.capeUrl) await viewer.loadCape(props.capeUrl)
      
      // 渲染一帧
      viewer.render()
      
      // 2. 导出为 Data URL 并保存到响应式变量
      snapshotUrl.ref = tempCanvas.toDataURL('image/png')
      snapshotUrl.value = snapshotUrl.ref
      
      // 3. 彻底销毁 Viewer，释放 WebGL 上下文！！
      viewer.dispose()
      viewer = null
    } catch (e) {
      console.error('Failed to render static skin:', e)
    }
  } else {
    // 默认 3D 交互模式：挂载到 DOM
    const canvas = document.createElement('canvas')
    viewer = new skinview3d.SkinViewer({
      canvas: canvas,
      ...config
    })
    container.value.appendChild(viewer.canvas)
    
    viewer.autoRotate = true
    viewer.autoRotateSpeed = 0.5
    viewer.zoom = 0.8
    viewer.animation = new skinview3d.WalkingAnimation()
    viewer.animation.speed = 0.5
  }
}

onMounted(() => {
  if (props.skinUrl) initViewer()
})

onUnmounted(() => {
  if (viewer) viewer.dispose()
})

watch(() => [props.skinUrl, props.model, props.isStatic, props.capeUrl], () => {
  initViewer()
}, { deep: true })

</script>

<style scoped>
.skin-viewer-container {
  display: flex;
  justify-content: center;
  align-items: center;
  overflow: hidden;
}
.skin-snapshot {
  display: block;
  image-rendering: pixelated;
  object-fit: contain;
}
</style>
