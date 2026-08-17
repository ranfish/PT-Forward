import client from './client'
import type { ApiResponse, ApiResponsePaginated, RSSSubscription, RuleCondition, UpdatePartial } from './types'

export const subscriptionsApi = {
  list(page = 1, size = 20) {
    return client.get<ApiResponsePaginated<RSSSubscription>>('/rss/subscriptions', { params: { page, size } })
  },
  get(id: number) {
    return client.get<ApiResponse<RSSSubscription>>(`/rss/subscriptions/${id}`)
  },
  create(data: UpdatePartial<RSSSubscription>) {
    return client.post<ApiResponse<RSSSubscription>>('/rss/subscriptions', data)
  },
  update(id: number, data: UpdatePartial<RSSSubscription>) {
    return client.put<ApiResponse<RSSSubscription>>(`/rss/subscriptions/${id}`, data)
  },
  delete(id: number) {
    return client.delete<ApiResponse<void>>(`/rss/subscriptions/${id}`)
  },
  pause(id: number) {
    return client.post<ApiResponse<void>>(`/rss/subscriptions/${id}/pause`)
  },
  resume(id: number) {
    return client.post<ApiResponse<void>>(`/rss/subscriptions/${id}/resume`)
  },
  trigger(id: number) {
    return client.post<ApiResponse<void>>(`/rss/subscriptions/${id}/trigger`)
  },
  dryrun(id: number) {
    return client.post<ApiResponse<Record<string, unknown>>>(`/rss/subscriptions/${id}/dryrun`)
  },
  updateRules(id: number, data: { conditions: RuleCondition[] }) {
    return client.put<ApiResponse<RSSSubscription>>(`/rss/subscriptions/${id}/rules`, data)
  },
  // §59.33: 清 seen 重推
  listSeen(id: number, limit?: number) {
    return client.get<ApiResponse<{ items: Array<{ torrentId: string; title: string; status: string; isFree: boolean }>; total: number }>>(`/rss/subscriptions/${id}/seen`, { params: limit ? { limit } : undefined })
  },
  clearSeen(id: number, data: { torrent_ids?: string[]; all?: boolean }) {
    return client.post<ApiResponse<{ cleared: number; message: string }>>(`/rss/subscriptions/${id}/seen/clear`, data)
  },
}
