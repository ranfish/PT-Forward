<template>
  <div>
    <a-row :gutter="16" style="margin-bottom: 24px">
      <a-col :span="6">
        <a-card>
          <a-statistic
            :title="t('dashboard.sitesOnline')"
            :value="`${overview.sites?.online || 0} / ${overview.sites?.total || 0}`"
            :value-style="{ color: '#1890ff' }"
          >
            <template #prefix><GlobalOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic
            :title="t('dashboard.downloadersOnline')"
            :value="`${overview.downloaders?.online || 0} / ${overview.downloaders?.total || 0}`"
            :value-style="{ color: '#52c41a' }"
          >
            <template #prefix><CloudDownloadOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic
            :title="t('dashboard.seedingTorrents')"
            :value="overview.torrents?.seeding || 0"
            :value-style="{ color: '#faad14' }"
          >
            <template #prefix><ThunderboltOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic
            :title="t('dashboard.pendingPublish')"
            :value="overview.publish?.pendingCount || 0"
            :value-style="{ color: '#722ed1' }"
          >
            <template #prefix><SendOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
    </a-row>

    <a-row :gutter="16" style="margin-bottom: 24px">
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('dashboard.reseedTodayTotal')" :value="`${overview.reseed?.todayCount || 0} / ${overview.reseed?.totalCount || 0}`" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('dashboard.publishTodayTotal')" :value="`${overview.publish?.todayCount || 0} / ${overview.publish?.totalCount || 0}`" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('dashboard.downloadingTorrents')" :value="overview.torrents?.downloading || 0" />
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card>
          <a-statistic :title="t('dashboard.goroutines')" :value="overview.system?.goroutines || 0" />
        </a-card>
      </a-col>
    </a-row>

    <a-row :gutter="16" style="margin-bottom: 24px">
      <a-col :span="6">
        <a-card size="small" :bordered="true" style="height: 100%">
          <template #title><ThunderboltOutlined /> 刷流</template>
          <a-descriptions :column="1" size="small">
            <a-descriptions-item label="活跃种子">{{ sysDash?.seeding?.active ?? '-' }}</a-descriptions-item>
            <a-descriptions-item label="今日删种">{{ sysDash?.seeding?.deleted_today ?? '-' }}</a-descriptions-item>
            <a-descriptions-item label="RSS 订阅">{{ sysDash?.seeding?.rss_enabled ?? '-' }}</a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card size="small" :bordered="true" style="height: 100%">
          <template #title><CloudDownloadOutlined /> 下载</template>
          <a-descriptions :column="1" size="small">
            <a-descriptions-item label="下载中">{{ sysDash?.download?.downloading ?? '-' }}</a-descriptions-item>
            <a-descriptions-item label="今日完成">{{ sysDash?.download?.completed_today ?? '-' }}</a-descriptions-item>
            <a-descriptions-item label="待转移">{{ sysDash?.download?.transfer_pending ?? '-' }}</a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card size="small" :bordered="true" style="height: 100%">
          <template #title><ApiOutlined /> 辅种</template>
          <a-descriptions :column="1" size="small">
            <a-descriptions-item label="活跃任务">{{ sysDash?.reseed?.active_tasks ?? '-' }}</a-descriptions-item>
            <a-descriptions-item label="待注入">{{ sysDash?.reseed?.pending_injection ?? '-' }}</a-descriptions-item>
            <a-descriptions-item label="今日注入">{{ sysDash?.reseed?.injected_today ?? '-' }}</a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>
      <a-col :span="6">
        <a-card size="small" :bordered="true" style="height: 100%">
          <template #title><SendOutlined /> 发布</template>
          <a-descriptions :column="1" size="small">
            <a-descriptions-item label="发布中">{{ sysDash?.publish?.publishing ?? '-' }}</a-descriptions-item>
            <a-descriptions-item label="今日完成">{{ sysDash?.publish?.done_today ?? '-' }}</a-descriptions-item>
            <a-descriptions-item label="待发布">{{ sysDash?.publish?.pending ?? '-' }}</a-descriptions-item>
          </a-descriptions>
        </a-card>
      </a-col>
    </a-row>

    <a-collapse :bordered="false" style="margin-bottom: 24px">
      <a-collapse-panel key="seeding" header="刷流详情">
        <a-descriptions :column="4" size="small" v-if="seedingMonitor">
          <a-descriptions-item v-for="(v, k) in (seedingMonitor as any).by_status" :key="k" :label="k">{{ v }}</a-descriptions-item>
          <a-descriptions-item label="今日删种">{{ (seedingMonitor as any).deleted_today }}</a-descriptions-item>
        </a-descriptions>
        <a-empty v-else description="暂无数据" />
      </a-collapse-panel>
      <a-collapse-panel key="reseed" header="辅种详情">
        <a-descriptions :column="4" size="small" v-if="reseedMonitor">
          <a-descriptions-item v-for="(v, k) in (reseedMonitor as any).by_status" :key="k" :label="k">{{ v }}</a-descriptions-item>
        </a-descriptions>
        <a-empty v-else description="暂无数据" />
      </a-collapse-panel>
      <a-collapse-panel key="publish" header="发布详情">
        <a-descriptions :column="4" size="small" v-if="publishMonitor">
          <a-descriptions-item v-for="(v, k) in (publishMonitor as any).by_status" :key="k" :label="k">{{ v }}</a-descriptions-item>
        </a-descriptions>
        <a-empty v-else description="暂无数据" />
      </a-collapse-panel>
    </a-collapse>

    <a-card :title="t('dashboard.trends7d')" style="margin-bottom: 24px">
      <div ref="chartRef" style="height: 320px; width: 100%" />
    </a-card>

    <a-card title="流量统计" style="margin-bottom: 24px">
      <template #extra>
        <a-select v-model:value="trafficClientId" style="width: 160px" @change="fetchTraffic">
          <a-select-option value="">全部</a-select-option>
          <a-select-option v-for="c in trafficClients" :key="c" :value="c">{{ c }}</a-select-option>
        </a-select>
        <a-radio-group v-model:value="trafficDays" size="small" style="margin-left: 8px" @change="fetchTraffic">
          <a-radio-button :value="1">24h</a-radio-button>
          <a-radio-button :value="7">7天</a-radio-button>
          <a-radio-button :value="30">30天</a-radio-button>
        </a-radio-group>
      </template>
      <div ref="trafficChartRef" style="height: 280px; width: 100%" />
    </a-card>

    <a-card :title="t('dashboard.recentActivity')">
      <a-table
        :columns="activityColumns"
        :data-source="activities"
        :loading="loading"
        :pagination="{
          current: activityPage,
          pageSize: activitySize,
          total: activityTotal,
          showSizeChanger: true,
          showTotal: (total: number) => t('common.totalCount', { count: total }),
        }"
        row-key="id"
        size="small"
        @change="(pag: { current: number; pageSize: number }) => { activityPage = pag.current; activitySize = pag.pageSize; fetchActivities() }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'title'">
            <a v-if="record.detail_url" :href="record.detail_url" target="_blank" style="color: #1890ff">{{ record.title }}</a>
            <span v-else>{{ record.title }}</span>
          </template>
          <template v-if="column.key === 'info_hash'">
            <span v-if="/^[0-9a-f]{40}$/i.test(record.info_hash)" style="cursor:pointer;font-family:monospace;font-size:12px" @click="copyHash(record.info_hash)">{{ record.info_hash }}</span>
            <span v-else style="color: #999">-</span>
          </template>
          <template v-if="column.key === 'size'">
            {{ formatBytes(record.size) }}
          </template>
        </template>
      </a-table>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import * as echarts from 'echarts'
