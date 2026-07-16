import type { WhitelistEntry } from '@/api/types'

export interface FallbackRow {
  id: number | null
  rowKey: string | number
  priority: number
  session_url: string
  account_url: string
  services_url: string
  cache_ttl: number
  enable_profile: boolean
  enable_hasjoined: boolean
  enable_whitelist: boolean
  note: string
  skin_domains_text: string
  _whitelist: WhitelistEntry[]
  _initialWhitelist: WhitelistEntry[]
  _new_user: string
  _loaded: boolean
}

export interface FallbackEndpoint {
  id: number | null
  priority: number
  session_url: string
  account_url: string
  services_url: string
  cache_ttl: number
  enable_profile: boolean
  enable_hasjoined: boolean
  enable_whitelist: boolean
  note?: string
  skin_domains: string[]
}
