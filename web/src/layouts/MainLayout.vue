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
        v-model:selectedKeys="selectedKeys"
        v-model:openKeys="openKeys"
        theme="dark"
        mode="inline"
      >
        <a-menu-item-group :title="'资源管理'">
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

        <a-menu-item-group :title="'核心功能'">
          <a-sub-menu key="seeding-sub">
            <template #icon><ThunderboltOutlined /></template>
            <template #title>{{ t('nav.seeding') }}</template>
            <a-menu-item key="/seeding" @click="$router.push('/seeding')">{{ t('nav.seedingDashboard') }}</a-menu-item>
            <a-menu-item key="/seeding/rules" @click="$router.push('/seeding/rules')">{{ t('nav.seedingRules') }}</a-menu-item>
            <a-menu-item key="/seeding/torrents" @click="$router.push('/seeding/torrents')">{{ t('nav.seedingTorrents') }}</a-menu-item>
            <a-menu-item key="/seeding/stats" @click="$router.push('/seeding/stats')">{{ t('nav.seedingStats') }}</a-menu-item>
          </a-sub-menu>
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

        <a-menu-item-group :title="'系统'">
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
    </a-layout-sider>

    <a-layout>
      <a-layout-header class="header">
        <div class="header-right">
          <a-select
            :value="locale"
            size="small"
            style="width: 100px"
            @change="switchLocale"
          >
            <a-select-option value="zh-CN">中文</a-select-option>
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
import {
  DashboardOutlined,
  GlobalOutlined,
  CloudDownloadOutlined,
  NotificationOutlined,
  ThunderboltOutlined,
  CopyOutlined,
  SendOutlined,
  SafetyOutlined,
  FileTextOutlined,
  SettingOutlined,
  BulbOutlined,
  LogoutOutlined,
  ApiOutlined,
  CloudSyncOutlined,
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

const { t, locale } = useI18n()
const route = useRoute()
const authStore = useAuthStore()
const { toggle: toggleTheme } = useTheme()
const wsStore = useWebSocketStore()

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
})
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
