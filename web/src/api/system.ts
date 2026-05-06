import client from './client'

export const systemApi = {
  ping() {
    return client.get('/system/ping')
  },
  health() {
    return client.get('/system/health')
  },
  info() {
    return client.get('/system/info')
  },
  listLogs(params?: { level?: string; limit?: number }) {
    return client.get('/system/logs', { params })
  },
  clearLogs() {
    return client.delete('/system/logs')
  },
}
