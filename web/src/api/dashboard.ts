import client from './client'

export const dashboardApi = {
  getOverview() {
    return client.get('/dashboard/overview')
  },
  getActivities(page = 1, size = 20) {
    return client.get('/dashboard/activities', { params: { page, size } })
  },
  getTrends(days = 7) {
    return client.get('/dashboard/trends', { params: { days } })
  },
}
