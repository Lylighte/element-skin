import {
  changeEmail,
  changePassword,
  deleteMe,
  getMe,
  patchMe,
  sendEmailChangeCode,
} from '../../me'
import type { ApiCase } from './types'

export function meApiCases(): ApiCase[] {
  return [
    { name: 'getMe gets /me', method: 'get', call: getMe, args: ['/v1/users/me'] },
    {
      name: 'patchMe patches profile fields',
      method: 'patch',
      call: () => patchMe({ display_name: 'Display', avatar_hash: null }),
      args: ['/v1/users/me', { display_name: 'Display', avatar_hash: null }],
    },
    { name: 'deleteMe deletes /me', method: 'delete', call: deleteMe, args: ['/v1/users/me'] },
    {
      name: 'changePassword posts password payload',
      method: 'post',
      call: () =>
        changePassword({ old_password: 'OldPassword123', new_password: 'NewPassword123' }),
      args: [
        '/v1/users/me/password',
        { old_password: 'OldPassword123', new_password: 'NewPassword123' },
      ],
    },
    {
      name: 'sendEmailChangeCode posts the new email',
      method: 'post',
      call: () => sendEmailChangeCode({ email: 'new@example.com' }),
      args: ['/v1/users/me/email/verification-code', { email: 'new@example.com' }],
    },
    {
      name: 'changeEmail puts the verified email and code',
      method: 'put',
      call: () => changeEmail({ email: 'new@example.com', code: 'EMAIL123' }),
      args: ['/v1/users/me/email', { email: 'new@example.com', code: 'EMAIL123' }],
    },
  ]
}
