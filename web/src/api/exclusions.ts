import client from './client'

export const exclusionsApi = {
  list() {
    return client.get('/publish/exclusions')
  },
  create(data: { target_site: string; source_site: string }) {
    return client.post('/publish/exclusions', data)
  },
  remove(data: { target_site: string; source_site: string }) {
    return client.delete('/publish/exclusions', { data })
  },
}
