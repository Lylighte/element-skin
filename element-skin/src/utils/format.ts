export function formatUUID(uuid: string) {
  if (!uuid) return ''
  if (uuid.length === 32) {
    return `${uuid.slice(0, 8)}-${uuid.slice(8, 12)}-${uuid.slice(12, 16)}-${uuid.slice(16, 20)}-${uuid.slice(20)}`
  }
  return uuid
}
