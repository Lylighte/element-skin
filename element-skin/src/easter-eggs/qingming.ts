import type { EasterEggCleanup } from './index'

export function start(): EasterEggCleanup {
  const style = document.createElement('style')
  style.dataset.easterEgg = 'qingming'
  style.textContent = `
    html.easter-egg-qingming body {
      filter: grayscale(0.72) saturate(0.72);
    }

    html.easter-egg-qingming .surface-card,
    html.easter-egg-qingming .layout-header-wrap {
      box-shadow: none !important;
    }
  `
  document.head.appendChild(style)

  return () => {
    style.remove()
  }
}
