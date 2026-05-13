import client from './client'

export const seedingApi = {
  getStatus() {
    return client.get('/seeding/status')
  },
  getConfig() {
    return client.get('/seeding/configs')
  },
  updateConfig(id: number, data: Record<string, unknown>) {
    return client.put(`/seeding/configs/${id}`, data)
  },
  createConfig(data: Record<string, unknown>) {
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
  scoringDryrun(data: Record<string, unknown>) {
    return client.post('/seeding/scoring-dryrun', data)
  },
  getScoringConfig(subscriptionId?: number) {
    const params: Record<string, number> = {}
    if (subscriptionId) params.subscriptionId = subscriptionId
    return client.get('/seeding/scoring-config', { params })
  },
  updateScoringConfig(data: Record<string, unknown>, subscriptionId?: number) {
    const params: Record<string, number> = {}
    if (subscriptionId) params.subscriptionId = subscriptionId
    return client.put('/seeding/scoring-config', data, { params })
  },
  listScoringLogs(params?: Record<string, unknown>) {
    return client.get('/seeding/scoring-logs', { params })
  },
}

export const seedingRulesApi = {
  list() {
    return client.get('/seeding/rules')
  },
  create(data: Record<string, unknown>) {
    return client.post('/seeding/rules', data)
  },
  get(id: number) {
    return client.get(`/seeding/rules/${id}`)
  },
  update(id: number, data: Record<string, unknown>) {
    return client.put(`/seeding/rules/${id}`, data)
  },
  delete(id: number) {
    return client.delete(`/seeding/rules/${id}`)
  },
  test(id: number, data?: Record<string, unknown>) {
    return client.post(`/seeding/rules/${id}/test`, data || {})
  },
}

export const seedingClientsApi = {
  list() {
    return client.get('/seeding/clients')
  },
  trigger(clientId: string) {
    return client.post(`/seeding/clients/${clientId}/trigger`)
  },
}

export const seedingDryrunApi = {
  runAll() {
    return client.post('/seeding/dryrun')
  },
  runBySubscription(subId: string) {
    return client.post(`/seeding/dryrun/${subId}`)
  },
}

export const seedingStatsApi = {
  overview() {
    return client.get('/seeding/stats/overview')
  },
  bySite() {
    return client.get('/seeding/stats/by-site')
  },
  torrents(page = 1, size = 20) {
    return client.get('/seeding/stats/torrents', { params: { page, size } })
  },
  siteTrend(site: string) {
    return client.get(`/seeding/stats/by-site/${encodeURIComponent(site)}/trend`)
  },
  downloaderSpeedTrend(id: number) {
    return client.get(`/seeding/stats/downloader/${id}/speed-trend`)
  },
}

export const deleteRulesApi = {
  list() {
    return client.get('/seeding/delete-rules')
  },
  create(data: Record<string, unknown>) {
    return client.post('/seeding/delete-rules', data)
  },
  update(id: number, data: Record<string, unknown>) {
    return client.put(`/seeding/delete-rules/${id}`, data)
  },
  delete(id: number) {
    return client.delete(`/seeding/delete-rules/${id}`)
  },
  test(id: number) {
    return client.post(`/seeding/delete-rules/${id}/test`)
  },
}
