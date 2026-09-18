import client from './client'
import type { ApiResponse } from './types'

export interface TrafficHourlyPoint {
  client_uid: number
  hour: string
  uploaded_delta: number
  downloaded_delta: number
  avg_upload_speed: number
  avg_download_speed: number
  peak_upload_speed: number
  peak_download_speed: number
  active_torrents: number
  sample_count: number
}

export const statsApi = {
  getTrafficHourly(clientId?: number, days = 7) {
    const params: Record<string, string | number> = { days }
    if (clientId) params.client_id = clientId
    return client.get<ApiResponse<TrafficHourlyPoint[]>>('/stats/traffic/hourly', { params })
  },
}
