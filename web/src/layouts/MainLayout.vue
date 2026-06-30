<template>
  <a-layout style="min-height: 100vh">
    <a-layout-sider
      v-model:collapsed="collapsed"
      collapsible
      :width="220"
      theme="dark"
    >
      <div class="logo">PT-Forward</div>
      <a-menu
        v-model:selected-keys="selectedKeys"
        v-model:open-keys="openKeys"
        theme="dark"
        mode="inline"
      >
        <a-menu-item-group :title="t('nav.resourceManagement')">
          <a-menu-item key="/" @click="$router.push('/')">
            <template #icon><DashboardOutlined /></template>
            <span>{{ t('nav.dashboard') }}</span>
          </a-menu-item>
          <a-menu-item key="/sites" @click="$router.push('/sites')">
            <template #icon><GlobalOutlined /></template>
            <span>{{ t('nav.sites') }}</span>
          </a-menu-item>
          <a-menu-item key="/downloaders" @click="$router.push('/downloaders')">
            <template #icon><CloudDownloadOutlined /></template>
            <span>{{ t('nav.downloaders') }}</span>
          </a-menu-item>
          <a-menu-item key="/subscriptions" @click="$router.push('/subscriptions')">
            <template #icon><NotificationOutlined /></template>
            <span>{{ t('nav.subscriptions') }}</span>
          </a-menu-item>
        </a-menu-item-group>

        <a-menu-item-group :title="t('nav.coreFeatures')">
          <a-sub-menu key="seeding-sub">
            <template #icon><ThunderboltOutlined /></template>
            <template #title>{{ t('nav.seeding') }}</template>
            <a-menu-item key="/seeding" @click="$router.push('/seeding')">{{ t('nav.seedingDashboard') }}</a-menu-item>
            <a-menu-item key="/seeding/rules" @click="$router.push('/seeding/rules')">{{ t('nav.seedingRules') }}</a-menu-item>
            <a-menu-item key="/seeding/scoring" @click="$router.push('/seeding/scoring')">{{ t('nav.seedingScoring') }}</a-menu-item>
            <a-menu-item key="/seeding/torrents" @click="$router.push('/seeding/torrents')">{{ t('nav.seedingTorrents') }}</a-menu-item>
            <a-menu-item key="/seeding/stats" @click="$router.push('/seeding/stats')">{{ t('nav.seedingStats') }}</a-menu-item>
          </a-sub-menu>
          <a-menu-item key="/downloads" @click="$router.push('/downloads')">
            <template #icon><DownloadOutlined /></template>
            <span>{{ t('nav.downloads') }}</span>
          </a-menu-item>
          <a-menu-item key="/reseed" @click="$router.push('/reseed')">
            <template #icon><CopyOutlined /></template>
            <span>{{ t('nav.reseed') }}</span>
          </a-menu-item>
          <a-sub-menu key="publish-sub">
            <template #icon><SendOutlined /></template>
            <template #title>{{ t('nav.publish') }}</template>
            <a-menu-item key="/publish" @click="$router.push('/publish')">{{ t('publish.candidates') }}</a-menu-item>
            <a-menu-item key="/publish/exclusions" @click="$router.push('/publish/exclusions')">{{ t('nav.publishExclusions') }}</a-menu-item>
          </a-sub-menu>
          <a-menu-item key="/iyuu" @click="$router.push('/iyuu')">
            <template #icon><ApiOutlined /></template>
            <span>{{ t('nav.iyuu') }}</span>
          </a-menu-item>
          <a-menu-item key="/cloud-fp" @click="$router.push('/cloud-fp')">
            <template #icon><CloudOutlined /></template>
            <span>{{ t('nav.cloudFp') }}</span>
          </a-menu-item>
          <a-menu-item key="/cookiecloud" @click="$router.push('/cookiecloud')">
            <template #icon><CloudSyncOutlined /></template>
            <span>{{ t('nav.cookiecloud') }}</span>
          </a-menu-item>
          <a-menu-item key="/ptgen" @click="$router.push('/ptgen')">
            <template #icon><VideoCameraOutlined /></template>
            <span>{{ t('nav.ptgen') }}</span>
          </a-menu-item>
          <a-menu-item key="/tracker" @click="$router.push('/tracker')">
            <template #icon><ClusterOutlined /></template>
            <span>{{ t('nav.tracker') }}</span>
          </a-menu-item>
          <a-menu-item key="/lifecycle" @click="$router.push('/lifecycle')">
            <template #icon><HeartOutlined /></template>
            <span>{{ t('nav.lifecycle') }}</span>
          </a-menu-item>
          <a-menu-item key="/scheduler" @click="$router.push('/scheduler')">
            <template #icon><FieldTimeOutlined /></template>
            <span>{{ t('nav.scheduler') }}</span>
          </a-menu-item>
        </a-menu-item-group>

        <a-menu-item-group :title="t('nav.system')">
          <a-menu-item key="/system" @click="$router.push('/system')">
            <template #icon><DashboardOutlined /></template>
            <span>{{ t('nav.systemHealth') }}</span>
          </a-menu-item>
          <a-menu-item key="/events" @click="$router.push('/events')">
            <template #icon><UnorderedListOutlined /></template>
            <span>{{ t('nav.torrentEvents') }}</span>
          </a-menu-item>
          <a-menu-item key="/httpclient" @click="$router.push('/httpclient')">
            <template #icon><StopOutlined /></template>
            <span>{{ t('nav.freezeStatus') }}</span>
          </a-menu-item>
          <a-menu-item key="/fingerprints" @click="$router.push('/fingerprints')">
            <template #icon><SafetyOutlined /></template>
            <span>{{ t('nav.fingerprints') }}</span>
          </a-menu-item>
          <a-menu-item key="/logs" @click="$router.push('/logs')">
            <template #icon><FileTextOutlined /></template>
            <span>{{ t('nav.logs') }}</span>
          </a-menu-item>
          <a-menu-item key="/audit" @click="$router.push('/audit')">
            <template #icon><AuditOutlined /></template>
            <span>{{ t('nav.audit') }}</span>
          </a-menu-item>
          <a-sub-menu key="settings-sub">
            <template #icon><SettingOutlined /></template>
            <template #title>{{ t('nav.settings') }}</template>
            <a-menu-item key="/settings" @click="$router.push('/settings')">{{ t('nav.settings') }}</a-menu-item>
            <a-menu-item key="/settings/notifications" @click="$router.push('/settings/notifications')">{{ t('nav.notifications') }}</a-menu-item>
            <a-menu-item key="/settings/auth" @click="$router.push('/settings/auth')">{{ t('nav.authSettings') }}</a-menu-item>
            <a-menu-item key="/settings/filter-rules" @click="$router.push('/settings/filter-rules')">{{ t('nav.filterRules') }}</a-menu-item>
          </a-sub-menu>
        </a-menu-item-group>
      </a-menu>
      <div class="sidebar-version">
        <span>{{ backendVersion }}</span>
        <a-button type="link" size="small" style="padding: 0 0 0 4px; font-size: 12px;" :loading="updateChecking" @click="checkUpdate">
          检查更新
        </a-button>
      </div>
    </a-layout-sider>

    <a-modal v-model:open="updateModalVisible" title="系统更新" :footer="null" width="500px">
      <div v-if="updateChecking" style="text-align: center; padding: 20px;">
        <a-spin tip="正在检查更新..." />
      </div>
      <div v-else-if="updateInfo">
        <a-descriptions :column="1" size="small" bordered>
          <a-descriptions-item label="当前版本">{{ updateInfo.current_version }}</a-descriptions-item>
          <a-descriptions-item label="最新版本">
            <span :style="{ color: updateInfo.has_update ? '#fa541c' : '#52c41a' }">
              {{ updateInfo.latest_version || '-' }}
            </span>
          </a-descriptions-item>
        </a-descriptions>
        <div v-if="updateInfo.error" style="color: #ff4d4f; margin-top: 12px;">{{ updateInfo.error }}</div>
        <div v-if="updateInfo.has_update" style="margin-top: 16px;">
          <a-alert v-if="updateInfo.release_notes" type="info" show-icon style="margin-bottom: 12px;">
            <template #message>更新内容</template>
            <pre style="white-space: pre-wrap; font-size: 12px; max-height: 200px; overflow: auto;">{{ updateInfo.release_notes }}</pre>
          </a-alert>
          <a-popconfirm title="确认更新？系统将自动重启。" ok-text="确认" cancel-text="取消" @confirm="performUpdate">
            <a-button type="primary" :loading="updating" style="width: 100%">
              {{ updating ? '正在下载并更新...' : '立即更新' }}
            </a-button>
          </a-popconfirm>
          <div style="text-align: center; margin-top: 8px; color: #999; font-size: 12px;">
            更新过程约 1-2 分钟，完成后系统自动重启
          </div>
        </div>
        <div v-else style="text-align: center; padding: 20px; color: #52c41a;">
          <a-check-circle-outlined style="font-size: 24px;" />
          <div style="margin-top: 8px;">已是最新版本</div>
        </div>
      </div>
    </a-modal>

    <a-layout>
      <a-layout-header class="header">
        <div class="header-right">
          <a-select
            :value="locale"
            size="small"
            style="width: 100px"
            @change="switchLocale"
          >
            <a-select-option value="zh-CN">{{ t('common.chinese') }}</a-select-option>
            <a-select-option value="en">English</a-select-option>
          </a-select>
          <a-button type="text" @click="toggleTheme">
            <template #icon>
              <BulbOutlined />
            </template>
          </a-button>
          <a-button type="text" danger @click="authStore.logout()">
            <template #icon><LogoutOutlined /></template>
            {{ t('auth.logout') }}
          </a-button>
        </div>
      </a-layout-header>
      <a-layout-content class="content">
        <router-view />
      </a-layout-content>
    </a-layout>
  </a-layout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { message as antMessage } from 'ant-design-vue'
