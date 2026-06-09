import type { EasterEggCleanup } from './index'

interface Bubble {
  x: number
  y: number
  radius: number
  speed: number
  drift: number
  color: string
}

const colors: [string, ...string[]] = ['#ff9ff3', '#feca57', '#48dbfb', '#1dd1a1', '#5f27cd']

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
  canvas.dataset.easterEgg = 'children-day'
  document.body.appendChild(canvas)

  let raf = 0
  let width = 0
  let height = 0
  let dpr = 1
  const bubbles: Bubble[] = []

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

  function resetBubble(bubble: Bubble, initial = false): void {
    bubble.x = Math.random() * width
    bubble.y = initial ? Math.random() * height : height + 24
    bubble.radius = 5 + Math.random() * 13
    bubble.speed = 0.25 + Math.random() * 0.65
    bubble.drift = -0.35 + Math.random() * 0.7
    bubble.color = colors[Math.floor(Math.random() * colors.length)] || colors[0]
  }

  function draw(): void {
    ctx.clearRect(0, 0, width, height)
    for (const bubble of bubbles) {
      bubble.y -= bubble.speed
      bubble.x += bubble.drift
      if (bubble.y + bubble.radius < 0 || bubble.x < -30 || bubble.x > width + 30) {
        resetBubble(bubble)
      }

      ctx.globalAlpha = 0.28
      ctx.beginPath()
      ctx.fillStyle = bubble.color
      ctx.arc(bubble.x, bubble.y, bubble.radius, 0, Math.PI * 2)
      ctx.fill()
      ctx.globalAlpha = 1
    }
    raf = requestAnimationFrame(draw)
  }

  resize()
  for (let i = 0; i < 38; i += 1) {
    const bubble = {} as Bubble
    resetBubble(bubble, true)
    bubbles.push(bubble)
  }
  window.addEventListener('resize', resize)
  raf = requestAnimationFrame(draw)

  return () => {
    cancelAnimationFrame(raf)
    window.removeEventListener('resize', resize)
    canvas.remove()
  }
}