import {
  GlobalOutlined,
  CloudDownloadOutlined,
  ThunderboltOutlined,
  SendOutlined,
  ApiOutlined,
} from '@ant-design/icons-vue'
import { dashboardApi, type TrendPoint, type SystemDashboard } from '@/api/dashboard'
import { statsApi, type TrafficHourlyPoint } from '@/api/stats'
import { useWebSocketStore } from '@/stores/websocket'
import { formatBytes, formatTime, copyToClipboard } from '@/utils/format'

const torrentStatusLabels: Record<string, string> = {
  pushed: '已入库',
  seen: '已发现',
  skipped: '已跳过',
  filtered: '已过滤',
  error: '出错',
  pending: '待处理',
}

const discountLabels: Record<string, string> = {
  NONE: '-',
  FREE: '免费',
  '2XFREE': '2x免费',
  '2XUP': '2x上传',
  '2X50': '2x上传 50%下载',
  PERCENT_25: '75折',
  PERCENT_30: '7折',
  PERCENT_50: '5折',
  PERCENT_70: '3折',
  PERCENT_75: '25折',
  CUSTOM: '自定义',
}

function discountLabel(s: string): string {
  return torrentStatusLabels[s] || discountLabels[s] || s
}

function copyHash(text: string) {
  copyToClipboard(text)
  message.success(t('common.copied'))
}

