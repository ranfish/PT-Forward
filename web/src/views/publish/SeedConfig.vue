<template>
  <div style="padding: 24px">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <a-input-search
          v-model:value="searchText"
          placeholder="搜索标题或副标题..."
          style="width: 280px"
          allow-clear
          @change="onFilterChange"
        />
        <a-select
          v-model:value="siteFilter"
          style="width: 140px"
          placeholder="源站"
          allow-clear
          @change="onFilterChange"
        >
          <a-select-option v-for="s in siteOptions" :key="s" :value="s">{{ s }}</a-select-option>
        </a-select>
        <a-select
          v-model:value="typeFilter"
          style="width: 120px"
          placeholder="类型"
          allow-clear
          @change="onFilterChange"
        >
          <a-select-option value="movie">电影</a-select-option>
          <a-select-option value="tv_series">剧集</a-select-option>
          <a-select-option value="animation">动漫</a-select-option>
          <a-select-option value="documentary">纪录</a-select-option>
          <a-select-option value="music">音乐</a-select-option>
        </a-select>
        <a-radio-group v-model:value="reviewFilter" button-style="solid" size="small" @change="onFilterChange">
          <a-radio-button value="all">全部</a-radio-button>
          <a-radio-button value="reviewed">已审核</a-radio-button>
          <a-radio-button value="unreviewed">待审核</a-radio-button>
        </a-radio-group>
        <a-tag color="blue">{{ filteredData.length }} / {{ mockData.length }}</a-tag>
      </div>
      <div class="toolbar-right">
        <template v-if="selectedIds.length > 0">
          <a-button size="small" type="primary" @click="batchReview(true)">
            批量审核 ({{ selectedIds.length }})
          </a-button>
          <a-button size="small" @click="batchReview(false)">取消审核</a-button>
          <a-popconfirm :title="`确定删除选中的 ${selectedIds.length} 条？`" @confirm="batchDelete">
            <a-button size="small" danger>批量删除</a-button>
          </a-popconfirm>
        </template>
        <a-button type="primary" @click="showGetModal = true">
          <PlusOutlined /> 获取数据
        </a-button>
      </div>
    </div>

    <!-- Table -->
    <a-table
      :columns="columns"
      :data-source="filteredData"
      :loading="loading"
      :pagination="{
        current: currentPage,
        pageSize: pageSize,
        total: filteredData.length,
        showSizeChanger: true,
        pageSizeOptions: ['20', '50', '100'],
        showTotal: (total: number) => `共 ${total} 条`,
        size: 'small',
      }"
      row-key="id"
      size="small"
      :scroll="{ x: 1400 }"
      :sticky="{ offsetHeader: 48 }"
      :row-class-name="(record: SeedItem) => record.forbidden ? 'row-forbidden' : (record.reviewed ? '' : 'row-unreviewed')"
      :row-selection="{ selectedRowKeys: selectedIds, onChange: (keys: number[]) => selectedIds = keys }"
      @change="onTableChange"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'title'">
          <div class="title-cell">
            <div class="title-main" :class="{ 'text-missing': !record.title }">{{ record.title || '(未设置)' }}</div>
            <div v-if="record.subtitle" class="title-sub">{{ record.subtitle }}</div>
          </div>
        </template>

        <template v-else-if="column.key === 'site_name'">
          <div class="site-cell">
            <a-tag color="blue" size="small">{{ record.siteName }}</a-tag>
            <CheckCircleFilled v-if="record.reviewed" style="color: #52c41a; font-size: 12px" />
          </div>
        </template>

        <template v-else-if="column.key === 'size'">
          {{ formatSize(record.size) }}
        </template>

        <template v-else-if="column.key === 'type'">
          <a-tag :color="typeColor(record.type)" size="small">{{ typeLabel(record.type) }}</a-tag>
        </template>

        <template v-else-if="column.key === 'tech'">
          <div class="tech-cell">
            <span v-if="record.resolution" class="tech-badge tech-res">{{ record.resolution }}</span>
            <span v-if="record.videoCodec" class="tech-badge tech-codec">{{ record.videoCodec }}</span>
            <span v-if="record.audioCodec" class="tech-badge tech-audio">{{ record.audioCodec }}</span>
            <span v-if="!record.resolution && !record.videoCodec && !record.audioCodec" class="text-missing">未配置</span>
          </div>
        </template>

        <template v-else-if="column.key === 'team'">
          <a-tag v-if="record.team" size="small">{{ record.team }}</a-tag>
          <span v-else class="text-missing">-</span>
        </template>

        <template v-else-if="column.key === 'completeness'">
          <a-tooltip>
            <template #title>
              <div class="completeness-detail">
                <span :class="record.title ? 'ck' : 'ck-miss'">标题</span>
                <span :class="record.subtitle ? 'ck' : 'ck-miss'">副标题</span>
                <span :class="record.poster ? 'ck' : 'ck-miss'">海报</span>
                <span :class="record.screenshots ? 'ck' : 'ck-miss'">截图</span>
                <span :class="record.body ? 'ck' : 'ck-miss'">简介</span>
                <span :class="record.mediainfo ? 'ck' : 'ck-miss'">媒体信息</span>
                <span :class="record.resolution ? 'ck' : 'ck-miss'">技术参数</span>
              </div>
            </template>
            <a-progress
              type="circle"
              :size="36"
              :percent="completenessPercent(record)"
              :stroke-color="completenessColor(record)"
            />
          </a-tooltip>
        </template>

        <template v-else-if="column.key === 'reviewed'">
          <a-switch
            :checked="record.reviewed"
            size="small"
            @change="(val: boolean) => toggleReview(record, val)"
          />
        </template>

        <template v-else-if="column.key === 'flags'">
          <a-tag v-if="record.forbidden" color="red" size="small">禁转</a-tag>
          <a-tag v-if="record.doubanId" color="purple" size="small">豆瓣</a-tag>
          <a-tag v-if="record.imdbId" color="purple" size="small">IMDb</a-tag>
        </template>

        <template v-else-if="column.key === 'action'">
          <a-space size="small">
            <a-button size="small" type="link" @click="openEdit(record)">编辑</a-button>
            <a-popconfirm title="确定删除？" @confirm="deleteItem(record)">
              <a-button size="small" type="link" danger>删除</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <!-- Edit Modal (placeholder) -->
    <a-modal
      v-model:open="editOpen"
      title="编辑种子配置"
      width="90%"
      :footer="null"
      destroy-on-close
    >
      <a-result
        status="info"
        title="编辑功能开发中"
        sub-title="后续接入 CrossSeedPanel 编辑器"
      />
    </a-modal>

    <!-- Get Data Modal (placeholder) -->
    <a-modal
      v-model:open="showGetModal"
      title="获取种子数据"
      :footer="null"
      destroy-on-close
    >
      <a-result
        status="info"
        title="获取数据功能开发中"
        sub-title="后续接入批量获取 + 源站抓取"
      />
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { PlusOutlined, CheckCircleFilled } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'

