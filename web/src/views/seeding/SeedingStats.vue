<template>
  <div>
    <a-row :gutter="16" style="margin-bottom: 24px">
      <a-col :span="4">
        <a-card>
          <a-statistic :title="t('seeding.totalUpload')" :value="formatSize(overview.totalUploadBytes || 0)" :value-style="{ color: '#52c41a' }">
            <template #prefix><CloudUploadOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="4">
        <a-card>
          <a-statistic :title="t('seeding.todayUpload')" :value="formatSize(overview.todayUploadBytes || 0)" :value-style="{ color: '#73d13d' }">
            <template #prefix><CloudUploadOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="4">
        <a-card>
          <a-statistic :title="t('seeding.totalDownload')" :value="formatSize(overview.totalDownloadBytes || 0)" :value-style="{ color: '#1890ff' }">
            <template #prefix><CloudDownloadOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="4">
        <a-card>
          <a-statistic :title="t('seeding.todayDownload')" :value="formatSize(overview.todayDownloadBytes || 0)" :value-style="{ color: '#40a9ff' }">
            <template #prefix><CloudDownloadOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="4">
        <a-card>
          <a-statistic :title="t('seeding.todayAdded')" :value="overview.todayAdded || 0" :value-style="{ color: '#faad14' }">
            <template #prefix><ClockCircleOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
      <a-col :span="4">
        <a-card>
          <a-statistic :title="t('seeding.globalRatio')" :value="overview.globalRatio === -1 ? '∞' : (overview.globalRatio || 0)" :precision="overview.globalRatio === -1 ? undefined : 2" :value-style="{ color: '#722ed1' }">
            <template #prefix><PieChartOutlined /></template>
          </a-statistic>
        </a-card>
      </a-col>
    </a-row>

    <a-card :title="t('seeding.siteStats')" style="margin-bottom: 24px">
      <a-table
        :columns="siteColumns"
        :data-source="siteStats"
        :loading="siteLoading"
        :pagination="false"
        row-key="siteName"
        size="small"
      />
    </a-card>

    <a-row :gutter="16" style="margin-bottom: 24px">
      <a-col :span="12">
        <a-card :title="t('seeding.speedTrend')">
          <div style="margin-bottom: 12px">
            <a-radio-group v-model:value="speedRange" size="small" @change="fetchSpeedTrend">
              <a-radio-button value="24h">24h</a-radio-button>
              <a-radio-button value="7d">7d</a-radio-button>
              <a-radio-button value="30d">30d</a-radio-button>
            </a-radio-group>
          </div>
          <div ref="speedChartRef" style="height: 280px" />
        </a-card>
      </a-col>
      <a-col :span="12">
        <a-card :title="t('seeding.siteTrend')">
          <div style="margin-bottom: 12px; display: flex; align-items: center; gap: 8px">
            <a-select v-model:value="trendSite" size="small" style="width: 150px" @change="fetchSiteTrend">
              <a-select-option v-for="s in siteStats" :key="s.siteName as string" :value="s.siteName as string">
                {{ s.siteName }}
              </a-select-option>
            </a-select>
            <a-radio-group v-model:value="siteRange" size="small" @change="fetchSiteTrend">
              <a-radio-button value="24h">24h</a-radio-button>
              <a-radio-button value="7d">7d</a-radio-button>
              <a-radio-button value="30d">30d</a-radio-button>
            </a-radio-group>
          </div>
          <div ref="siteChartRef" style="height: 280px" />
        </a-card>
      </a-col>
    </a-row>

    <a-card :title="t('seeding.torrentRanking')">
      <a-table
        :columns="torrentColumns"
        :data-source="torrentStats"
        :loading="torrentLoading"
        :pagination="{ pageSize: 20 }"
        row-key="id"
        size="small"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'status'">
            {{ translateSeedingStatus(record.status) }}
          </template>
          <template v-else-if="column.key === 'is_free'">
            <a-tag :color="record.is_free ? 'green' : 'default'">{{ record.is_free ? t('common.yes') : t('common.no') }}</a-tag>
          </template>
        </template>
      </a-table>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, nextTick, onBeforeUnmount } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import * as echarts from 'echarts'
import {
  CloudUploadOutlined,
  CloudDownloadOutlined,
  ClockCircleOutlined,
  PieChartOutlined,
} from '@ant-design/icons-vue'
import { seedingApi, seedingStatsApi } from '@/api/seeding'
import { useEnumLabels } from '@/utils/enumLabels'
import type { SeedingStatsOverview, SeedingSiteStat, SeedingTorrentRecord, SeedingClientConfig, SeedingSpeedTrendPoint, SeedingSiteTrendPoint } from '@/api/types'
import { formatTime } from '@/utils/format'

const { t } = useI18n()
const { translateSeedingStatus } = useEnumLabels()
const siteLoading = ref(false)
const torrentLoading = ref(false)
const overview = ref<SeedingStatsOverview>({
  totalTorrents: 0,
  activeTorrents: 0,
  pausedTorrents: 0,
  realSeeding: 0,
  realDownloading: 0,
  totalUploadBytes: 0,
  totalDownloadBytes: 0,
  todayUploadBytes: 0,
  todayDownloadBytes: 0,
  globalRatio: 0,
  todayDeleted: 0,
  todayAdded: 0,
})
const siteStats = ref<SeedingSiteStat[]>([])
const torrentStats = ref<SeedingTorrentRecord[]>([])
const configs = ref<SeedingClientConfig[]>([])

