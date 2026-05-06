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
  pauseTorrent(id: number, infoHash: string) {
    return client.post(`/downloaders/${id}/torrents/${infoHash}/pause`)
  },
  resumeTorrent(id: number, infoHash: string) {
    return client.post(`/downloaders/${id}/torrents/${infoHash}/resume`)
  },
  deleteTorrent(id: number, infoHash: string) {
    return client.delete(`/downloaders/${id}/torrents/${infoHash}`)
  },
  listPublishTargets() {
    return client.get('/downloaders/publish-targets')
  },
  createPublishTarget(data: any) {
    return client.post('/downloaders/publish-targets', data)
  },
  updatePublishTarget(data: any) {
    return client.put('/downloaders/publish-targets', data)
  },
  deletePublishTarget(id: number) {
    return client.delete('/downloaders/publish-targets', { data: { id } })
  },
}
