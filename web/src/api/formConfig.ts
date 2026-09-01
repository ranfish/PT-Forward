import client from './client'
import type { ApiResponse } from './types'

// §59.156 切片 3: 站点发布配置中心（HTML 上传半自动）
export interface FormValueMapping {
  label: string
  value: string
  standard_keys?: string[]
  auto?: boolean | null
}

export interface PublishFormConfig {
  enabled: boolean
  framework?: string
  pre_audit_url?: string
  anonymous?: boolean
  form_fields?: Record<string, string>
  value_mappings?: Record<string, FormValueMapping[]>
  tag_config_legacy?: { mode?: string } | null
}

export interface FormConfigDiffItem {
  domain: string
  kind: 'matched' | 'changed' | 'added' | 'removed'
  label: string
  current_value?: string
  draft_value?: string
  current_keys?: string[]
  auto_false?: boolean
  field_rename?: string
}

export const formConfigApi = {
  get(siteName: string) {
    return client.get<ApiResponse<{ site_name: string; config: PublishFormConfig | null }>>(
      '/publish/form-config/get',
      { params: { site_name: siteName } },
    )
  },
  parse(siteName: string, html: string) {
    return client.post<
      ApiResponse<{ site_name: string; draft: PublishFormConfig; merged: PublishFormConfig; diffs: FormConfigDiffItem[] }>
    >('/publish/form-config/parse', { site_name: siteName, html })
  },
  setAnonymous(siteName: string, anonymous: boolean) {
    return client.post<ApiResponse<{ ok: boolean }>>('/publish/form-config/set-anonymous', {
      site_name: siteName,
      anonymous,
    })
  },
  apply(siteName: string, config: PublishFormConfig, note?: string) {
    return client.post<ApiResponse<{ ok: boolean }>>('/publish/form-config/apply', {
      site_name: siteName,
      config,
      note,
    })
  },
}

export interface ExecuteResult {
  status: string
  message: string
  pre_audit?: { passed: boolean; totalScore: number; details?: Array<{ ruleType: string; errorCode: string; message: string; level: string }> } | null
  form?: Record<string, string>
  tags?: string[]
  upload?: { torrent_id?: string; detail_url?: string } | null
  target_torrent_url?: string
}

export interface PublishTarget {
  name: string
  has_pre_audit: boolean
}

export const executeApi = {
  targets() {
    return client.get<ApiResponse<PublishTarget[]>>('/publish/form-config/targets')
  },
  execute(infoHash: string, targetSite: string, opts?: { dryRun?: boolean; tagOverrides?: string[]; anonymous?: boolean; pushOnly?: boolean; torrentId?: string; pushClientId?: string; pushSavePath?: string }) {
    return client.post<ApiResponse<{ result: ExecuteResult }>>('/publish/seeds/execute', {
      info_hash: infoHash,
      target_site: targetSite,
      dry_run: opts?.dryRun ?? true,
      tag_overrides: opts?.tagOverrides ?? [],
      anonymous: opts?.anonymous ?? false,
      push_only: opts?.pushOnly ?? false,
      torrent_id: opts?.torrentId ?? '',
      push_client_id: opts?.pushClientId ?? '',
      push_save_path: opts?.pushSavePath ?? '',
    })
  },
}
