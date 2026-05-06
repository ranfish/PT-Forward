import client from './client'

export const seedingApi = {
  getStatus() {
    return client.get('/seeding/status')
  },
  getConfig() {
    return client.get('/seeding/configs')
  },
  updateConfig(id: number, data: any) {
    return client.put(`/seeding/configs/${id}`, data)
  },
  createConfig(data: any) {
    return client.post('/seeding/configs', data)
  },
  deleteConfig(id: number) {
    return client.delete(`/seeding/configs/${id}`)
  },
  listRecords(page = 1, size = 20) {
    return client.get('/seeding/records', { params: { page, size } })
  },
  getStatsOverview() {
    return client.get('/seeding/stats/overview')
  },
  getStatsBySite() {
    return client.get('/seeding/stats/by-site')
  },
  getStatsTorrents(page = 1, size = 20) {
    return client.get('/seeding/stats/torrents', { params: { page, size } })
  },
  getTorrents(page = 1, size = 20) {
    return client.get('/seeding/torrents', { params: { page, size } })
  },
  resumeRecord(id: number) {
    return client.post(`/seeding/records/${id}/resume`)
  },
  pauseRecord(id: number) {
    return client.post(`/seeding/records/${id}/pause`)
  },
}

export const deleteRulesApi = {
  list() {
    return client.get('/seeding/delete-rules')
  },
  get(id: number) {
    return client.get(`/seeding/delete-rules/${id}`)
  },
  create(data: any) {
    return client.post('/seeding/delete-rules', data)
  },
  update(id: number, data: any) {
    return client.put(`/seeding/delete-rules/${id}`, data)
  },
  delete(id: number) {
    return client.delete(`/seeding/delete-rules/${id}`)
  },
  test(id: number) {
    return client.post(`/seeding/delete-rules/${id}/test`)
  },
}
