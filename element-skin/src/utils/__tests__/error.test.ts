import { describe, expect, it } from 'vitest'

import { getErrorMessage } from '../error'

describe('getErrorMessage', () => {
  it('prefers detail over OAuth error description', () => {
    expect(
      getErrorMessage({
        response: {
          data: {
            detail: 'permission denied',
            error_description: 'invalid refresh_token',
          },
        },
      }),
    ).toBe('permission denied')
  })

  it('uses OAuth error description when detail is absent', () => {
    expect(
      getErrorMessage({
        response: {
          data: {
            error_description: 'invalid refresh_token',
          },
        },
      }),
    ).toBe('invalid refresh_token')
  })
})
