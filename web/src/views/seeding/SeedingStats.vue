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

    <a-alert type="info" :message="t('seeding.statsScopeNote')" style="margin-bottom: 24px" show-icon />

    <a-card :title="t('seeding.siteStats')" style="margin-bottom: 24px">
      <a-table
        :columns="siteColumns"
        :data-source="siteStats"
        :loading="siteLoading"
        :pagination="false"
        row-key="siteName"
        size="small"
      >
        <template #summary>
          <a-table-summary fixed>
            <a-table-summary-row>
              <a-table-summary-cell>{{ t('common.subtotal') }}</a-table-summary-cell>
              <a-table-summary-cell>{{ siteSummary.seedingCount }}</a-table-summary-cell>
              <a-table-summary-cell>{{ siteSummary.todayAdded }}</a-table-summary-cell>
              <a-table-summary-cell>{{ siteSummary.todayDeleted }}</a-table-summary-cell>
              <a-table-summary-cell>{{ siteSummary.activeFree }}</a-table-summary-cell>
              <a-table-summary-cell>{{ siteSummary.activeNonFree }}</a-table-summary-cell>
              <a-table-summary-cell>{{ formatSize(siteSummary.todayUploadBytes) }}</a-table-summary-cell>
              <a-table-summary-cell>{{ formatSize(siteSummary.historyUploadBytes) }}</a-table-summary-cell>
              <a-table-summary-cell>{{ siteSummary.totalCount }}</a-table-summary-cell>
            </a-table-summary-row>
          </a-table-summary>
        </template>
      </a-table>
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

    <a-row :gutter="16" style="margin-bottom: 24px">
      <a-col :span="12">
        <a-card :title="t('seeding.todayUploadDist')">
          <div ref="todayPieRef" style="height: 280px" />
        </a-card>
      </a-col>
      <a-col :span="12">
        <a-card :title="t('seeding.historyUploadDist')">
          <div ref="historyPieRef" style="height: 280px" />
        </a-card>
      </a-col>
    </a-row>

    <a-card :title="t('seeding.torrentRanking')">
      <div style="margin-bottom: 12px; display: flex; align-items: center; gap: 12px">
        <a-select v-model:value="torrentSort" size="small" style="width: 130px" @change="fetchTorrentStats">
          <a-select-option value="uploaded">{{ t('seeding.sortByUpload') }}</a-select-option>
          <a-select-option value="size">{{ t('seeding.sortBySize') }}</a-select-option>
          <a-select-option value="time">{{ t('seeding.sortByTime') }}</a-select-option>
        </a-select>
        <a-select v-model:value="torrentStatus" size="small" style="width: 120px" @change="fetchTorrentStats">
          <a-select-option value="">{{ t('seeding.allStatus') }}</a-select-option>
          <a-select-option value="seeding">{{ t('seeding.seedingStatus') }}</a-select-option>
          <a-select-option value="deleted">{{ t('seeding.deletedStatus') }}</a-select-option>
        </a-select>
      </div>
      <a-table
        :columns="torrentColumns"
        :data-source="torrentStats"
        :loading="torrentLoading"
        :pagination="{ pageSize: 20, total: torrentTotal, current: torrentPage, onChange: onTorrentPageChange }"
        row-key="id"
        size="small"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'title'">
            <a v-if="record.detail_url" :href="record.detail_url" target="_blank" style="color: #1890ff">{{ record.title || record.torrent_id }}</a>
            <span v-else>{{ record.title || record.torrent_id }}</span>
          </template>
          <template v-else-if="column.key === 'info_hash'">
            <span style="cursor:pointer;font-family:monospace;font-size:12px" @click="copyHash(record.info_hash)">{{ record.info_hash }}</span>
          </template>
          <template v-else-if="column.key === 'status'">
            {{ translateSeedingStatus(record.status) }}
          </template>
          <template v-else-if="column.key === 'is_free'">
            <a-tag :color="record.is_free ? 'green' : 'default'">{{ record.is_free ? t('common.yes') : t('common.no') }}</a-tag>
          </template>
          <template v-else-if="column.key === 'discount'">
            <a-tag v-if="record.discount && record.discount !== 'NONE'" color="blue">{{ record.discount }}</a-tag>
            <span v-else>-</span>
          </template>
          <template v-else-if="column.key === 'has_hr'">
            <a-tag :color="record.has_hr ? 'red' : 'default'">{{ record.has_hr ? t('common.yes') : t('common.no') }}</a-tag>
          </template>
          <template v-else-if="column.key === 'latest_upload'">
            <span style="color: #52c41a; font-weight: 500">{{ formatSize(record.latest_upload || 0) }}</span>
          </template>
          <template v-else-if="column.key === 'torrent_size'">
            {{ formatSize(record.torrent_size || 0) }}
          </template>
          <template v-else-if="column.key === 'flushed_at'">
            {{ record.flushed_at ? formatTime(record.flushed_at) : '-' }}
          </template>
        </template>
      </a-table>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick, onBeforeUnmount } from 'vue'
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
import { formatTime, copyToClipboard } from '@/utils/format'

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
const torrentStats = ref<any[]>([])
const torrentTotal = ref(0)
const torrentPage = ref(1)
const torrentSort = ref('uploaded')
const torrentStatus = ref('seeding')
const configs = ref<SeedingClientConfig[]>([])

