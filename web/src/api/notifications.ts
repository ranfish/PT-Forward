import client from './client'
import type { ApiResponse, NotificationChannel, NotificationHistory, UpdatePartial } from './types'

export const notificationsApi = {
  list() {
    return client.get<ApiResponse<{ items: NotificationChannel[]; total: number }>>('/notifications/channels')
  },
  create(data: UpdatePartial<NotificationChannel>) {
    return client.post<ApiResponse<NotificationChannel>>('/notifications/channels', data)
  },
  update(id: number, data: UpdatePartial<NotificationChannel>) {
    return client.put<ApiResponse<NotificationChannel>>(`/notifications/channels/${id}`, data)
  },
  delete(id: number) {
    return client.delete<ApiResponse<void>>(`/notifications/channels/${id}`)
  },
  test(id: number) {
    return client.post<ApiResponse<{ success: boolean }>>(`/notifications/channels/${id}/test`)
  },
  listHistory(channelId: number, limit = 50) {
    return client.get<ApiResponse<{ items: NotificationHistory[]; total: number }>>(`/notifications/channels/${channelId}/history`, { params: { limit } })
  },
}
