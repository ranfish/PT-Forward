import client from './client'

export const lifecycleApi = {
  getConfig() {
    return client.get('/lifecycle/config')
  },
  updateConfig(data: Record<string, unknown>) {
    return client.put('/lifecycle/config', data)
  },
  getBackpressure() {
    return client.get('/lifecycle/backpressure')
  },
  updateBackpressure(data: Record<string, unknown>) {
    return client.put('/lifecycle/backpressure', data)
  },
}
