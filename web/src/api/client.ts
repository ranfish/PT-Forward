import axios from 'axios'
import router from '@/router'

const client = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' },
})

let isRefreshing = false
let refreshSubscribers: Array<(token: string) => void> = []

function onTokenRefreshed(token: string) {
  refreshSubscribers.forEach((cb) => cb(token))
  refreshSubscribers = []
}

function addRefreshSubscriber(cb: (token: string) => void) {
  refreshSubscribers.push(cb)
}

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
    const originalRequest = error.config

    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true

      if (isRefreshing) {
        return new Promise((resolve) => {
          addRefreshSubscriber((token: string) => {
            originalRequest.headers.Authorization = `Bearer ${token}`
            resolve(client(originalRequest))
          })
        })
      }

      isRefreshing = true
      const refreshToken = localStorage.getItem('pt-forward-refresh-token')

      if (refreshToken) {
        try {
          const resp = await axios.post('/api/v1/auth/refresh', { refreshToken })
          const { accessToken, refreshToken: newRefresh } = resp.data.data
          localStorage.setItem('pt-forward-access-token', accessToken)
          localStorage.setItem('pt-forward-refresh-token', newRefresh)

          onTokenRefreshed(accessToken)
          isRefreshing = false

          originalRequest.headers.Authorization = `Bearer ${accessToken}`
          return client(originalRequest)
        } catch {
          isRefreshing = false
          refreshSubscribers = []
          localStorage.removeItem('pt-forward-access-token')
          localStorage.removeItem('pt-forward-refresh-token')
          router.push({ name: 'Login' })
        }
      } else {
        isRefreshing = false
        refreshSubscribers = []
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