interface SeedItem {
  id: number
  infoHash: string
  title: string
  subtitle: string
  siteName: string
  torrentId: string
  size: number
  type: string
  resolution: string
  videoCodec: string
  audioCodec: string
  medium: string
  team: string
  poster: string
  screenshots: string
  body: string
  mediainfo: string
  doubanId: string
  imdbId: string
  reviewed: boolean
  forbidden: boolean
  updatedAt: string
}

const loading = ref(false)
const searchText = ref('')
const siteFilter = ref<string | undefined>(undefined)
const typeFilter = ref<string | undefined>(undefined)
const reviewFilter = ref<'all' | 'reviewed' | 'unreviewed'>('all')
const selectedIds = ref<number[]>([])
const currentPage = ref(1)
const pageSize = ref(20)
const editOpen = ref(false)
const showGetModal = ref(false)

const siteOptions = ref<string[]>([])

const columns = [
  { title: '标题', key: 'title', ellipsis: true, width: 260 },
  { title: '源站', key: 'site_name', width: 90 },
  { title: '大小', key: 'size', width: 80 },
  { title: '类型', key: 'type', width: 70 },
  { title: '技术参数', key: 'tech', width: 160 },
  { title: '制作组', key: 'team', width: 90 },
  { title: '完整度', key: 'completeness', width: 70, align: 'center' as const },
  { title: '审核', key: 'reviewed', width: 60, align: 'center' as const },
  { title: '标记', key: 'flags', width: 100 },
  { title: '操作', key: 'action', width: 110, fixed: 'right' as const },
]

