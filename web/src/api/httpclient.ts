import client from './client'

export const httpclientApi = {
  getFreezeStatuses() {
    return client.get('/httpclient/freeze-status')
  },
  unfreezeDomain(domain: string) {
    return client.delete('/httpclient/freeze-status', { data: { domain } })
  },
  getCircuitStatuses() {
    return client.get('/httpclient/circuit-status')
  },
  resetCircuit(domain: string) {
    return client.post('/httpclient/circuit-status', { domain })
  },
}
