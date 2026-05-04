import client from './client'

export const ptgenApi = {
  query(data: { query: string }) {
    return client.post('/ptgen/query', data)
  },
  listCache(params?: { page?: number; size?: number; keyword?: string }) {
    return client.get('/ptgen/cache', { params })
  },
  cleanCache(retainDays?: number) {
    return client.delete('/ptgen/cache', { params: { retainDays } })
  },
}
