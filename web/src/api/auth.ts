import client from './client'
import type { ApiResponse } from './types'

export const authApi = {
  status() {
    return client.get<ApiResponse<{ initialized: boolean }>>('/auth/status')
  },
  setup(data: { username: string; password: string }) {
    return client.post<ApiResponse<{ accessToken: string; refreshToken: string }>>('/auth/setup', data)
  },
  login(data: { username: string; password: string }) {
    return client.post<ApiResponse<{ accessToken: string; refreshToken: string }>>('/auth/login', data)
  },
  changePassword(oldPassword: string, newPassword: string) {
    return client.put<ApiResponse<void>>('/auth/password', { oldPassword, newPassword })
  },
  getProfile() {
    return client.get<ApiResponse<{ username: string; displayName: string }>>('/auth/profile')
  },
  updateProfile(data: { displayName: string }) {
    return client.put<ApiResponse<void>>('/auth/profile', data)
  },
}
