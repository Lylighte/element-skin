<template>
  <div ref="container" class="cape-viewer-container" :class="{ 'is-static': isStatic }">
    <img v-if="isStatic && snapshotUrl" :src="snapshotUrl" :style="{ width: width + 'px', height: height + 'px' }" class="cape-snapshot" />
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, watch } from 'vue'
import * as skinview3d from 'skinview3d'

const props = defineProps({
  capeUrl: { type: String, required: true },
  width: { type: Number, default: 200 },
  height: { type: Number, default: 280 },
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

  const config = {
    width: props.width,
    height: props.height,
    skin: null,
    cape: props.capeUrl,
    preserveDrawingBuffer: true
  }

  if (props.isStatic) {
    const tempCanvas = document.createElement('canvas')
    viewer = new skinview3d.SkinViewer({
      canvas: tempCanvas,
      ...config
    })

    if (viewer.playerObject) viewer.playerObject.skin.visible = false
    
    viewer.autoRotate = false
    viewer.camera.position.set(0, 10, -50)
    viewer.camera.lookAt(0, 15, 0)
    viewer.zoom = 1.3

    try {
      await viewer.loadCape(props.capeUrl)
      viewer.render()
      snapshotUrl.value = tempCanvas.toDataURL('image/png')
      viewer.dispose()
      viewer = null
    } catch (e) {
      console.error('Failed to render static cape:', e)
    }
  } else {
    const canvas = document.createElement('canvas')
    viewer = new skinview3d.SkinViewer({
      canvas: canvas,
      ...config
    })
    container.value.appendChild(viewer.canvas)
    if (viewer.playerObject) viewer.playerObject.skin.visible = false
    viewer.autoRotate = true
    viewer.autoRotateSpeed = 0.5
    viewer.zoom = 1.2
  }
}

onMounted(() => {
  if (props.capeUrl) initViewer()
})

onUnmounted(() => {
  if (viewer) viewer.dispose()
})

watch(() => [props.capeUrl, props.isStatic], () => {
  initViewer()
}, { deep: true })

</script>

<style scoped>
.cape-viewer-container {
  display: flex;
  justify-content: center;
  align-items: center;
  overflow: hidden;
}
.cape-snapshot {
  display: block;
  image-rendering: pixelated;
  object-fit: contain;
}
</style>
