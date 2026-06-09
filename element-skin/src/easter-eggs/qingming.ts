import type { EasterEggCleanup } from './index'

interface Raindrop {
  x: number
  y: number
  length: number
  speed: number
  alpha: number
}

export function start(): EasterEggCleanup {
  const canvas = document.createElement('canvas')
  const context = canvas.getContext('2d')
  if (!context) return () => canvas.remove()
  const ctx = context

  canvas.style.position = 'fixed'
  canvas.style.inset = '0'
  canvas.style.width = '100vw'
  canvas.style.height = '100vh'
  canvas.style.pointerEvents = 'none'
  canvas.style.zIndex = '2147483000'
  canvas.dataset.easterEgg = 'qingming'
  document.body.appendChild(canvas)

  let raf = 0
  let width = 0
  let height = 0
  let dpr = 1
  const drops: Raindrop[] = []

  function resize(): void {
    dpr = Math.max(1, Math.min(window.devicePixelRatio || 1, 2))
    width = window.innerWidth
    height = window.innerHeight
    canvas.width = Math.floor(width * dpr)
    canvas.height = Math.floor(height * dpr)
    canvas.style.width = `${width}px`
    canvas.style.height = `${height}px`
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  }

  function resetDrop(drop: Raindrop, initial = false): void {
    drop.x = Math.random() * width
    drop.y = initial ? Math.random() * height : -24
    drop.length = 12 + Math.random() * 26
    drop.speed = 5 + Math.random() * 7
    drop.alpha = 0.16 + Math.random() * 0.22
  }

  function draw(): void {
    ctx.clearRect(0, 0, width, height)
    ctx.lineWidth = 1
    ctx.lineCap = 'round'

    for (const drop of drops) {
      drop.x -= drop.speed * 0.28
      drop.y += drop.speed
      if (drop.y - drop.length > height || drop.x < -40) {
        resetDrop(drop)
        drop.x = Math.random() * (width + 80)
      }

      ctx.strokeStyle = `rgba(128, 156, 176, ${drop.alpha})`
      ctx.beginPath()
      ctx.moveTo(drop.x, drop.y)
      ctx.lineTo(drop.x - drop.length * 0.32, drop.y + drop.length)
      ctx.stroke()
    }

    raf = requestAnimationFrame(draw)
  }

  resize()
  for (let i = 0; i < 110; i += 1) {
    const drop = {} as Raindrop
    resetDrop(drop, true)
    drops.push(drop)
  }
  window.addEventListener('resize', resize)
  raf = requestAnimationFrame(draw)

  return () => {
    cancelAnimationFrame(raf)
    window.removeEventListener('resize', resize)
    canvas.remove()
  }
}
