import client from './client'

export const settingsApi = {
  get(prefix?: string) {
    const params: Record<string, string> = {}
    if (prefix) params.prefix = prefix
    return client.get('/settings', { params })
  },
  getByKey(key: string) {
    return client.get(`/settings/${key}`)
  },
  update(key: string, data: { value: string }) {
    return client.put(`/settings/${key}`, data)
  },
  remove(key: string) {
    return client.delete(`/settings/${key}`)
  },
  backup() {
    return client.get('/settings/backup')
  },
  restore(data: Record<string, unknown>) {
    return client.post('/settings/restore', data)
  },
}
