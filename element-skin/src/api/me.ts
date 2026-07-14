import client from './client'
import type { User } from './types'

export function getMe(): Promise<{ data: User }> {
  return client.get('/v1/users/me')
}

export function patchMe(data: {
  display_name?: string
  preferred_language?: string
  avatar_hash?: string | null
}): Promise<{ data: { ok: boolean } }> {
  return client.patch('/v1/users/me', data)
}

export function sendEmailChangeCode(data: {
  email: string
}): Promise<{ data: { ok: boolean; ttl: number } }> {
  return client.post('/v1/users/me/email/verification-code', data)
}

export function changeEmail(data: {
  email: string
  code: string
}): Promise<{ data: { ok: boolean } }> {
  return client.put('/v1/users/me/email', data)
}

export function deleteMe(): Promise<{ data: { ok: boolean } }> {
  return client.delete('/v1/users/me')
}

export function changePassword(data: {
  old_password: string
  new_password: string
}): Promise<{ data: { ok: boolean; message: string } }> {
  return client.post('/v1/users/me/password', data)
}
