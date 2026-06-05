import client from './client'
import type { ApiResponse } from './types'

export interface SupportedSite {
  domain: string
  name_cn: string
  framework: string
  auth_type: string
  cookiecloud_domain: string
  download_url_template: string
  passkey_label?: string
  passkey_hint?: string
  api_key_label?: string
  api_key_hint?: string
  rss_key_label?: string
  rss_key_hint?: string
  show_cookie?: boolean
  show_passkey?: boolean
  show_apikey?: boolean
  show_rsskey?: boolean
  show_bearer_token?: boolean
  show_authkey?: boolean
  show_authhash?: boolean
  show_userid?: boolean
  publish_form_fields?: Record<string, string>
  paths?: Record<string, string>
  verification_status: 'verified' | 'blocked'
  special_notes?: string
}

export interface SupportedSitesListResponse {
  items: SupportedSite[]
  total: number
}

// SiteDetail.vue / SiteList.vue 使用的字段覆盖（camelCase 适配层）
export interface SiteFieldOverride {
  showCookie?: boolean
  showPasskey?: boolean
  showApiKey?: boolean
  showBearerToken?: boolean
  showAuthKey?: boolean
  showAuthHash?: boolean
  showUserId?: boolean
  showRssKey?: boolean
  apiKeyLabel?: string
  apiKeyHint?: string
  rssKeyLabel?: string
  rssKeyHint?: string
  passkeyLabel?: string
  passkeyHint?: string
}

export const supportedSitesApi = {
  list(params?: { framework?: string; status?: string; search?: string }) {
    return client.get<ApiResponse<SupportedSitesListResponse>>('/supported-sites', { params })
  },
  get(domain: string) {
    return client.get<ApiResponse<SupportedSite>>(`/supported-sites/${domain}`)
  },
}

// 全局缓存：避免每个组件 onMounted 都重新请求
let _cache: SupportedSite[] | null = null
let _byDomain: Map<string, SupportedSite> | null = null

/** 拉取并缓存白名单；已缓存时直接返回。 */
export async function ensureSupportedSitesCache(): Promise<Map<string, SupportedSite>> {
  if (_cache && _byDomain) return _byDomain
  const resp = await supportedSitesApi.list()
  _cache = resp.data?.data?.items || []
  _byDomain = new Map(_cache.map(s => [s.domain, s]))
  return _byDomain
}

/** 清除缓存（测试用）。 */
export function clearSupportedSitesCache(): void {
  _cache = null
  _byDomain = null
}

/** 按 domain 查询字段覆盖；未加载缓存时返回 undefined。 */
export function getSiteFieldOverride(domain: string | undefined): SiteFieldOverride | undefined {
  if (!domain || !_byDomain) return undefined
  const s = _byDomain.get(domain)
  if (!s) return undefined
  return {
    showCookie: s.show_cookie,
    showPasskey: s.show_passkey,
    showApiKey: s.show_apikey,
    showBearerToken: s.show_bearer_token,
    showAuthKey: s.show_authkey,
    showAuthHash: s.show_authhash,
    showUserId: s.show_userid,
    showRssKey: s.show_rsskey,
    apiKeyLabel: s.api_key_label,
    apiKeyHint: s.api_key_hint,
    rssKeyLabel: s.rss_key_label,
    rssKeyHint: s.rss_key_hint,
    passkeyLabel: s.passkey_label,
    passkeyHint: s.passkey_hint,
  }
}
