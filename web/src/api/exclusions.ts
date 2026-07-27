import client from './client'
import type { ApiResponse } from './types'

export interface ExclusionRule {
  id?: number
  target_site: string
  source_site: string
  created_at?: string
  is_hardcoded?: boolean
}

export const exclusionsApi = {
  list() {
    return client.get<ApiResponse<ExclusionRule[]>>('/publish/exclusions')
  },
  create(data: { target_site: string; source_site: string }) {
    return client.post<ApiResponse<void>>('/publish/exclusions', data)
  },
  remove(data: { target_site: string; source_site: string }) {
    return client.delete<ApiResponse<void>>('/publish/exclusions', { data })
  },
}

export interface ComplianceRule {
  id: number
  rule_type: 'adult' | 'forbidden_keyword' | 'forbidden_group' | 'site_blacklist_category'
  pattern: string
  category?: string
  site_code?: string
  scope: 'all' | 'publish' | 'reseed' | 'share' | 'download'
  source: 'builtin' | 'user'
  created_at?: string
}

export const complianceApi = {
  list() {
    return client.get<ApiResponse<{ items: ComplianceRule[]; total: number }>>('/compliance/rules')
  },
  create(data: { rule_type: string; pattern: string; scope?: string }) {
    return client.post<ApiResponse<ComplianceRule>>('/compliance/rules', data)
  },
  remove(id: number) {
    return client.delete<ApiResponse<{ deleted: number }>>(`/compliance/rules/${id}`)
  },
}
