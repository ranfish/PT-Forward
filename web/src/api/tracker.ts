import client from './client'

export const trackerApi = {
  listMembers() {
    return client.get('/tracker/members')
  },
  getHistory(params?: { site?: string; limit?: number }) {
    return client.get('/tracker/history', { params })
  },
}
