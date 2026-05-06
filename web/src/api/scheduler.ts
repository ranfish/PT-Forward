import client from './client'

export interface SchedulerTask {
  name: string
  type: string
  schedule: string
  last_run_at: string | null
  last_error: string
  success_count: number
  error_count: number
  paused: boolean
}

export const schedulerApi = {
  list() {
    return client.get<{ data: { items: SchedulerTask[]; total: number } }>('/scheduler/tasks')
  },
  get(name: string) {
    return client.get<{ data: SchedulerTask }>(`/scheduler/tasks/${encodeURIComponent(name)}`)
  },
  pause(name: string) {
    return client.post(`/scheduler/tasks/${encodeURIComponent(name)}/pause`)
  },
  resume(name: string) {
    return client.post(`/scheduler/tasks/${encodeURIComponent(name)}/resume`)
  },
  trigger(name: string) {
    return client.post(`/scheduler/tasks/${encodeURIComponent(name)}/trigger`)
  },
  reschedule(name: string, schedule: string) {
    return client.put(`/scheduler/tasks/${encodeURIComponent(name)}/schedule`, { schedule })
  },
}
