import client from './client'
import type { ApiResponse, ApiResponsePaginated, FilterRule, UpdatePartial } from './types'

export const filterRulesApi = {
  list(page = 1, size = 20) {
    return client.get<ApiResponsePaginated<FilterRule>>('/filters/rules', { params: { page, size } })
  },
  create(data: UpdatePartial<FilterRule>) {
    return client.post<ApiResponse<FilterRule>>('/filters/rules', data)
  },
  update(id: number, data: UpdatePartial<FilterRule>) {
    return client.put<ApiResponse<FilterRule>>(`/filters/rules/${id}`, data)
  },
  delete(id: number) {
    return client.delete<ApiResponse<void>>(`/filters/rules/${id}`)
  },
  test(id: number, data?: { torrentName?: string; size?: number; [key: string]: unknown }) {
    return client.post<ApiResponse<{ passed: boolean; matchedConditions?: string[] }>>(`/filters/rules/${id}/test`, data || {})
  },
}
