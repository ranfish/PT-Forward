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
    return client.get<ApiResponsePaginated<unknown>>('/dashboard/activities', { params: { page, size } })
  },
  getTrends(days = 7) {
    return client.get<ApiResponse<{ trends: TrendPoint[]; days: number }>>('/dashboard/trends', { params: { days } })
  },
}
