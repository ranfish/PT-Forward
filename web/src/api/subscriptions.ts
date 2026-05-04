import client from './client'

export const subscriptionsApi = {
  list(page = 1, size = 20) {
    return client.get('/rss/subscriptions', { params: { page, size } })
  },
  get(id: number) {
    return client.get(`/rss/subscriptions/${id}`)
  },
  create(data: any) {
    return client.post('/rss/subscriptions', data)
  },
  update(id: number, data: any) {
    return client.put(`/rss/subscriptions/${id}`, data)
  },
  delete(id: number) {
    return client.delete(`/rss/subscriptions/${id}`)
  },
  pause(id: number) {
    return client.post(`/rss/subscriptions/${id}/pause`)
  },
  resume(id: number) {
    return client.post(`/rss/subscriptions/${id}/resume`)
  },
  trigger(id: number) {
    return client.post(`/rss/subscriptions/${id}/trigger`)
  },
}
