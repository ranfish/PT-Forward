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
}
