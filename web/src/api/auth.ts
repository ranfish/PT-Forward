import client from './client'

export interface LoginRequest {
  username: string
  password: string
}

export interface TokenPair {
  accessToken: string
  refreshToken: string
  expiresIn: number
}

export const authApi = {
  login(data: LoginRequest) {
    return client.post<{ data: TokenPair }>('/auth/login', data)
  },
  refreshToken(refreshToken: string) {
    return client.post<{ data: TokenPair }>('/auth/refresh', { refreshToken })
  },
  changePassword(oldPassword: string, newPassword: string) {
    return client.put('/auth/password', { oldPassword, newPassword })
  },
  getProfile() {
    return client.get('/auth/profile')
  },
  updateProfile(displayName: string) {
    return client.put('/auth/profile', { displayName })
  },
  isInitialized() {
    return client.get('/auth/status')
  },
}