const siteSummary = computed(() => {
  const s = { seedingCount: 0, todayAdded: 0, todayDeleted: 0, activeFree: 0, activeNonFree: 0, todayUploadBytes: 0, historyUploadBytes: 0, totalCount: 0 }
  for (const r of siteStats.value) {
    s.seedingCount += r.seedingCount
    s.todayAdded += r.todayAdded
    s.todayDeleted += r.todayDeleted
    s.activeFree += r.activeFree
    s.activeNonFree += r.activeNonFree
    s.todayUploadBytes += r.todayUploadBytes
    s.historyUploadBytes += r.historyUploadBytes
    s.totalCount += r.totalCount
  }
  return s
})
const siteColumns = [
  { title: t('common.site'), dataIndex: 'siteName', key: 'siteName', width: 100 },
  { title: t('seeding.seedingCount'), dataIndex: 'seedingCount', key: 'seedingCount', width: 100 },
  { title: t('seeding.todayAdded'), dataIndex: 'todayAdded', key: 'todayAdded', width: 100 },
  { title: t('seeding.todayDeleted'), dataIndex: 'todayDeleted', key: 'todayDeleted', width: 100 },
  { title: t('seeding.activeFree'), dataIndex: 'activeFree', key: 'activeFree', width: 100 },
  { title: t('seeding.activeNonFree'), dataIndex: 'activeNonFree', key: 'activeNonFree', width: 120 },
  { title: t('seeding.todayUpload'), dataIndex: 'todayUploadBytes', key: 'todayUploadBytes', width: 120, customRender: ({ text }: { text: number }) => formatSize(text || 0) },
  { title: t('seeding.historyUpload'), dataIndex: 'historyUploadBytes', key: 'historyUploadBytes', width: 120, customRender: ({ text }: { text: number }) => formatSize(text || 0) },
  { title: t('seeding.totalCount'), dataIndex: 'totalCount', key: 'totalCount', width: 80 },
]

const torrentColumns = [
  { title: t('seeding.torrentName'), dataIndex: 'title', key: 'title', width: 280, ellipsis: true },
  { title: t('common.site'), dataIndex: 'site_name', key: 'site_name', width: 55 },
  { title: t('seeding.torrentId'), dataIndex: 'torrent_id', key: 'torrent_id', width: 55, ellipsis: true },
  { title: t('common.status'), dataIndex: 'status', key: 'status', width: 50 },
  { title: t('seeding.free'), dataIndex: 'is_free', key: 'is_free', width: 35 },
  { title: t('seeding.discount'), dataIndex: 'discount', key: 'discount', width: 50 },
  { title: t('seeding.hr'), dataIndex: 'has_hr', key: 'has_hr', width: 35 },
  { title: t('seeding.size'), dataIndex: 'torrent_size', key: 'torrent_size', width: 65 },
  { title: t('seeding.upload'), dataIndex: 'latest_upload', key: 'latest_upload', width: 75 },
  { title: t('seeding.infoHash'), dataIndex: 'info_hash', key: 'info_hash', width: 180 },
  { title: t('seeding.flushedAt'), dataIndex: 'flushed_at', key: 'flushed_at', width: 120 },
]