const siteColumns = [
  { title: t('common.site'), dataIndex: 'siteName', key: 'siteName' },
  { title: t('seeding.seedingCount'), dataIndex: 'count', key: 'count', width: 80 },
]

const torrentColumns = [
  { title: t('seeding.torrentId'), dataIndex: 'torrent_id', key: 'torrent_id', ellipsis: true },
  { title: t('common.site'), dataIndex: 'site_name', key: 'site_name', width: 120 },
  { title: t('common.status'), dataIndex: 'status', key: 'status', width: 100 },
  { title: t('seeding.free'), dataIndex: 'is_free', key: 'is_free', width: 60 },
  { title: t('common.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 180, customRender: ({ text }: { text: string }) => formatTime(text) },
]

function formatSize(bytes: number) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let val = bytes
  while (val >= 1024 && i < units.length - 1) { val /= 1024; i++ }
  return `${val.toFixed(1)} ${units[i]}`
}

async function fetchAll() {
  try {
    const [overviewResp, siteResp, torrentResp] = await Promise.all([
      seedingApi.getStatsOverview(),
      seedingApi.getStatsBySite(),
      seedingApi.getStatsTorrents(1, 20),
    ])
    overview.value = (overviewResp.data.data as SeedingStatsOverview) || overview.value
    siteStats.value = siteResp.data.data?.items ?? []
    torrentStats.value = (torrentResp.data.data?.items || []) as SeedingTorrentRecord[]

    const configResp = await seedingApi.getConfig()
    const configData = configResp.data.data
    configs.value = configData?.items ?? []

    await nextTick()
    if (siteStats.value.length > 0 && !trendSite.value) {
      trendSite.value = siteStats.value[0].siteName
    }
    fetchSiteTrend()
    fetchSpeedTrend()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

const speedRange = ref('24h')
const siteRange = ref('7d')
const trendSite = ref('')
const speedChartRef = ref<HTMLElement>()
const siteChartRef = ref<HTMLElement>()
let speedChart: echarts.ECharts | null = null
let siteChart: echarts.ECharts | null = null

function formatSpeed(bytesPerSec: number) {
  if (!bytesPerSec) return '0 B/s'
  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s']
  let i = 0
  let val = bytesPerSec
  while (val >= 1024 && i < units.length - 1) { val /= 1024; i++ }
  return `${val.toFixed(1)} ${units[i]}`
}

async function fetchSpeedTrend() {
  if (!configs.value?.length) return
  const clientId = configs.value[0]?.client_id as string
  if (!clientId) return
  try {
    const resp = await seedingStatsApi.downloaderSpeedTrend(clientId, speedRange.value)
    const data = resp.data.data?.points
    if (!data?.length) return

    await nextTick()
    if (!speedChart && speedChartRef.value) {
      speedChart = echarts.init(speedChartRef.value)
    }
    if (!speedChart) return

    const timestamps = data.map((p: SeedingSpeedTrendPoint) => {
      const ts = p.timestamp
      return ts.length > 16 ? ts.substring(5, 16) : ts
    })
    const upload = data.map((p: SeedingSpeedTrendPoint) => p.uploadSpeed / 1024 / 1024)
    const download = data.map((p: SeedingSpeedTrendPoint) => p.downloadSpeed / 1024 / 1024)

    speedChart.setOption({
      tooltip: { trigger: 'axis', formatter: (params: unknown) => {
        const items = params as { name: string; seriesName: string; value: number }[]
        return items.map(i => `${i.seriesName}: ${formatSpeed(i.value * 1024 * 1024)}`).join('<br/>')
      }},
      grid: { left: 60, right: 20, top: 20, bottom: 30 },
      xAxis: { type: 'category', data: timestamps, axisLabel: { fontSize: 10 } },
      yAxis: { type: 'value', axisLabel: { formatter: '{value} MB/s' } },
      series: [
        { name: t('seeding.uploadSpeed'), type: 'line', data: upload, smooth: true, lineStyle: { width: 2 }, itemStyle: { color: '#52c41a' } },
        { name: t('seeding.downloadSpeed'), type: 'line', data: download, smooth: true, lineStyle: { width: 2 }, itemStyle: { color: '#1890ff' } },
      ],
    })
  } catch {
  }
}

async function fetchSiteTrend() {
  if (!trendSite.value) return
  try {
    const resp = await seedingStatsApi.siteTrend(trendSite.value, siteRange.value)
    const data = resp.data.data?.trends
    if (!data?.length) return

    await nextTick()
    if (!siteChart && siteChartRef.value) {
      siteChart = echarts.init(siteChartRef.value)
    }
    if (!siteChart) return

    const dates = data.map((p: SeedingSiteTrendPoint) => p.date.substring(5))
    const counts = data.map((p: SeedingSiteTrendPoint) => p.count)

    siteChart.setOption({
      tooltip: { trigger: 'axis' },
      grid: { left: 50, right: 20, top: 20, bottom: 30 },
      xAxis: { type: 'category', data: dates, axisLabel: { fontSize: 10 } },
      yAxis: { type: 'value' },
      series: [
        { name: t('seeding.seedingTrendCount'), type: 'bar', data: counts, itemStyle: { color: '#1890ff' } },
      ],
    })
  } catch {
  }
}

function handleResize() {
  speedChart?.resize()
  siteChart?.resize()
}

onMounted(() => {
  fetchAll()
  window.addEventListener('resize', handleResize)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', handleResize)
  speedChart?.dispose()
  siteChart?.dispose()
})
</script>
