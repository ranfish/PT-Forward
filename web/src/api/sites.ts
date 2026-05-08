import client from './client'

export const sitesApi = {
  list(page = 1, size = 20, search = '') {
    return client.get('/sites', { params: { page, size, search } })
  },
  get(id: number) {
    return client.get(`/sites/${id}`)
  },
  create(data: any) {
    return client.post('/sites', data)
  },
  update(id: number, data: any) {
    return client.put(`/sites/${id}`, data)
  },
  delete(id: number) {
    return client.delete(`/sites/${id}`)
  },
  testConnection(id: number) {
    return client.post(`/sites/${id}/test`)
  },
  detect(id: number) {
    return client.post(`/sites/${id}/detect`)
  },
  getStats(id: number) {
    return client.get(`/sites/${id}/stats`)
  },
  updateCredentials(id: number, data: any) {
    return client.put(`/sites/${id}/credentials`, data)
  },
  getOverrides(id: number) {
    return client.get(`/sites/${id}/overrides`)
  },
  updateOverrides(id: number, data: any) {
    return client.put(`/sites/${id}/overrides`, data)
  },
  deleteOverrides(id: number) {
    return client.delete(`/sites/${id}/overrides`)
  },
}
