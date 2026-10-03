import { describe, expect, it } from 'vitest'
import fixtures from '../../../../testdata/password-policy.json'
import { meetsPasswordPolicy } from '../passwordPolicy'

describe('password policy', () => {
  it.each(fixtures)('$name with strong validation enabled', ({ password, strong_errors }) => {
    expect(meetsPasswordPolicy(password, true)).toBe(strong_errors.length === 0)
  })

  it.each(fixtures)('$name with strong validation disabled', ({ password, basic_errors }) => {
    expect(meetsPasswordPolicy(password, false)).toBe(basic_errors.length === 0)
  })
})
