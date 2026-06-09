import type { Router } from 'vue-router'

export type EasterEggCleanup = () => void

export interface EasterEggModule {
  start: () => void | EasterEggCleanup | Promise<void | EasterEggCleanup>
}

interface EasterEggDefinition {
  id: string
  name: string
  description: string
  htmlClass?: string
  active: (date: Date) => boolean
  load: () => Promise<EasterEggModule>
}

export interface EasterEggConfig {
  enabled?: string[]
}

const EASTER_EGG_DISABLED_KEY = 'disableEasterEgg'
const LEGACY_EASTER_EGG_DISABLED_KEY = 'disableMeowEasterEgg'

const definitions: EasterEggDefinition[] = [
  {
    id: 'april-fools',
    name: '愚人节',
    description: '4 月 1 日启用点击元素物理效果。',
    htmlClass: 'easter-egg-april-fools',
    active: (date) => date.getMonth() === 3 && date.getDate() === 1,
    load: () => import('./aprilFools'),
  },
  {
    id: 'qingming',
    name: '清明',
    description: '4 月 4 日至 4 月 5 日启用低饱和静默主题。',
    htmlClass: 'easter-egg-qingming',
    active: (date) => date.getMonth() === 3 && date.getDate() >= 4 && date.getDate() <= 5,
    load: () => import('./qingming'),
  },
  {
    id: 'children-day',
    name: '儿童节',
    description: '6 月 1 日启用轻量彩色气泡效果。',
    htmlClass: 'easter-egg-children-day',
    active: (date) => date.getMonth() === 5 && date.getDate() === 1,
    load: () => import('./childrenDay'),
  },
  {
    id: 'christmas',
    name: '圣诞节',
    description: '12 月 24 日至 12 月 25 日启用飘雪效果。',
    htmlClass: 'easter-egg-christmas',
    active: (date) => date.getMonth() === 11 && date.getDate() >= 24 && date.getDate() <= 25,
    load: () => import('./christmas'),
  },
]

let activeCleanup: EasterEggCleanup | null = null
let activeClass: string | null = null
let runToken = 0
let serverConfig: EasterEggConfig | null = null

function hasDOM(): boolean {
  return typeof window !== 'undefined' && typeof document !== 'undefined'
}

function stopActive(): void {
  if (activeCleanup) {
    activeCleanup()
    activeCleanup = null
  }
  if (activeClass) {
    document.documentElement.classList.remove(activeClass)
    activeClass = null
  }
}

export function isEasterEggDisabled(): boolean {
  if (!hasDOM()) return true
  return localStorage.getItem(EASTER_EGG_DISABLED_KEY) === '1' || localStorage.getItem(LEGACY_EASTER_EGG_DISABLED_KEY) === '1'
}

export function setEasterEggDisabled(disabled: boolean): void {
  if (!hasDOM()) return
  if (disabled) {
    localStorage.setItem(EASTER_EGG_DISABLED_KEY, '1')
    localStorage.removeItem(LEGACY_EASTER_EGG_DISABLED_KEY)
    cleanupEasterEgg()
    return
  }
  localStorage.removeItem(EASTER_EGG_DISABLED_KEY)
  localStorage.removeItem(LEGACY_EASTER_EGG_DISABLED_KEY)
  void refreshEasterEgg()
}

export function availableEasterEggs(): Array<Pick<EasterEggDefinition, 'id' | 'name' | 'description'>> {
  return definitions.map(({ id, name, description }) => ({ id, name, description }))
}

export function activeEasterEggFor(date = new Date()): EasterEggDefinition | null {
  const enabled = serverConfig?.enabled
  return definitions.find((definition) => {
    if (enabled && !enabled.includes(definition.id)) return false
    return definition.active(date)
  }) || null
}

export function setServerEasterEggConfig(config?: EasterEggConfig | null): void {
  serverConfig = config || null
  void refreshEasterEgg()
}

function resolveEasterEgg(date: Date): EasterEggDefinition | null {
  return activeEasterEggFor(date)
}

export function cleanupEasterEgg(): void {
  runToken += 1
  if (!hasDOM()) return
  stopActive()
}

export async function refreshEasterEgg(date = new Date()): Promise<void> {
  if (!hasDOM()) return

  const token = runToken + 1
  runToken = token
  stopActive()

  if (isEasterEggDisabled()) return

  const definition = resolveEasterEgg(date)
  if (!definition) return

  try {
    const mod = await definition.load()
    if (token !== runToken) return

    if (definition.htmlClass) {
      document.documentElement.classList.add(definition.htmlClass)
      activeClass = definition.htmlClass
    }

    const cleanup = await mod.start()
    if (token !== runToken) {
      if (cleanup) cleanup()
      return
    }
    activeCleanup = cleanup || null
  } catch (error) {
    console.warn(`Failed to start easter egg "${definition.id}":`, error)
    if (token === runToken) stopActive()
  }
}

export function installEasterEggRouterHooks(router: Router): void {
  router.beforeEach(() => {
    cleanupEasterEgg()
  })
  router.afterEach(() => {
    void refreshEasterEgg()
  })
}
