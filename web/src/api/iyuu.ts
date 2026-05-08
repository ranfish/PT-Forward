import client from './client'

export const iyuuApi = {
  getConfig() {
    return client.get('/iyuu/config')
  },
  saveConfig(data: any) {
    return client.put('/iyuu/config', data)
  },
  listSites() {
    return client.get('/iyuu/sites')
  },
  syncSites() {
    return client.post('/iyuu/sites')
  },
  query(data: { infoHashes: string[] }) {
    return client.post('/iyuu/query', data)
  },
  test() {
    return client.post('/iyuu/test')
  },
}
