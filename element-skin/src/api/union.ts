import client from './union-client'

// Union profile (role binding entry)
export interface UnionProfile {
  id: string
  name: string
  self: any
  dup_name: any[]
  _token_input?: string
}

// Admin union settings
export interface UnionSettings {
  union_api_root: string
  union_member_key: string
  union_enable_update: string
  union_enable_oauth2: string
  union_oauth2_sig_private_key: string
  union_oauth2_sig_public_key: string
  union_server_list_version: number
  union_private_key_version: number
  union_ygg_private_key_fingerprint?: string
  union_ygg_private_key_present?: boolean
  union_server_list: any[]
}

// --- Dashboard functions ---

export function getUnionProfiles() {
  return client.get<{ items: UnionProfile[] }>('/union/profiles')
}

export function bindUnionProfile(uuid: string) {
  return client.post<{ token: string }>('/union/bind', { uuid })
}

export function unbindUnionProfile(uuid: string) {
  return client.post('/union/unbind', { uuid })
}

export function bindToUnionProfile(uuid: string, token: string) {
  return client.post('/union/bindto', { uuid, token })
}

export function remapUnionUUID(me: string, target: string) {
  return client.post('/union/remapuuid', { me, target })
}

export function getUnionSecurityLevel() {
  return client.get<{ security_level: number }>('/union/security/level')
}

// --- Admin functions ---

export function getAdminUnionSettings() {
  return client.get<UnionSettings>('/admin/union/settings')
}

export function saveAdminUnionSettings(settings: Record<string, any>) {
  return client.post('/admin/union/settings', settings)
}

export function generateUnionKeypair() {
  return client.post<{ privateKey: string; publicKey: string }>('/admin/union/generate-keypair')
}

export function updateUnionServerList() {
  return client.post('/admin/union/update-list')
}

export function updateUnionPrivateKey() {
  return client.post('/admin/union/update-key')
}

export function syncUnionProfiles() {
  return client.post('/admin/union/sync')
}

export function diagnoseUnion() {
  return client.post<{ status: string; data: any }>('/admin/union/diagnose')
}

export function getUnionBlacklist(params?: { q?: string; page?: number }) {
  return client.get('/admin/union/blacklist', { params })
}

export function createUnionBlacklist(email: string, reason?: string) {
  return client.post('/admin/union/blacklist', { email, reason })
}

export function invalidateUnionBlacklist(entryId: string) {
  return client.post(`/admin/union/blacklist/${entryId}/invalidate`)
}

export function deleteUnionBlacklist(entryId: string) {
  return client.delete(`/admin/union/blacklist/${entryId}`)
}
