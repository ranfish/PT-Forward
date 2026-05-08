import client from './client'

export const torrentEventsApi = {
  list(params?: { site?: string }) {
    return client.get('/torrent-events', { params })
  },
  cleanup() {
    return client.post('/torrent-events/cleanup')
  },
}
