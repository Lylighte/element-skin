import { ref } from 'vue'
import { getNotices } from '@/api/notices'

const hasUnreadNotifications = ref(false)
const loadingUnreadNotifications = ref(false)

let refreshPromise: Promise<void> | null = null

async function refreshUnreadNotifications() {
  if (refreshPromise) return refreshPromise

  loadingUnreadNotifications.value = true
  refreshPromise = getNotices({
    limit: 1,
    include_read: false,
  })
    .then((res) => {
      hasUnreadNotifications.value =
        res.data.page_size > 0 || res.data.has_next || res.data.items.length > 0
    })
    .catch(() => {
      hasUnreadNotifications.value = false
    })
    .finally(() => {
      loadingUnreadNotifications.value = false
      refreshPromise = null
    })

  return refreshPromise
}

function clearUnreadNotifications() {
  hasUnreadNotifications.value = false
}

export function useNotificationIndicator() {
  return {
    hasUnreadNotifications,
    loadingUnreadNotifications,
    refreshUnreadNotifications,
    clearUnreadNotifications,
  }
}
