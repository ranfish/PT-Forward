import client from './client'

export const reseedApi = {
  listTasks(page = 1, size = 20) {
    return client.get('/reseed/tasks', { params: { page, size } })
  },
  getTask(id: number) {
    return client.get(`/reseed/tasks/${id}`)
  },
  createTask(data: Record<string, unknown>) {
    return client.post('/reseed/tasks', data)
  },
  deleteTask(id: number) {
    return client.delete(`/reseed/tasks/${id}`)
  },
  triggerTask(id: number) {
    return client.post(`/reseed/tasks/${id}/trigger`)
  },
  cancelTask(id: number) {
    return client.post(`/reseed/tasks/${id}/cancel`)
  },
  getMatches(taskId: number) {
    return client.get(`/reseed/tasks/${taskId}/matches`)
  },
  retryMatch(taskId: number, matchId: number) {
    return client.post(`/reseed/tasks/${taskId}/matches/${matchId}/retry`)
  },
  deleteNegativeCache(taskId: number, infoHash: string, site?: string) {
    const params: Record<string, string> = { infoHash }
    if (site) params.site = site
    return client.delete(`/reseed/tasks/${taskId}/negative-cache`, { params })
  },
}
