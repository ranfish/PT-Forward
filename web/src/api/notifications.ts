import client from './client'

export const notificationsApi = {
  list() {
    return client.get('/notifications/channels')
  },
  get(id: number) {
    return client.get(`/notifications/channels/${id}`)
  },
  create(data: any) {
    return client.post('/notifications/channels', data)
  },
  update(id: number, data: any) {
    return client.put(`/notifications/channels/${id}`, data)
  },
  delete(id: number) {
    return client.delete(`/notifications/channels/${id}`)
  },
  test(id: number) {
    return client.post(`/notifications/channels/${id}/test`)
  },
  listHistory(channelId?: number, limit = 50) {
    return client.get('/notifications/history', { params: { channelId, limit } })
  },
}
