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
  apply(siteName: string, config: PublishFormConfig, note?: string) {
    return client.post<ApiResponse<{ ok: boolean }>>('/publish/form-config/apply', {
      site_name: siteName,
      config,
      note,
    })
  },
}
