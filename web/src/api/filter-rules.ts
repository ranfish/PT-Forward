import client from './client'

export const filterRulesApi = {
  list(page = 1, size = 20) {
    return client.get('/filters/rules', { params: { page, size } })
  },
  create(data: any) {
    return client.post('/filters/rules', data)
  },
  update(id: number, data: any) {
    return client.put(`/filters/rules/${id}`, data)
  },
  delete(id: number) {
    return client.delete(`/filters/rules/${id}`)
  },
  test(id: number, data?: any) {
    return client.post(`/filters/rules/${id}/test`, data || {})
  },
}