interface ActivityItem {
  id: number
  title: string
  site_name: string
  info_hash: string
  detail_url: string
  size: number
  status: string
  created_at: string
}

interface DashboardOverview {
  sites?: { online?: number; total?: number }
  downloaders?: { online?: number; total?: number }
  torrents?: { seeding?: number; downloading?: number }
  publish?: { pendingCount?: number; todayCount?: number; totalCount?: number }
  reseed?: { todayCount?: number; totalCount?: number }
  system?: { goroutines?: number }
}

interface WSMessage {
  type?: string
  payload?: Record<string, unknown>
  timestamp?: string
}

const { t } = useI18n()
const loading = ref(false)
const overview = ref<DashboardOverview>({})
const sysDash = ref<SystemDashboard | null>(null)
const seedingMonitor = ref<Record<string, unknown> | null>(null)
const reseedMonitor = ref<Record<string, unknown> | null>(null)
const publishMonitor = ref<Record<string, unknown> | null>(null)
const activities = ref<ActivityItem[]>([])
const activityPage = ref(1)
const activitySize = ref(20)
const activityTotal = ref(0)
const chartRef = ref<HTMLElement>()
let chartInstance: echarts.ECharts | null = null
let trafficChart: echarts.ECharts | null = null
let resizeTimer: ReturnType<typeof setTimeout> | null = null
const wsStore = useWebSocketStore()
const trafficClientId = ref('')
const trafficDays = ref(7)
const trafficClients = ref<string[]>([])
const trafficChartRef = ref<HTMLElement>()

const activityColumns = [
  { title: 'ID', dataIndex: 'id', key: 'id', width: 60 },
  { title: t('dashboard.torrentTitle'), dataIndex: 'title', key: 'title', ellipsis: true, width: 220 },
  { title: t('common.site'), dataIndex: 'site_name', key: 'site_name', width: 80 },
  { title: '种子ID', dataIndex: 'torrent_id', key: 'torrent_id', width: 100 },
  { title: 'InfoHash', key: 'info_hash', width: 280 },
  { title: t('common.size'), key: 'size', width: 100 },
  { title: t('common.status'), dataIndex: 'status', key: 'status', width: 100, customRender: ({ text }: { text: string }) => discountLabel(text) },
  { title: t('common.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 180, customRender: ({ text }: { text: string }) => formatTime(text) },
]


function initChart(points: TrendPoint[]) {
  if (!chartRef.value) return
  if (chartInstance) {
    chartInstance.dispose()
  }
  chartInstance = echarts.init(chartRef.value)
  const dates = points.map(p => p.date)
  chartInstance.setOption({
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'cross', label: { backgroundColor: '#6a7985' } },
    },
    legend: { data: [t('dashboard.chartEvents'), t('dashboard.chartRSS'), t('dashboard.chartPublish'), t('dashboard.chartReseed')], bottom: 0 },
    grid: { left: 50, right: 30, top: 20, bottom: 40 },
    xAxis: { type: 'category', data: dates, boundaryGap: true },
    yAxis: { type: 'value', minInterval: 1 },
    series: [
      {
        name: t('dashboard.chartEvents'), type: 'line', smooth: true,
        data: points.map(p => p.events),
        itemStyle: { color: '#1890ff' },
        lineStyle: { width: 2 },
        areaStyle: { opacity: 0.1 },
      },
      {
        name: t('dashboard.chartRSS'), type: 'line', smooth: true,
        data: points.map(p => p.rss),
        itemStyle: { color: '#52c41a' },
        lineStyle: { width: 2 },
      },
      {
        name: t('dashboard.chartPublish'), type: 'line', smooth: true,
        data: points.map(p => p.publish),
        itemStyle: { color: '#722ed1' },
        lineStyle: { width: 2 },
      },
      {
        name: t('dashboard.chartReseed'), type: 'line', smooth: true,
        data: points.map(p => p.reseed),
        itemStyle: { color: '#faad14' },
        lineStyle: { width: 2 },
      },
    ],
  })
}

function handleResize() {
  if (resizeTimer) clearTimeout(resizeTimer)
  resizeTimer = setTimeout(() => {
    chartInstance?.resize()
    trafficChart?.resize()
  }, 200)
}

