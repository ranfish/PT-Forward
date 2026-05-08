import client from './client'

export const fingerprintsApi = {
  list(page = 1, size = 20) {
    return client.get('/fingerprints', { params: { page, size } })
  },
  search(params: { infoHash?: string; piecesHash?: string }) {
    return client.get('/fingerprints/search', { params })
  },
  delete(id: number) {
    return client.delete(`/fingerprints/${id}`)
  },
  deleteCache() {
    return client.delete('/fingerprints/cache')
  },
}
