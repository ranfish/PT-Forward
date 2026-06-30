import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/auth/Login.vue'),
    meta: { requiresAuth: false, layout: 'blank' },
  },
  {
    path: '/',
    component: () => import('@/layouts/MainLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      { path: '', name: 'Dashboard', component: () => import('@/views/dashboard/Index.vue') },
      { path: 'sites', name: 'Sites', component: () => import('@/views/sites/SiteList.vue') },
      { path: 'sites/:id', name: 'SiteDetail', component: () => import('@/views/sites/SiteDetail.vue') },
      { path: 'downloaders', name: 'Downloaders', component: () => import('@/views/downloaders/DownloaderList.vue') },
      { path: 'downloaders/:id', name: 'DownloaderDetail', component: () => import('@/views/downloaders/DownloaderDetail.vue') },
      { path: 'subscriptions', name: 'Subscriptions', component: () => import('@/views/subscriptions/SubscriptionList.vue') },
      { path: 'subscriptions/:id', name: 'SubscriptionDetail', component: () => import('@/views/subscriptions/SubscriptionDetail.vue') },
      { path: 'seeding', name: 'SeedingDashboard', component: () => import('@/views/seeding/SeedingDashboard.vue') },
      { path: 'seeding/rules', name: 'SeedingRules', component: () => import('@/views/seeding/SeedingRuleList.vue') },
      { path: 'seeding/torrents', name: 'SeedingTorrents', component: () => import('@/views/seeding/SeedingTorrentList.vue') },
      { path: 'seeding/stats', name: 'SeedingStats', component: () => import('@/views/seeding/SeedingStats.vue') },
      { path: 'seeding/scoring', name: 'SeedingScoring', component: () => import('@/views/seeding/SeedingScoring.vue') },
      { path: 'downloads', name: 'Downloads', component: () => import('@/views/downloads/DownloadTaskList.vue') },
      { path: 'reseed', name: 'ReseedTasks', component: () => import('@/views/reseed/ReseedTaskList.vue') },
      { path: 'reseed/tasks/:id', name: 'ReseedTaskDetail', component: () => import('@/views/reseed/ReseedTaskDetail.vue') },
      { path: 'publish', name: 'PublishList', component: () => import('@/views/publish/PublishList.vue') },
      { path: 'publish/exclusions', name: 'PublishExclusions', component: () => import('@/views/publish/ExclusionList.vue') },
      { path: 'publish/groups/:id', name: 'PublishGroupDetail', component: () => import('@/views/publish/PublishGroupDetail.vue') },
      { path: 'publish/manual', name: 'PublishManual', component: () => import('@/views/publish/PublishWizard.vue') },
      { path: 'fingerprints', name: 'Fingerprints', component: () => import('@/views/fingerprints/FingerprintList.vue') },
      { path: 'events', name: 'TorrentEvents', component: () => import('@/views/events/TorrentEventList.vue') },
      { path: 'iyuu', name: 'IYUU', component: () => import('@/views/iyuu/IYUUConfig.vue') },
      { path: 'cloud-fp', name: 'CloudFP', component: () => import('@/views/cloudfp/CloudFPConfig.vue') },
      { path: 'cookiecloud', name: 'CookieCloud', component: () => import('@/views/cookiecloud/CookieCloudConfig.vue') },
      { path: 'ptgen', name: 'PTGen', component: () => import('@/views/ptgen/PTGenPage.vue') },
      { path: 'tracker', name: 'Tracker', component: () => import('@/views/tracker/TrackerList.vue') },
      { path: 'lifecycle', name: 'Lifecycle', component: () => import('@/views/lifecycle/LifecycleConfig.vue') },
      { path: 'scheduler', name: 'Scheduler', component: () => import('@/views/scheduler/SchedulerTaskList.vue') },
      { path: 'system', name: 'SystemHealth', component: () => import('@/views/system/SystemHealth.vue') },
      { path: 'httpclient', name: 'FreezeStatus', component: () => import('@/views/httpclient/FreezeStatus.vue') },
      { path: 'logs', name: 'Logs', component: () => import('@/views/system/SystemHealth.vue') },
      { path: 'audit', name: 'AuditLogs', component: () => import('@/views/system/AuditLogList.vue') },
      { path: 'settings', name: 'Settings', component: () => import('@/views/settings/Index.vue') },
      { path: 'settings/notifications', name: 'NotificationSettings', component: () => import('@/views/settings/NotificationSettings.vue') },
      { path: 'settings/auth', name: 'AuthSettings', component: () => import('@/views/settings/AuthSettings.vue') },
      { path: 'settings/filter-rules', name: 'FilterRules', component: () => import('@/views/settings/FilterRuleList.vue') },
      { path: '/:pathMatch(.*)*', name: 'NotFound', redirect: '/' },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
  const token = localStorage.getItem('pt-forward-access-token')
  if (to.meta.requiresAuth !== false && !isTokenValid(token)) {
    return { name: 'Login', query: { redirect: to.fullPath } }
  }
  if (to.name === 'Login' && isTokenValid(token)) {
    return { name: 'Dashboard' }
  }
})

export function isTokenValid(token: string | null): boolean {
  if (!token) return false
  try {
    const parts = token.split('.')
    if (parts.length !== 3) return false
    const payload = JSON.parse(atob(parts[1]))
    if (payload.exp && payload.exp * 1000 < Date.now()) return false
    return true
  } catch {
    return false
  }
}

export default router
