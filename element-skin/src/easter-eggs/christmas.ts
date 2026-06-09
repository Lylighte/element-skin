import type { EasterEggCleanup } from './index'

interface Flake {
  x: number
  y: number
  radius: number
  speed: number
  sway: number
  phase: number
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
  canvas.dataset.easterEgg = 'christmas'
  document.body.appendChild(canvas)

  let raf = 0
  let width = 0
  let height = 0
  let dpr = 1
  const flakes: Flake[] = []

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

  function resetFlake(flake: Flake, initial = false): void {
    flake.x = Math.random() * width
    flake.y = initial ? Math.random() * height : -12
    flake.radius = 1.2 + Math.random() * 2.8
    flake.speed = 0.35 + Math.random() * 0.9
    flake.sway = 0.4 + Math.random() * 1.4
    flake.phase = Math.random() * Math.PI * 2
  }

  function draw(now: number): void {
    ctx.clearRect(0, 0, width, height)
    ctx.fillStyle = 'rgba(255, 255, 255, 0.82)'
    for (const flake of flakes) {
      flake.y += flake.speed
      const x = flake.x + Math.sin(now / 900 + flake.phase) * flake.sway * 10
      if (flake.y - flake.radius > height) resetFlake(flake)
      ctx.beginPath()
      ctx.arc(x, flake.y, flake.radius, 0, Math.PI * 2)
      ctx.fill()
    }
    raf = requestAnimationFrame(draw)
  }

  resize()
  for (let i = 0; i < 80; i += 1) {
    const flake = {} as Flake
    resetFlake(flake, true)
    flakes.push(flake)
  }
  window.addEventListener('resize', resize)
  raf = requestAnimationFrame(draw)

  return () => {
    cancelAnimationFrame(raf)
    window.removeEventListener('resize', resize)
    canvas.remove()
  }
}