const mockData = ref<SeedItem[]>([
  { id: 1, infoHash: 'abc123', title: 'The.Matrix.1999.UHD.BluRay.2160p.Atmos.TrueHD', subtitle: '黑客帝国', siteName: '朋友', torrentId: '27801', size: 68100000000, type: 'movie', resolution: '2160p', videoCodec: 'x265', audioCodec: 'TrueHD Atmos', medium: 'UHD BluRay', team: 'FRDS', poster: 'url', screenshots: 'url', body: 'text', mediainfo: 'text', doubanId: '1294639', imdbId: 'tt0133093', reviewed: true, forbidden: false, updatedAt: '2026-08-10T10:00:00Z' },
  { id: 2, infoHash: 'def456', title: 'Dune.Part.Two.2024.1080p.WEB-DL.DDP5.1.Atmos', subtitle: '沙丘2', siteName: '彩虹岛', torrentId: '56901', size: 15200000000, type: 'movie', resolution: '1080p', videoCodec: 'x265', audioCodec: 'DDP5.1', medium: 'WEB-DL', team: 'CHDWEB', poster: 'url', screenshots: 'url', body: 'text', mediainfo: '', doubanId: '', imdbId: 'tt15239678', reviewed: false, forbidden: false, updatedAt: '2026-08-09T15:30:00Z' },
  { id: 3, infoHash: 'ghi789', title: 'Beast.Race.2026.UHD.BluRay.2160p.DV.HDR10plus', subtitle: '穿普拉达的女王2', siteName: '朋友', torrentId: '27823', size: 21960000000, type: 'movie', resolution: '2160p', videoCodec: 'x265', audioCodec: 'TrueHD7.1', medium: 'UHD BluRay', team: 'FRDS', poster: 'url', screenshots: '', body: 'text', mediainfo: 'text', doubanId: '', imdbId: '', reviewed: false, forbidden: false, updatedAt: '2026-08-09T12:00:00Z' },
  { id: 4, infoHash: 'jkl012', title: 'Brothers.in.Arms.S01.1080p.WEB-DL', subtitle: '战火兄弟', siteName: '彩虹岛', torrentId: '47016', size: 5149000000, type: 'tv_series', resolution: '1080p', videoCodec: 'x264', audioCodec: 'AAC2.0', medium: 'WEB-DL', team: 'CHDWEB', poster: '', screenshots: '', body: '', mediainfo: '', doubanId: '', imdbId: '', reviewed: false, forbidden: false, updatedAt: '2026-08-08T18:00:00Z' },
  { id: 5, infoHash: 'mno345', title: 'Otto.wo.Koroshita.S01.1080p.WEB-DL', subtitle: '明明杀了丈夫', siteName: '彩虹岛', torrentId: '48087', size: 5699000000, type: 'tv_series', resolution: '1080p', videoCodec: 'x264', audioCodec: 'AAC2.0', medium: 'WEB-DL', team: 'CHDWEB', poster: 'url', screenshots: 'url', body: 'text', mediainfo: 'text', doubanId: '', imdbId: '', reviewed: true, forbidden: false, updatedAt: '2026-08-09T20:00:00Z' },
  { id: 6, infoHash: 'pqr678', title: 'The.Big.Lebowski.1998.UHD.BluRay.2160p', subtitle: '谋杀绿脚趾', siteName: '不可说', torrentId: '62001', size: 67000000000, type: 'movie', resolution: '2160p', videoCodec: 'x265', audioCodec: 'TrueHD7.1', medium: 'UHD BluRay', team: 'CMCTV', poster: 'url', screenshots: 'url', body: 'text', mediainfo: 'text', doubanId: '1292635', imdbId: 'tt0118715', reviewed: true, forbidden: false, updatedAt: '2026-08-10T08:00:00Z' },
  { id: 7, infoHash: 'stu901', title: 'Criminal.Minds.S19.2160p.WEB-DL.HDR', subtitle: '犯罪心理 第十九季', siteName: '彩虹岛', torrentId: '47020', size: 42800000000, type: 'tv_series', resolution: '2160p', videoCodec: 'H265', audioCodec: 'DDP5.1', medium: 'WEB-DL', team: 'CHDWEB', poster: 'url', screenshots: '', body: 'text', mediainfo: '', doubanId: '', imdbId: '', reviewed: false, forbidden: true, updatedAt: '2026-08-07T22:00:00Z' },
  { id: 8, infoHash: 'vwx234', title: 'Akira.1988.UHD.BluRay.2160p.DV.HDR', subtitle: '阿基拉', siteName: '海豹', torrentId: '18306', size: 85000000000, type: 'animation', resolution: '2160p', videoCodec: 'x265', audioCodec: 'TrueHD', medium: 'UHD BluRay', team: 'CMCT', poster: 'url', screenshots: 'url', body: 'text', mediainfo: 'text', doubanId: '', imdbId: 'tt0094625', reviewed: true, forbidden: false, updatedAt: '2026-08-06T14:00:00Z' },
  { id: 9, infoHash: 'yza567', title: 'Beethoven.Symphony.No.9.Blu-ray.1080p', subtitle: '贝多芬第九交响曲', siteName: '皇后', torrentId: '10062', size: 32000000000, type: 'music', resolution: '1080p', videoCodec: 'AVC', audioCodec: 'LPCM', medium: 'Blu-ray', team: 'CMCT', poster: 'url', screenshots: '', body: 'text', mediainfo: 'text', doubanId: '', imdbId: '', reviewed: false, forbidden: false, updatedAt: '2026-08-05T10:00:00Z' },
  { id: 10, infoHash: 'bcd890', title: 'Oppenheimer.2023.UHD.BluRay.2160p.Atmos', subtitle: '奥本海默', siteName: '朋友', torrentId: '27001', size: 95000000000, type: 'movie', resolution: '2160p', videoCodec: 'x265', audioCodec: 'TrueHD Atmos', medium: 'UHD BluRay', team: 'FRDS', poster: 'url', screenshots: 'url', body: 'text', mediainfo: 'text', doubanId: '35593344', imdbId: 'tt15398776', reviewed: true, forbidden: false, updatedAt: '2026-08-10T09:30:00Z' },
  { id: 11, infoHash: 'efg123', title: '', subtitle: '', siteName: '猫', torrentId: '87180', size: 14000000000, type: 'movie', resolution: '', videoCodec: '', audioCodec: '', medium: '', team: 'PTerWEB', poster: '', screenshots: '', body: '', mediainfo: '', doubanId: '', imdbId: '', reviewed: false, forbidden: false, updatedAt: '2026-08-09T16:00:00Z' },
  { id: 12, infoHash: 'hij456', title: 'The.One.2001.BluRay.720p.x264-WiKi', subtitle: '救世主', siteName: '套套哥', torrentId: '61118', size: 4748000000, type: 'movie', resolution: '720p', videoCodec: 'x264', audioCodec: 'DTS', medium: 'BluRay', team: 'WiKi', poster: '', screenshots: '', body: '', mediainfo: '', doubanId: '', imdbId: 'tt0259740', reviewed: false, forbidden: false, updatedAt: '2026-08-08T11:00:00Z' },
])

