import client from './client'
import type { ApiResponse } from './types'

export interface DownloadTask {
  id: number
  created_at: string
  updated_at: string
  source: string
  subscription_id?: number
  client_id: string
  info_hash: string
  torrent_name: string
  save_path: string
  total_size: number
  site_name: string
  status: string
  progress: number
  upload_speed: number
  download_speed: number
  ratio: number
  uploaded: number
  num_seeds: number
  num_peers: number
  error_message: string
  transfer_status: string
  transfer_client_id: string
  transfer_hash: string
  transferred_at?: string
  deleted_at?: string
  delete_action: string
  category: string
}

export interface DownloadTaskListResponse {
  items: DownloadTask[]
  total: number
  page: number
  size: number
}

export const downloadsApi = {
  list(params?: { page?: number; size?: number; client_id?: string; status?: string }) {
    const query = new URLSearchParams()
    if (params?.page) query.set('page', String(params.page))
    if (params?.size) query.set('size', String(params.size))
    if (params?.client_id) query.set('client_id', params.client_id)
    if (params?.status) query.set('status', params.status)
    const qs = query.toString()
    return client.get<ApiResponse<DownloadTaskListResponse>>(`/downloads${qs ? '?' + qs : ''}`)
  },

  get(id: number) {
    return client.get<ApiResponse<DownloadTask>>(`/downloads/${id}`)
  },

  delete(id: number, deleteCompanions: boolean) {
    return client.delete<ApiResponse<unknown>>(`/downloads/${id}`, { data: { delete_companions: deleteCompanions } })
  },

  bulkAction(ids: number[], action: string, deleteCompanions?: boolean) {
    return client.post<ApiResponse<{ succeeded: number; failed: number }>>(`/downloads/bulk-action`, {
      ids,
      action,
      delete_companions: deleteCompanions,
    })
  },
}
