import client from './client'

export const torrentEventsApi = {
  list(params?: { site?: string }) {
    return client.get('/torrent-events', { params })
  },
  get(id: number) {
    return client.get(`/torrent-events/${id}`)
  },
  cleanup() {
    return client.post('/torrent-events/cleanup')
  },
}