const filteredData = computed(() => {
  let result = mockData.value
  if (searchText.value) {
    const q = searchText.value.toLowerCase()
    result = result.filter(item =>
      item.title.toLowerCase().includes(q) ||
      item.subtitle.toLowerCase().includes(q)
    )
  }
  if (siteFilter.value) {
    result = result.filter(item => item.siteName === siteFilter.value)
  }
  if (typeFilter.value) {
    result = result.filter(item => item.type === typeFilter.value)
  }
  if (reviewFilter.value === 'reviewed') {
    result = result.filter(item => item.reviewed)
  } else if (reviewFilter.value === 'unreviewed') {
    result = result.filter(item => !item.reviewed)
  }
  return result
})

function onFilterChange() {
  currentPage.value = 1
}

function onTableChange(pag: { current?: number; pageSize?: number }) {
  if (pag.current) currentPage.value = pag.current
  if (pag.pageSize) pageSize.value = pag.pageSize
}

function formatSize(bytes: number): string {
  if (bytes >= 1073741824) return (bytes / 1073741824).toFixed(1) + ' GiB'
  if (bytes >= 1048576) return (bytes / 1048576).toFixed(0) + ' MiB'
  return bytes + ' B'
}

function typeLabel(type: string): string {
  const map: Record<string, string> = {
    movie: '电影', tv_series: '剧集', animation: '动漫',
    documentary: '纪录', music: '音乐', tv_shows: '综艺',
  }
  return map[type] || type
}

