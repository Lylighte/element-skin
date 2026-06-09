/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<{}, {}, any>
  export default component
}

declare const __APP_VERSION__: string

// matter-js ships no type declarations and the easter egg only touches a small
// runtime API surface.
declare module 'matter-js' {
  const Matter: any
  export default Matter
}

interface Window {
  elementSkinEasterEggs?: {
    list: () => Array<{ id: string; name: string; description: string }>
    start: (id: string) => Promise<boolean>
    stop: () => void
    refreshAt: (date: string | Date) => Promise<void>
    setDisabled: (disabled: boolean) => void
  }
}