import {
  DashboardOutlined,
  GlobalOutlined,
  CloudDownloadOutlined,
  NotificationOutlined,
  ThunderboltOutlined,
  CopyOutlined,
  SendOutlined,
  DownloadOutlined,
  SafetyOutlined,
  FileTextOutlined,
  AuditOutlined,
  SettingOutlined,
  BulbOutlined,
  LogoutOutlined,
  ApiOutlined,
  CloudSyncOutlined,
  CloudOutlined,
  VideoCameraOutlined,
  ClusterOutlined,
  HeartOutlined,
  FieldTimeOutlined,
  UnorderedListOutlined,
  StopOutlined,
} from '@ant-design/icons-vue'
import { useAuthStore } from '@/stores/auth'
import { useTheme } from '@/composables/useTheme'
import { useWebSocketStore } from '@/stores/websocket'
import { systemApi } from '@/api/system'
import type { UpdateInfo } from '@/api/system'

const { t, locale } = useI18n()
const route = useRoute()
const authStore = useAuthStore()
const { toggle: toggleTheme } = useTheme()
const wsStore = useWebSocketStore()

const backendVersion = ref('-')
const updateChecking = ref(false)
const updateModalVisible = ref(false)
const updateInfo = ref<UpdateInfo | null>(null)
const updating = ref(false)

