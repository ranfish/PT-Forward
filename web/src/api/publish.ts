import client from './client'

export const publishApi = {
  listTasks(page = 1, size = 20) {
    return client.get('/publish/tasks', { params: { page, size } })
  },
  createTask(data: any) {
    return client.post('/publish/tasks', data)
  },
  getTask(id: number) {
    return client.get(`/publish/tasks/${id}`)
  },
  cancelTask(id: number) {
    return client.post(`/publish/tasks/${id}/cancel`)
  },
  listCandidates(params?: any) {
    return client.get('/publish/candidates', { params })
  },
  getCandidate(id: number) {
    return client.get(`/publish/candidates/${id}`)
  },
  deleteCandidate(id: number) {
    return client.delete(`/publish/candidates/${id}`)
  },
  manualPublish(id: number) {
    return client.post(`/publish/candidates/${id}/publish`)
  },
  listGroups() {
    return client.get('/publish/groups')
  },
  getGroup(id: number) {
    return client.get(`/publish/groups/${id}`)
  },
  createGroup(data: any) {
    return client.post('/publish/groups', data)
  },
  updateGroup(id: number, data: any) {
    return client.put(`/publish/groups/${id}`, data)
  },
  deleteGroup(id: number) {
    return client.delete(`/publish/groups/${id}`)
  },
  pauseGroup(id: number) {
    return client.post(`/publish/groups/${id}/lifecycle/pause`)
  },
  resumeGroup(id: number) {
    return client.post(`/publish/groups/${id}/lifecycle/resume`)
  },
  listResults(params?: any) {
    return client.get('/publish/results', { params })
  },
}
