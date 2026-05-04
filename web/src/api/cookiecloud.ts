import client from './client'

export const cookiecloudApi = {
  getConfig() {
    return client.get('/cookiecloud/config')
  },
  saveConfig(data: any) {
    return client.put('/cookiecloud/config', data)
  },
  sync() {
    return client.post('/cookiecloud/sync')
  },
  listHistory(params?: { page?: number; size?: number }) {
    return client.get('/cookiecloud/history', { params })
  },
  test() {
    return client.post('/cookiecloud/test')
  },
}
