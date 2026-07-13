import client from './client'
import type { ApiResponse, ApiResponsePaginated } from './types'

export interface TrendPoint {
  date: string
  events: number
  rss: number
  publish: number
  reseed: number
}

export const dashboardApi = {
  getOverview() {
    return client.get<ApiResponse<Record<string, unknown>>>('/dashboard/overview')
  },
  getActivities(page = 1, size = 20) {
    return client.get<ApiResponse<{ items: unknown[]; total: number }>>('/dashboard/activities', { params: { page, size } })
  },
  getTrends(days = 7) {
    return client.get<ApiResponse<{ trends: TrendPoint[] }>>('/dashboard/trends', { params: { days } })
  },
  getSystemDashboard() {
    return client.get<ApiResponse<SystemDashboard>>('/system/dashboard')
  },
  getSeedingMonitor() {
    return client.get<ApiResponse<Record<string, unknown>>>('/seeding/monitor')
  },
  getReseedMonitor() {
    return client.get<ApiResponse<Record<string, unknown>>>('/reseed/monitor')
  },
  getPublishMonitor() {
    return client.get<ApiResponse<Record<string, unknown>>>('/publish/monitor')
  },
}

export interface SystemDashboard {
  seeding: {
    active: number
    deleted_today: number
    rss_enabled: number
    last_rss_fetch: string
  }
  download: {
    downloading: number
    completed: number
    completed_today: number
    error: number
    transfer_pending: number
  }
  reseed: {
    active_tasks: number
    pending_injection: number
    injected_today: number
  }
  publish: {
    publishing: number
    pending: number
    done_total: number
    done_today: number
  }
  system: {
    uptime: number
    version: string
    goroutines: number
    memory_mb: number
    clients: number
  }
}
