const maxPasswordBytes = 72

export function meetsPasswordPolicy(password: string, strong: boolean) {
  if (!password || new TextEncoder().encode(password).length > maxPasswordBytes) return false
  if (!strong) return true
  if ([...password].length < 8) return false

  const categories = [
    /\p{Ll}/u.test(password),
    /\p{Lu}/u.test(password),
    /\p{Nd}/u.test(password),
    /[\p{P}\p{S}]/u.test(password),
  ]
  return categories.filter(Boolean).length >= 2
}

export const genericPasswordPlaceholder = '请输入符合安全要求的密码'
export const genericPasswordError = '密码不符合安全要求，请调整后重试'
