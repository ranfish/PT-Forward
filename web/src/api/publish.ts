import client from './client'

export const publishApi = {
  listCandidates(params?: any) {
    return client.get('/publish/candidates', { params })
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
  deleteGroup(id: number) {
    return client.delete(`/publish/groups/${id}`)
  },
  pauseGroup(id: number) {
    return client.post(`/publish/groups/${id}/lifecycle/pause`)
  },
  resumeGroup(id: number) {
    return client.post(`/publish/groups/${id}/lifecycle/resume`)
  },
  lifecycleDeleteGroup(id: number) {
    return client.post(`/publish/groups/${id}/lifecycle/delete`)
  },
  createTask(data: any) {
    return client.post('/publish/tasks', data)
  },
  listTasks(params?: any) {
    return client.get('/publish/tasks', { params })
  },
  getTask(id: number) {
    return client.get(`/publish/tasks/${id}`)
  },
  deleteTask(id: number) {
    return client.delete(`/publish/tasks/${id}`)
  },
  listResults(params?: any) {
    return client.get('/publish/results', { params })
  },
}
