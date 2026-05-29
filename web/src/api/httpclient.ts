import client from './client'
import type { ApiResponse } from './types'

export const httpclientApi = {
  getFreezeStatuses() {
    return client.get<ApiResponse<Array<{ domain: string; frozenAt: string; reason: string }>>>('/httpclient/freeze-status')
  },
  unfreezeDomain(domain: string) {
    return client.delete<ApiResponse<void>>('/httpclient/freeze-status', { data: { domain } })
  },
  getCircuitStatuses() {
    return client.get<ApiResponse<Array<{ domain: string; state: string; failures: number }>>>('/httpclient/circuit-status')
  },
  resetCircuit(domain: string) {
    return client.post<ApiResponse<void>>('/httpclient/circuit-status', { domain })
  },
}
