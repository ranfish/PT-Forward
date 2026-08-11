<template>
  <div style="padding: 24px">
    <!-- Toolbar -->
    <div class="toolbar">
      <div class="toolbar-left">
        <a-select
          v-model:value="selectedClient"
          style="width: 180px"
          placeholder="选择下载器"
          @change="onClientChange"
        >
          <a-select-option v-for="c in clientOptions" :key="c.name" :value="c.name">
            {{ c.name }}
          </a-select-option>
        </a-select>
        <a-select
          v-if="selectedClient && pathOptions.length > 0"
          v-model:value="selectedPath"
          style="width: 220px"
          placeholder="资源路径"
          allow-clear
          @change="onFilterChange"
        >
          <a-select-option v-for="p in pathOptions" :key="p" :value="p">{{ p }}</a-select-option>
        </a-select>
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
          <a-tooltip>
            <template #title>只有配置完整的种子才能审核（进入编辑面板保存后自动审核）</template>
            <a-button size="small" @click="batchReview(true)">
              批量审核 ({{ selectedIds.length }})
            </a-button>
          </a-tooltip>
          <a-button size="small" @click="batchReview(false)">取消审核</a-button>
          <a-popconfirm :title="`确定删除选中的 ${selectedIds.length} 条数据？删除后需重新获取。`" @confirm="batchDelete">
            <a-button size="small" danger>删除数据 ({{ selectedIds.length }})</a-button>
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
          <div class="type-cell">
            <a-tag :color="typeColor(record.type)" size="small">{{ typeLabel(record.type) }}</a-tag>
            <a-tag v-if="record.form" :color="formColor(record.form)" size="small">{{ formLabel(record.form) }}</a-tag>
          </div>
        </template>

        <template v-else-if="column.key === 'tech'">
          <a-tooltip placement="topLeft">
            <template #title>
              <div class="tech-detail">
                <div><span class="td-label">分辨率</span><span :class="record.resolution ? 'td-val' : 'td-miss'">{{ record.resolution || '—' }}</span></div>
                <div><span class="td-label">视频编码</span><span :class="record.videoCodec ? 'td-val' : 'td-miss'">{{ record.videoCodec || '—' }}</span></div>
                <div><span class="td-label">片源类型</span><span :class="record.sourceType ? 'td-val' : 'td-miss'">{{ record.sourceType || '—' }}</span></div>
                <div><span class="td-label">规格</span><span :class="record.specification ? 'td-val' : 'td-miss'">{{ record.specification || '—' }}</span></div>
                <div><span class="td-label">音频编码</span><span :class="record.audioCodec ? 'td-val' : 'td-miss'">{{ record.audioCodec || '—' }}</span></div>
                <div><span class="td-label">声道</span><span :class="record.audioChannels ? 'td-val' : 'td-miss'">{{ record.audioChannels || '—' }}</span></div>
                <div><span class="td-label">音频技术</span><span :class="record.audioTechnology ? 'td-val' : 'td-miss'">{{ record.audioTechnology || '—' }}</span></div>
                <div><span class="td-label">HDR</span><span :class="record.hdr ? 'td-val' : 'td-miss'">{{ record.hdr || '—' }}</span></div>
                <div><span class="td-label">bit</span><span :class="record.bitDepth ? 'td-val' : 'td-miss'">{{ record.bitDepth || '—' }}</span></div>
                <div><span class="td-label">平台</span><span :class="record.sourcePlatform ? 'td-val' : 'td-miss'">{{ record.sourcePlatform || '—' }}</span></div>
                <div><span class="td-label">版本</span><span :class="record.editionInfo ? 'td-val' : 'td-miss'">{{ record.editionInfo || '—' }}</span></div>
                <div><span class="td-label">地区码</span><span :class="record.regionCode ? 'td-val' : 'td-miss'">{{ record.regionCode || '—' }}</span></div>
              </div>
            </template>
            <div class="tech-cell">
              <span v-if="record.resolution" class="tech-badge tech-res">{{ record.resolution }}</span>
              <span v-if="record.videoCodec" class="tech-badge tech-codec">{{ record.videoCodec }}</span>
              <span v-if="audioSummary(record)" class="tech-badge tech-audio">{{ audioSummary(record) }}</span>
              <span v-if="record.hdr" class="tech-badge tech-hdr">{{ record.hdr }}</span>
              <span v-if="!record.resolution && !record.videoCodec && !record.audioCodec && !record.hdr" class="text-missing">未配置</span>
            </div>
          </a-tooltip>
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
          <a-tag v-if="record.forbidden" color="red" size="small">禁转</a-tag>
          <a-tag v-else-if="record.reviewed" color="success" size="small">
            <CheckCircleFilled /> 已审核
          </a-tag>
          <a-tooltip v-else>
            <template #title>
              <div style="font-size: 12px">
                <div v-for="m in reviewBlockers(record)" :key="m" style="color: #ff4d4f">✗ {{ m }}</div>
                <div v-if="reviewBlockers(record).length === 0" style="color: #faad14">必需字段已齐全，进入编辑面板保存后自动审核</div>
              </div>
            </template>
            <a-tag :color="reviewBlockers(record).length > 0 ? 'default' : 'warning'" size="small">
              {{ reviewBlockers(record).length > 0 ? '配置不完整' : '待审核' }}
            </a-tag>
          </a-tooltip>
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

    <!-- Edit Panel -->
    <CrossSeedPanel
      v-model:open="editOpen"
      :preset-torrent="editPreset"
      maintenance-only
      @success="onEditSuccess"
    />

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
import CrossSeedPanel from './CrossSeedPanel.vue'