function handleWSMessage(msg: WSMessage) {
  if (!msg || !msg.type) return
  if (msg.type === 'torrent.added') {
    const p = msg.payload || {}
    activities.value.unshift({
      id: (p.eventId as number) || Date.now(),
      title: (p.title as string) || '',
      site_name: (p.siteName as string) || '',
      info_hash: '',
      detail_url: '',
      size: typeof p.size === 'number' ? p.size : 0,
      status: (p.discount as string) || '',
      created_at: msg.timestamp || new Date().toISOString(),
    })
    if (activities.value.length > activitySize.value) {
      activities.value = activities.value.slice(0, activitySize.value)
    }
  }
  if (msg.type === 'system.site.frozen') {
    fetchData()
  }
}

async function fetchActivities() {
  try {
    const resp = await dashboardApi.getActivities(activityPage.value, activitySize.value)
    const body = resp.data.data || { items: [], total: 0 }
    activities.value = (body.items || []) as ActivityItem[]
    activityTotal.value = body.total || 0
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function fetchData() {
  loading.value = true
  try {
    const [overviewResp, trendsResp, sysDashResp] = await Promise.all([
      dashboardApi.getOverview(),
      dashboardApi.getTrends(7),
      dashboardApi.getSystemDashboard(),
    ])
    overview.value = overviewResp.data.data || {}
    sysDash.value = sysDashResp.data.data || null
    const trendData = trendsResp.data.data || { trends: [] }
    await nextTick()
    initChart(trendData.trends || [])
    await fetchActivities()

    const [seedMon, reseedMon, pubMon] = await Promise.all([
      dashboardApi.getSeedingMonitor().catch(() => null),
      dashboardApi.getReseedMonitor().catch(() => null),
      dashboardApi.getPublishMonitor().catch(() => null),
    ])
    if (seedMon?.data?.data) seedingMonitor.value = seedMon.data.data as any
    if (reseedMon?.data?.data) reseedMonitor.value = reseedMon.data.data as any
    if (pubMon?.data?.data) publishMonitor.value = pubMon.data.data as any
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchData()
  fetchTraffic()
  window.addEventListener('resize', handleResize)
  wsStore.subscribe(['dashboard', 'torrent', 'system'])
})

watch(() => wsStore.lastMessage, (msg) => {
  if (msg) handleWSMessage(msg)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  if (resizeTimer) clearTimeout(resizeTimer)
  chartInstance?.dispose()
  trafficChart?.dispose()
  wsStore.unsubscribe(['dashboard', 'torrent', 'system'])
})

async function fetchTraffic() {
  try {
    const { data } = await statsApi.getTrafficHourly(trafficClientId.value || undefined, trafficDays.value)
    const items: TrafficHourlyPoint[] = (data.data as any)?.items || []
    const clientSet = new Set<string>()
    for (const item of items) {
      if (item.client_id) clientSet.add(item.client_id)
    }
    trafficClients.value = [...clientSet].sort()
    await nextTick()
    initTrafficChart(items)
  } catch {
    // traffic stats optional, fail silently
  }
}

function initTrafficChart(items: TrafficHourlyPoint[]) {
  if (!trafficChartRef.value) return
  if (trafficChart) trafficChart.dispose()
  trafficChart = echarts.init(trafficChartRef.value)

  const byHour = new Map<string, { up: number; down: number }>()
  for (const item of items) {
    const key = item.hour
    const existing = byHour.get(key) || { up: 0, down: 0 }
    existing.up += item.uploaded_delta || 0
    existing.down += item.downloaded_delta || 0
    byHour.set(key, existing)
  }

  const hours = [...byHour.keys()].sort()
  const uploadData = hours.map(h => Math.round(((byHour.get(h)?.up || 0) / 1024 / 1024 / 1024) * 100) / 100)
  const downloadData = hours.map(h => Math.round(((byHour.get(h)?.down || 0) / 1024 / 1024 / 1024) * 100) / 100)

  trafficChart.setOption({
    tooltip: { trigger: 'axis', axisPointer: { type: 'cross' } },
    legend: { data: ['上传 (GB)', '下载 (GB)'], bottom: 0 },
    grid: { left: 50, right: 30, top: 20, bottom: 40 },
    xAxis: { type: 'category', data: hours, axisLabel: { formatter: (v: string) => v.substring(5, 16) } },
    yAxis: { type: 'value', name: 'GB' },
    series: [
      { name: '上传 (GB)', type: 'bar', data: uploadData, itemStyle: { color: '#52c41a' } },
      { name: '下载 (GB)', type: 'bar', data: downloadData, itemStyle: { color: '#1890ff' } },
    ],
  })
}
</script>
