import client from './client'
import type { SchedulerTask } from './types'
import type { ApiResponse } from './types'

export const schedulerApi = {
  list() {
    return client.get<ApiResponse<{ items: SchedulerTask[]; total: number }>>('/scheduler/tasks')
  },
  get(name: string) {
    return client.get<ApiResponse<SchedulerTask>>(`/scheduler/tasks/${encodeURIComponent(name)}`)
  },
  pause(name: string) {
    return client.post<ApiResponse<void>>(`/scheduler/tasks/${encodeURIComponent(name)}/pause`)
  },
  resume(name: string) {
    return client.post<ApiResponse<void>>(`/scheduler/tasks/${encodeURIComponent(name)}/resume`)
  },
  trigger(name: string) {
    return client.post<ApiResponse<void>>(`/scheduler/tasks/${encodeURIComponent(name)}/trigger`)
  },
  reschedule(name: string, schedule: string) {
    return client.put<ApiResponse<void>>(`/scheduler/tasks/${encodeURIComponent(name)}/schedule`, { schedule })
  },
}
