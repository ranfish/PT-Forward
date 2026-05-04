import client from './client'

export const downloadersApi = {
  list(page = 1, size = 20) {
    return client.get('/downloaders', { params: { page, size } })
  },
  get(id: number) {
    return client.get(`/downloaders/${id}`)
  },
  create(data: any) {
    return client.post('/downloaders', data)
  },
  update(id: number, data: any) {
    return client.put(`/downloaders/${id}`, data)
  },
  delete(id: number) {
    return client.delete(`/downloaders/${id}`)
  },
  testConnection(id: number) {
    return client.post(`/downloaders/${id}/test`)
  },
  getTorrents(id: number, params?: any) {
    return client.get(`/downloaders/${id}/torrents`, { params })
  },
  getMaindata(id: number) {
    return client.get(`/downloaders/${id}/maindata`)
  },
}
