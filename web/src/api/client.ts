import axios from 'axios'
import router from '@/router'

const client = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
})

client.interceptors.request.use((config) => {
  const token = localStorage.getItem('pt-forward-access-token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

client.interceptors.response.use(
  (resp) => {
    const body = resp.data
    if (body && body.code !== undefined && body.code !== 0) {
      return Promise.reject(new Error(body.message || '请求失败'))
    }
    return resp
  },
  async (error) => {
    if (error.response?.status === 401) {
      const refreshToken = localStorage.getItem('pt-forward-refresh-token')
      if (refreshToken && !error.config._retry) {
        error.config._retry = true
        try {
          const resp = await axios.post('/api/v1/auth/refresh', { refreshToken })
          const { accessToken, refreshToken: newRefresh } = resp.data.data
          localStorage.setItem('pt-forward-access-token', accessToken)
          localStorage.setItem('pt-forward-refresh-token', newRefresh)
          error.config.headers.Authorization = `Bearer ${accessToken}`
          return client(error.config)
        } catch {
          localStorage.removeItem('pt-forward-access-token')
          localStorage.removeItem('pt-forward-refresh-token')
          router.push({ name: 'Login' })
        }
      } else {
        localStorage.removeItem('pt-forward-access-token')
        localStorage.removeItem('pt-forward-refresh-token')
        router.push({ name: 'Login' })
      }
    }
    const msg = error.response?.data?.message || error.message || '网络错误'
    return Promise.reject(new Error(msg))
  },
)

export default client