function typeColor(type: string): string {
  const map: Record<string, string> = {
    movie: 'blue', tv_series: 'green', animation: 'purple',
    documentary: 'cyan', music: 'orange', tv_shows: 'magenta',
  }
  return map[type] || 'default'
}

function completenessPercent(item: SeedItem): number {
  const fields = ['title', 'subtitle', 'poster', 'screenshots', 'body', 'mediainfo', 'resolution']
  const filled = fields.filter(f => item[f as keyof SeedItem]).length
  return Math.round((filled / fields.length) * 100)
}

function completenessColor(item: SeedItem): string {
  const pct = completenessPercent(item)
  if (pct >= 85) return '#52c41a'
  if (pct >= 50) return '#faad14'
  return '#ff4d4f'
}

function toggleReview(item: SeedItem, val: boolean) {
  item.reviewed = val
  message.success(val ? `${item.title || '未命名'} 已审核` : '已取消审核')
}

function batchReview(val: boolean) {
  let count = 0
  mockData.value.forEach(item => {
    if (selectedIds.value.includes(item.id)) {
      item.reviewed = val
      count++
    }
  })
  message.success(`${count} 条记录已${val ? '审核' : '取消审核'}`)
  selectedIds.value = []
}

function batchDelete() {
  mockData.value = mockData.value.filter(item => !selectedIds.value.includes(item.id))
  message.success(`已删除 ${selectedIds.value.length} 条记录`)
  selectedIds.value = []
}

function deleteItem(item: SeedItem) {
  mockData.value = mockData.value.filter(i => i.id !== item.id)
  message.success('已删除')
}

function openEdit(_item: SeedItem) {
  editOpen.value = true
}

onMounted(() => {
  siteOptions.value = [...new Set(mockData.value.map(i => i.siteName))].sort()
})
</script>

<style scoped>
.toolbar {
  margin-bottom: 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}
.toolbar-left {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.toolbar-right {
  display: flex;
  align-items: center;
  gap: 8px;
}
.title-cell .title-main {
  font-weight: 500;
}
.title-cell .title-sub {
  font-size: 12px;
  color: #999;
}
.site-cell {
  display: flex;
  align-items: center;
  gap: 4px;
}
.tech-cell {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}
.tech-badge {
  font-size: 10px;
  padding: 1px 5px;
  border-radius: 3px;
  white-space: nowrap;
}
.tech-res { background: #e6f7ff; color: #1890ff; }
.tech-codec { background: #f6ffed; color: #52c41a; }
.tech-audio { background: #fff7e6; color: #faad14; }
.text-missing { color: #ccc; font-style: italic; }
.completeness-detail {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 12px;
}
.completeness-detail .ck { color: #52c41a; }
.completeness-detail .ck::before { content: '✓ '; }
.completeness-detail .ck-miss { color: #ff4d4f; }
.completeness-detail .ck-miss::before { content: '✗ '; }
:deep(.row-forbidden) {
  background: #fff2f0;
}
:deep(.row-unreviewed) {
  opacity: 0.7;
}
</style>
