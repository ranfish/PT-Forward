import client from './client'

export const authApi = {
  status() {
    return client.get('/auth/status')
  },
  setup(data: { username: string; password: string }) {
    return client.post('/auth/setup', data)
  },
  login(data: { username: string; password: string }) {
    return client.post('/auth/login', data)
  },
  changePassword(oldPassword: string, newPassword: string) {
    return client.put('/auth/password', { oldPassword, newPassword })
  },
  getProfile() {
    return client.get('/auth/profile')
  },
  updateProfile(data: { username?: string; password?: string }) {
    return client.put('/auth/profile', data)
  },
}
