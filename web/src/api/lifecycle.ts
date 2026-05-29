import client from './client'
import type { ApiResponse, LifecycleConfig, BackpressureResponse, BackpressureUpdateRequest } from './types'

export const lifecycleApi = {
  getConfig() {
    return client.get<ApiResponse<LifecycleConfig>>('/lifecycle/config')
  },
  updateConfig(data: Partial<LifecycleConfig>) {
    return client.put<ApiResponse<LifecycleConfig>>('/lifecycle/config', data)
  },
  getBackpressure() {
    return client.get<ApiResponse<BackpressureResponse>>('/lifecycle/backpressure')
  },
  updateBackpressure(data: BackpressureUpdateRequest) {
    return client.put<ApiResponse<BackpressureResponse>>('/lifecycle/backpressure', data)
  },
}