function copyHash(text: string) {
  copyToClipboard(text)
  message.success(t('common.copied'))
}

function formatSize(bytes: number) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let val = bytes
  while (val >= 1024 && i < units.length - 1) { val /= 1024; i++ }
  return `${val.toFixed(1)} ${units[i]}`
}

async function fetchTorrentStats(p?: number) {
  torrentLoading.value = true
  try {
    const pg = p || torrentPage.value
    const resp = await seedingApi.getStatsTorrents(pg, 20, torrentSort.value, torrentStatus.value)
    torrentStats.value = resp.data.data?.items || []
    torrentTotal.value = resp.data.data?.total || 0
    torrentPage.value = pg
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    torrentLoading.value = false
  }
}

function onTorrentPageChange(p: number) {
  fetchTorrentStats(p)
}

async function fetchAll() {
  try {
    const [overviewResp, siteResp] = await Promise.all([
      seedingApi.getStatsOverview(),
      seedingApi.getStatsBySite(),
    ])
    overview.value = (overviewResp.data.data as SeedingStatsOverview) || overview.value
    siteStats.value = siteResp.data.data?.items ?? []

    const configResp = await seedingApi.getConfig()
    const configData = configResp.data.data
    configs.value = configData?.items ?? []

    await nextTick()
    if (siteStats.value.length > 0 && !trendSite.value) {
      trendSite.value = siteStats.value[0].siteName
    }
    fetchSiteTrend()
    fetchSpeedTrend()
    fetchTorrentStats(1)
    await nextTick()
    renderPies()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

const speedRange = ref('24h')
const siteRange = ref('7d')
const trendSite = ref('')
const speedChartRef = ref<HTMLElement>()
const siteChartRef = ref<HTMLElement>()
const todayPieRef = ref<HTMLElement>()
const historyPieRef = ref<HTMLElement>()
let speedChart: echarts.ECharts | null = null
let siteChart: echarts.ECharts | null = null
let todayPieChart: echarts.ECharts | null = null
let historyPieChart: echarts.ECharts | null = null

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

const pieColors = ['#5470c6','#91cc75','#fac858','#ee6666','#73c0de','#3ba272','#fc8452','#9a60b4','#ea7ccc','#48b8d0']

function renderPies() {
  const data = siteStats.value
    .filter(s => s.todayUploadBytes > 0 || s.historyUploadBytes > 0)
    .map(s => ({ name: s.siteName, today: s.todayUploadBytes, history: s.historyUploadBytes }))
  if (data.length === 0) return

  const todayData = data.filter(d => d.today > 0).map(d => ({ name: d.name, value: d.today }))
  const historyData = data.filter(d => d.history > 0).map(d => ({ name: d.name, value: d.history }))
  const pieOption = (pieData: { name: string; value: number }[]) => ({
    tooltip: { trigger: 'item', formatter: (p: { name: string; value: number; percent: number }) => `${p.name}<br/>${formatSize(p.value)} (${p.percent}%)` },
    legend: { top: 0, textStyle: { fontSize: 11 } },
    color: pieColors,
    series: [{ type: 'pie', radius: ['40%', '70%'], center: ['50%', '55%'], label: { formatter: '{b}: {d}%', fontSize: 11 }, data: pieData }],
  })

  if (!todayPieChart && todayPieRef.value) todayPieChart = echarts.init(todayPieRef.value)
  if (todayPieChart && todayData.length > 0) todayPieChart.setOption(pieOption(todayData))

  if (!historyPieChart && historyPieRef.value) historyPieChart = echarts.init(historyPieRef.value)
  if (historyPieChart && historyData.length > 0) historyPieChart.setOption(pieOption(historyData))
}

function handleResize() {
  speedChart?.resize()
  siteChart?.resize()
  todayPieChart?.resize()
  historyPieChart?.resize()
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