function switchLocale(lang: string) {
  locale.value = lang
  localStorage.setItem('pt-forward-locale', lang)
}

const collapsed = ref(false)
const openKeys = ref<string[]>([])

const selectedKeys = computed(() => [route.path])

watch(
  () => route.path,
  (path) => {
    if (path.startsWith('/seeding')) {
      if (!openKeys.value.includes('seeding-sub')) {
        openKeys.value = [...openKeys.value, 'seeding-sub']
      }
    }
    if (path.startsWith('/publish')) {
      if (!openKeys.value.includes('publish-sub')) {
        openKeys.value = [...openKeys.value, 'publish-sub']
      }
    }
    if (path.startsWith('/settings')) {
      if (!openKeys.value.includes('settings-sub')) {
        openKeys.value = [...openKeys.value, 'settings-sub']
      }
    }
  },
  { immediate: true },
)

onMounted(() => {
  wsStore.connect()
  systemApi.health().then((resp) => {
    backendVersion.value = resp.data?.data?.version || '-'
  }).catch(() => {})
})

async function checkUpdate() {
  updateChecking.value = true
  updateModalVisible.value = true
  updateInfo.value = null
  try {
    const resp = await systemApi.checkUpdate()
    updateInfo.value = resp.data.data as unknown as UpdateInfo
  } catch {
    updateInfo.value = {
      has_update: false,
      current_version: backendVersion.value,
      latest_version: '',
      release_notes: '',
      download_url: '',
      error: '检查更新失败，请检查网络连接',
    }
  } finally {
    updateChecking.value = false
  }
}

async function performUpdate() {
  updating.value = true
  try {
    await systemApi.performUpdate()
    antMessage.loading('正在下载并更新，系统将在几秒后重启...', 0)
    setTimeout(() => {
      window.location.reload()
    }, 30000)
  } catch {
    antMessage.error('更新失败')
    updating.value = false
  }
}
</script>

<style scoped>
.logo {
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 18px;
  font-weight: 700;
  letter-spacing: 1px;
}

.sidebar-version {
  padding: 8px 16px 16px;
  text-align: center;
  color: rgba(255, 255, 255, 0.35);
  font-size: 11px;
  font-family: monospace;
  margin-top: auto;
}

.header {
  background: #fff;
  padding: 0 24px;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
}

.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.content {
  margin: 16px;
  padding: 24px;
  background: #fff;
  border-radius: 8px;
  min-height: 360px;
}

:deep(.dark) .header {
  background: #141414;
}

:deep(.dark) .content {
  background: #1f1f1f;
}
</style>
