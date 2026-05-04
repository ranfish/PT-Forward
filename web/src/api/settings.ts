import client from './client'

export const settingsApi = {
  get() {
    return client.get('/settings')
  },
  update(key: string, data: { value: string }) {
    return client.put(`/settings/${key}`, data)
  },
  backup() {
    return client.get('/settings/backup')
  },
  restore(data: any) {
    return client.post('/settings/restore', data)
  },
}
