import type { InjectionKey } from 'vue'

// A single source-of-truth background renderer.
// One requestAnimationFrame loop draws the crossfade onto the background
// canvas; glass buttons subscribe and copy a blurred crop of that exact
// frame, so the buttons can never drift from or jump ahead of the background.
export interface HeroSceneController {
  setTarget(canvas: HTMLCanvasElement | null): void
  setImages(urls: string[]): void
  subscribe(fn: () => void): () => void
  getCanvas(): HTMLCanvasElement | null
  getDpr(): number
  start(): void
  stop(): void
  destroy(): void
}

export const heroSceneKey: InjectionKey<HeroSceneController> = Symbol('heroScene')

export interface HeroSceneOptions {
  interval?: number
  transition?: number
  overlay?: string
}

export function createHeroScene(options: HeroSceneOptions = {}): HeroSceneController {
  const interval = options.interval ?? 5000
  const transition = options.transition ?? 800
  const overlay = options.overlay ?? 'rgba(0, 0, 0, 0.45)'

  let target: HTMLCanvasElement | null = null
  let ctx: CanvasRenderingContext2D | null = null
  let dpr = Math.max(window.devicePixelRatio || 1, 1)
  let cssW = 0
  let cssH = 0

  let images: string[] = []
  const loaded = new Map<string, HTMLImageElement>()

  let current = 0
  let next = 0
  let transitioning = false
  let transStart = 0
  let lastSwitch = 0

  let dirty = true
  let rafId = 0
  let running = false
  let listenersBound = false
  const consumers = new Set<() => void>()

  function loadImage(url: string) {
    if (loaded.has(url)) return
    const img = new Image()
    img.onload = () => {
      if (img.naturalWidth > 0) {
        loaded.set(url, img)
        dirty = true
      }
    }
    img.src = url
  }

  function preload() {
    for (const url of images) loadImage(url)
  }

  function ready(index: number): HTMLImageElement | null {
    const url = images[index]
    if (!url) return null
    return loaded.get(url) ?? null
  }

  function resize() {
    if (!target) return
    const rect = target.getBoundingClientRect()
    const w = Math.max(Math.ceil(rect.width), 1)
    const h = Math.max(Math.ceil(rect.height), 1)
    dpr = Math.max(window.devicePixelRatio || 1, 1)
    const pw = Math.ceil(w * dpr)
    const ph = Math.ceil(h * dpr)
    if (target.width !== pw || target.height !== ph) {
      target.width = pw
      target.height = ph
    }
    cssW = w
    cssH = h
    dirty = true
  }

  // Cover-fit an image into the viewport-sized canvas (mirrors object-fit: cover).
  function drawCover(c: CanvasRenderingContext2D, img: HTMLImageElement) {
    const scale = Math.max(cssW / img.naturalWidth, cssH / img.naturalHeight)
    const dw = img.naturalWidth * scale
    const dh = img.naturalHeight * scale
    const dx = (cssW - dw) / 2
    const dy = (cssH - dh) / 2
    c.drawImage(img, dx, dy, dw, dh)
  }

  function easeInOut(t: number) {
    return t < 0.5 ? 2 * t * t : 1 - Math.pow(-2 * t + 2, 2) / 2
  }

  function render(now: number) {
    if (!ctx || !target) return

    // Advance the crossfade state machine on the shared clock.
    let progress = 0
    if (images.length > 1) {
      if (!transitioning && now - lastSwitch >= interval) {
        next = (current + 1) % images.length
        if (ready(next)) {
          transitioning = true
          transStart = now
        } else {
          lastSwitch = now // wait for the image, retry next interval
        }
      }
      if (transitioning) {
        progress = Math.min((now - transStart) / transition, 1)
        if (progress >= 1) {
          current = next
          transitioning = false
          lastSwitch = now
        }
        dirty = true
      }
    }

    if (!dirty) return
    dirty = false

    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    ctx.clearRect(0, 0, cssW, cssH)

    const base = ready(current)
    if (base) {
      ctx.globalAlpha = 1
      drawCover(ctx, base)
      if (transitioning) {
        const incoming = ready(next)
        if (incoming) {
          ctx.globalAlpha = easeInOut(progress)
          drawCover(ctx, incoming)
          ctx.globalAlpha = 1
        }
      }
    } else {
      const g = ctx.createLinearGradient(0, 0, cssW, cssH)
      g.addColorStop(0, '#1a1a1a')
      g.addColorStop(1, '#333333')
      ctx.fillStyle = g
      ctx.fillRect(0, 0, cssW, cssH)
    }

    ctx.fillStyle = overlay
    ctx.fillRect(0, 0, cssW, cssH)

    for (const fn of consumers) fn()
  }

  function loop() {
    if (!running) return
    render(performance.now())
    rafId = requestAnimationFrame(loop)
  }

  function bindListeners() {
    if (listenersBound) return
    window.addEventListener('resize', resize)
    listenersBound = true
  }

  function unbindListeners() {
    if (!listenersBound) return
    window.removeEventListener('resize', resize)
    listenersBound = false
  }

  return {
    setTarget(canvas) {
      target = canvas
      ctx = canvas?.getContext('2d') ?? null
      if (canvas) resize()
    },
    setImages(urls) {
      images = urls.slice()
      current = 0
      next = 0
      transitioning = false
      lastSwitch = performance.now()
      preload()
      dirty = true
    },
    subscribe(fn) {
      consumers.add(fn)
      return () => consumers.delete(fn)
    },
    getCanvas: () => target,
    getDpr: () => dpr,
    start() {
      if (running) return
      running = true
      bindListeners()
      lastSwitch = performance.now()
      rafId = requestAnimationFrame(loop)
    },
    stop() {
      running = false
      cancelAnimationFrame(rafId)
    },
    destroy() {
      this.stop()
      unbindListeners()
      consumers.clear()
      loaded.clear()
      target = null
      ctx = null
    },
  }
}
