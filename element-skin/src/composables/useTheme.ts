import { onUnmounted, ref } from 'vue'

import { appStorage } from '@/storage'

export function useTheme() {
  const isDark = ref(false)
  let mediaQuery: MediaQueryList | null = null

  function applyTheme() {
    document.documentElement.classList.toggle('dark', isDark.value)
  }

  function handlePreferenceChange(event: MediaQueryListEvent) {
    if (!appStorage.theme.hasUserPreference()) {
      isDark.value = event.matches
      applyTheme()
    }
  }

  function startSystemPreferenceWatcher() {
    if (mediaQuery || typeof window === 'undefined') return
    mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    mediaQuery.addEventListener('change', handlePreferenceChange)
  }

  function stopSystemPreferenceWatcher() {
    if (!mediaQuery) return
    mediaQuery.removeEventListener('change', handlePreferenceChange)
    mediaQuery = null
  }

  function initTheme() {
    const savedTheme = appStorage.theme.get()
    if (savedTheme) isDark.value = savedTheme === 'dark'
    else isDark.value = window.matchMedia('(prefers-color-scheme: dark)').matches
    applyTheme()
    startSystemPreferenceWatcher()
  }

  function toggleTheme() {
    isDark.value = !isDark.value
    appStorage.theme.set(isDark.value ? 'dark' : 'light')
    applyTheme()
  }

  onUnmounted(stopSystemPreferenceWatcher)

  return {
    isDark,
    initTheme,
    toggleTheme,
  }
}