interface SeedItem {
  id: number
  infoHash: string
  title: string
  subtitle: string
  siteName: string
  torrentId: string
  size: number
  type: string
  form: string
  clientName: string
  savePath: string
  resolution: string
  videoCodec: string
  audioCodec: string
  audioChannels: string
  audioTechnology: string
  hdr: string
  bitDepth: string
  sourceType: string
  specification: string
  sourcePlatform: string
  editionInfo: string
  regionCode: string
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
const editPreset = ref<any>(null)
const showGetModal = ref(false)

interface ClientOption {
  name: string
  paths: string[]
}

const clientOptions = ref<ClientOption[]>([
  { name: 'tr辅种', paths: ['/PT4/SSD4', '/PT7/SSD7', '/PT2/TTG2'] },
  { name: 'qb下载', paths: ['/PT0/temp'] },
  { name: 'PT5', paths: ['/PT5/ Movies', '/PT5/TV'] },
])
const selectedClient = ref<string | undefined>(undefined)
const selectedPath = ref<string | undefined>(undefined)
const pathOptions = computed(() => {
  const c = clientOptions.value.find(c => c.name === selectedClient.value)
  return c ? c.paths : []
})

function onClientChange() {
  selectedPath.value = undefined
  onFilterChange()
}

const siteOptions = ref<string[]>([])

const columns = [
  { title: '标题', key: 'title', ellipsis: true, width: 260 },
  { title: '源站', key: 'site_name', width: 90 },
  { title: '大小', key: 'size', width: 80 },
  { title: '类型', key: 'type', width: 110 },
  { title: '技术参数', key: 'tech', width: 200 },
  { title: '制作组', key: 'team', width: 90 },
  { title: '完整度', key: 'completeness', width: 70, align: 'center' as const },
  { title: '审核', key: 'reviewed', width: 60, align: 'center' as const },
  { title: '标记', key: 'flags', width: 100 },
  { title: '操作', key: 'action', width: 110, fixed: 'right' as const },
]

const mockData = ref<SeedItem[]>([
  { id: 1, infoHash: 'abc123', title: 'The.Matrix.1999.UHD.BluRay.2160p.Atmos.TrueHD', subtitle: '黑客帝国', siteName: '朋友', torrentId: '27801', size: 68100000000, type: 'movie', form: '', clientName: 'tr辅种', savePath: '/PT4/SSD4', resolution: '2160p', videoCodec: 'x265', audioCodec: 'TrueHD', audioChannels: '7.1', audioTechnology: 'Atmos', hdr: 'DV HDR', bitDepth: '10bit', sourceType: 'UHD Blu-ray', specification: '', sourcePlatform: '', editionInfo: '', regionCode: 'USA', team: 'FRDS', poster: 'url', screenshots: 'url', body: 'text', mediainfo: 'text', doubanId: '1294639', imdbId: 'tt0133093', reviewed: true, forbidden: false, updatedAt: '2026-08-10T10:00:00Z' },
  { id: 2, infoHash: 'def456', title: 'Dune.Part.Two.2024.1080p.WEB-DL.DDP5.1.Atmos', subtitle: '沙丘2', siteName: '彩虹岛', torrentId: '56901', size: 15200000000, type: 'movie', form: '', clientName: 'tr辅种', savePath: '/PT4/SSD4', resolution: '1080p', videoCodec: 'x265', audioCodec: 'DDP', audioChannels: '5.1', audioTechnology: 'Atmos', hdr: '', bitDepth: '', sourceType: 'BluRay', specification: 'WEB-DL', sourcePlatform: 'AMZN', editionInfo: '', regionCode: '', team: 'CHDWEB', poster: 'url', screenshots: 'url', body: 'text', mediainfo: '', doubanId: '', imdbId: 'tt15239678', reviewed: false, forbidden: false, updatedAt: '2026-08-09T15:30:00Z' },
  { id: 3, infoHash: 'ghi789', title: 'Beast.Race.2026.UHD.BluRay.2160p.DV.HDR10plus', subtitle: '穿普拉达的女王2', siteName: '朋友', torrentId: '27823', size: 21960000000, type: 'movie', form: '', clientName: 'tr辅种', savePath: '/PT4/SSD4', resolution: '2160p', videoCodec: 'x265', audioCodec: 'TrueHD', audioChannels: '7.1', audioTechnology: 'Atmos', hdr: 'DV HDR10+', bitDepth: '10bit', sourceType: 'UHD Blu-ray', specification: '', sourcePlatform: '', editionInfo: '', regionCode: 'USA', team: 'FRDS', poster: 'url', screenshots: '', body: 'text', mediainfo: 'text', doubanId: '', imdbId: '', reviewed: false, forbidden: false, updatedAt: '2026-08-09T12:00:00Z' },
  { id: 4, infoHash: 'jkl012', title: 'Brothers.in.Arms.S01.1080p.WEB-DL', subtitle: '战火兄弟', siteName: '彩虹岛', torrentId: '47016', size: 5149000000, type: 'tv_series', form: 'partial_pack', clientName: 'tr辅种', savePath: '/PT7/SSD7', resolution: '1080p', videoCodec: 'x264', audioCodec: 'AAC', audioChannels: '2.0', audioTechnology: '', hdr: '', bitDepth: '', sourceType: 'BluRay', specification: 'WEB-DL', sourcePlatform: 'HamiVideo', editionInfo: '', regionCode: '', team: 'CHDWEB', poster: '', screenshots: '', body: '', mediainfo: '', doubanId: '', imdbId: '', reviewed: false, forbidden: false, updatedAt: '2026-08-08T18:00:00Z' },
  { id: 5, infoHash: 'mno345', title: 'Otto.wo.Koroshita.S01E02.1080p.WEB-DL', subtitle: '明明杀了丈夫', siteName: '彩虹岛', torrentId: '48087', size: 5699000000, type: 'tv_series', form: 'single_episode', clientName: 'tr辅种', savePath: '/PT7/SSD7', resolution: '1080p', videoCodec: 'x264', audioCodec: 'AAC', audioChannels: '2.0', audioTechnology: '', hdr: '', bitDepth: '', sourceType: 'BluRay', specification: 'WEB-DL', sourcePlatform: 'friDay', editionInfo: '', regionCode: 'JPN', team: 'CHDWEB', poster: 'url', screenshots: 'url', body: 'text', mediainfo: 'text', doubanId: '', imdbId: '', reviewed: true, forbidden: false, updatedAt: '2026-08-09T20:00:00Z' },
  { id: 6, infoHash: 'pqr678', title: 'The.Big.Lebowski.1998.UHD.BluRay.2160p', subtitle: '谋杀绿脚趾', siteName: '不可说', torrentId: '62001', size: 67000000000, type: 'movie', form: '', clientName: 'tr辅种', savePath: '/PT2/TTG2', resolution: '2160p', videoCodec: 'x265', audioCodec: 'TrueHD', audioChannels: '7.1', audioTechnology: 'Atmos', hdr: 'HDR', bitDepth: '10bit', sourceType: 'UHD Blu-ray', specification: '', sourcePlatform: '', editionInfo: '', regionCode: 'USA', team: 'CMCTV', poster: 'url', screenshots: 'url', body: 'text', mediainfo: 'text', doubanId: '1292635', imdbId: 'tt0118715', reviewed: true, forbidden: false, updatedAt: '2026-08-10T08:00:00Z' },
  { id: 7, infoHash: 'stu901', title: 'Criminal.Minds.S19.2160p.WEB-DL.HDR', subtitle: '犯罪心理 第十九季', siteName: '彩虹岛', torrentId: '47020', size: 42800000000, type: 'tv_series', form: 'season_pack', clientName: 'tr辅种', savePath: '/PT7/SSD7', resolution: '2160p', videoCodec: 'H265', audioCodec: 'DDP', audioChannels: '5.1', audioTechnology: '', hdr: 'HDR', bitDepth: '10bit', sourceType: 'BluRay', specification: 'WEB-DL', sourcePlatform: 'Disney+', editionInfo: '', regionCode: '', team: 'CHDWEB', poster: 'url', screenshots: '', body: 'text', mediainfo: '', doubanId: '', imdbId: '', reviewed: false, forbidden: true, updatedAt: '2026-08-07T22:00:00Z' },
  { id: 8, infoHash: 'vwx234', title: 'Akira.1988.UHD.BluRay.2160p.DV.HDR', subtitle: '阿基拉', siteName: '海豹', torrentId: '18306', size: 85000000000, type: 'animation', form: '', clientName: 'tr辅种', savePath: '/PT4/SSD4', resolution: '2160p', videoCodec: 'x265', audioCodec: 'TrueHD', audioChannels: '5.1', audioTechnology: '', hdr: 'DV HDR', bitDepth: '10bit', sourceType: 'UHD Blu-ray', specification: '', sourcePlatform: '', editionInfo: '', regionCode: 'JPN', team: 'CMCT', poster: 'url', screenshots: 'url', body: 'text', mediainfo: 'text', doubanId: '', imdbId: 'tt0094625', reviewed: true, forbidden: false, updatedAt: '2026-08-06T14:00:00Z' },
  { id: 9, infoHash: 'yza567', title: 'Beethoven.Symphony.No.9.Blu-ray.1080p', subtitle: '贝多芬第九交响曲', siteName: '皇后', torrentId: '10062', size: 32000000000, type: 'music', form: '', clientName: 'tr辅种', savePath: '/PT2/TTG2', resolution: '1080p', videoCodec: 'AVC', audioCodec: 'LPCM', audioChannels: '5.1', audioTechnology: '', hdr: '', bitDepth: '', sourceType: 'Blu-ray', specification: '', sourcePlatform: '', editionInfo: '', regionCode: 'DEU', team: 'CMCT', poster: 'url', screenshots: '', body: 'text', mediainfo: 'text', doubanId: '', imdbId: '', reviewed: false, forbidden: false, updatedAt: '2026-08-05T10:00:00Z' },
  { id: 10, infoHash: 'bcd890', title: 'Oppenheimer.2023.UHD.BluRay.2160p.Atmos', subtitle: '奥本海默', siteName: '朋友', torrentId: '27001', size: 95000000000, type: 'movie', form: '', clientName: 'tr辅种', savePath: '/PT4/SSD4', resolution: '2160p', videoCodec: 'x265', audioCodec: 'TrueHD', audioChannels: '7.1', audioTechnology: 'Atmos', hdr: 'DV HDR', bitDepth: '10bit', sourceType: 'UHD Blu-ray', specification: '', sourcePlatform: '', editionInfo: '', regionCode: 'USA', team: 'FRDS', poster: 'url', screenshots: 'url', body: 'text', mediainfo: 'text', doubanId: '35593344', imdbId: 'tt15398776', reviewed: true, forbidden: false, updatedAt: '2026-08-10T09:30:00Z' },
  { id: 11, infoHash: 'efg123', title: '', subtitle: '', siteName: '猫', torrentId: '87180', size: 14000000000, type: 'unknown', form: 'unknown', clientName: 'qb下载', savePath: '/PT0/temp', resolution: '', videoCodec: '', audioCodec: '', audioChannels: '', audioTechnology: '', hdr: '', bitDepth: '', sourceType: '', specification: '', sourcePlatform: '', editionInfo: '', regionCode: '', team: 'PTerWEB', poster: '', screenshots: '', body: '', mediainfo: '', doubanId: '', imdbId: '', reviewed: false, forbidden: false, updatedAt: '2026-08-09T16:00:00Z' },
  { id: 12, infoHash: 'hij456', title: 'The.One.2001.BluRay.720p.x264-WiKi', subtitle: '救世主', siteName: '套套哥', torrentId: '61118', size: 4748000000, type: 'movie', form: '', clientName: 'qb下载', savePath: '/PT0/temp', resolution: '720p', videoCodec: 'x264', audioCodec: 'DTS', audioChannels: '5.1', audioTechnology: '', hdr: '', bitDepth: '', sourceType: 'BluRay', specification: '', sourcePlatform: '', editionInfo: '', regionCode: '', team: 'WiKi', poster: '', screenshots: '', body: '', mediainfo: '', doubanId: '', imdbId: 'tt0259740', reviewed: false, forbidden: false, updatedAt: '2026-08-08T11:00:00Z' },
])

const filteredData = computed(() => {
  let result = mockData.value
  if (selectedClient.value) {
    result = result.filter(item => item.clientName === selectedClient.value)
  }
  if (selectedPath.value) {
    result = result.filter(item => item.savePath === selectedPath.value)
  }
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
    unknown: 'default',
  }
  return map[type] || 'default'
}

function formLabel(form: string): string {
  const map: Record<string, string> = {
    season_pack: '全集',
    partial_pack: '部分合集',
    single_episode: '单集',
    unknown: '未知',
  }
  return map[form] || ''
}

function formColor(form: string): string {
  const map: Record<string, string> = {
    season_pack: 'geekblue',
    partial_pack: 'gold',
    single_episode: 'lime',
    unknown: 'default',
  }
  return map[form] || 'default'
}

function audioSummary(item: SeedItem): string {
  const parts: string[] = []
  if (item.audioCodec) parts.push(item.audioCodec)
  if (item.audioChannels) parts.push(item.audioChannels)
  if (item.audioTechnology) parts.push(item.audioTechnology)
  return parts.join(' ')
}

function reviewBlockers(item: SeedItem): string[] {
  const blockers: string[] = []
  if (!item.title) blockers.push('标题')
  if (!item.poster) blockers.push('海报')
  if (!item.screenshots) blockers.push('截图')
  if (!item.body) blockers.push('简介')
  if (!item.mediainfo) blockers.push('MediaInfo')
  if (!item.resolution) blockers.push('分辨率')
  if (!item.videoCodec) blockers.push('视频编码')
  if (!item.audioCodec) blockers.push('音频编码')
  if (!item.team) blockers.push('制作组')
  return blockers
}

function isReviewable(item: SeedItem): boolean {
  return !item.forbidden && reviewBlockers(item).length === 0
}

function completenessPercent(item: SeedItem): number {
  const fields = ['title', 'subtitle', 'poster', 'screenshots', 'body', 'mediainfo', 'resolution', 'videoCodec', 'audioCodec']
  const filled = fields.filter(f => item[f as keyof SeedItem]).length
  return Math.round((filled / fields.length) * 100)
}

function completenessColor(item: SeedItem): string {
  const pct = completenessPercent(item)
  if (pct >= 85) return '#52c41a'
  if (pct >= 50) return '#faad14'
  return '#ff4d4f'
}

function batchReview(val: boolean) {
  let count = 0
  let skipped = 0
  mockData.value.forEach(item => {
    if (selectedIds.value.includes(item.id)) {
      if (val && !isReviewable(item)) {
        skipped++
        return
      }
      item.reviewed = val
      count++
    }
  })
  if (skipped > 0) {
    message.warning(`${count} 条已${val ? '审核' : '取消审核'}，${skipped} 条配置不完整被跳过`)
  } else {
    message.success(`${count} 条记录已${val ? '审核' : '取消审核'}`)
  }
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

function openEdit(item: SeedItem) {
  editPreset.value = {
    info_hash: item.infoHash,
    hash: item.infoHash,
    name: item.title,
    size: item.size,
    save_path: item.savePath,
    source_site: item.siteName,
  }
  editOpen.value = true
}

function onEditSuccess() {
  // TODO: 后续接入后端时刷新列表
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
.type-cell {
  display: flex;
  align-items: center;
  gap: 3px;
  flex-wrap: wrap;
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
.tech-hdr { background: #fff0f6; color: #eb2f96; }
.text-missing { color: #ccc; font-style: italic; }
.tech-detail {
  font-size: 12px;
  line-height: 1.6;
}
.tech-detail .td-label {
  display: inline-block;
  width: 56px;
  color: #888;
}
.tech-detail .td-val {
  color: #333;
}
.tech-detail .td-miss {
  color: #ccc;
}
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
