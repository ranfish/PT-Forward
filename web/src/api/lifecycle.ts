import client from './client'

export const lifecycleApi = {
  getConfig() {
    return client.get('/lifecycle/config')
  },
  updateConfig(data: any) {
    return client.put('/lifecycle/config', data)
  },
  getBackpressure() {
    return client.get('/lifecycle/backpressure')
  },
}
