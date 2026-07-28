<template>
  <div>
    <div class="page-toolbar">
      <a-select
        v-model:value="selectedClientId"
        style="width: 260px"
        :loading="clientsLoading"
        placeholder="选择下载器"
        @change="onClientChange"
      >
        <a-select-option v-for="c in clients" :key="c.id" :value="c.id">
          {{ c.name }} ({{ c.type }})
        </a-select-option>
      </a-select>
      <a-input-search
        v-if="torrents.length"
        v-model:value="searchText"
        placeholder="搜索种子名称..."
        style="width: 260px; margin-left: 12px"
        allow-clear
      />
      <a-select
        v-if="torrents.length"
        v-model:value="typeFilter"
        style="width: 130px; margin-left: 12px"
        placeholder="类型筛选"
        allow-clear
      >
        <a-select-option value="category.movie">电影</a-select-option>
        <a-select-option value="category.tv_series">电视剧</a-select-option>
        <a-select-option value="category.animation">动漫</a-select-option>
        <a-select-option value="category.documentaries">纪录片</a-select-option>
        <a-select-option value="category.tv_shows">综艺</a-select-option>
        <a-select-option value="category.music">音乐</a-select-option>
        <a-select-option value="category.sports">体育</a-select-option>
      </a-select>
      <a-select
        v-if="torrents.length"
        v-model:value="queryFilter"
        style="width: 130px; margin-left: 12px"
        placeholder="覆盖筛选"
        allow-clear
      >
        <a-select-option value="queried">已查询</a-select-option>
        <a-select-option value="unqueried">未查询</a-select-option>
      </a-select>
      <a-select
        v-if="torrents.length"
        v-model:value="stateFilter"
        mode="multiple"
        style="width: 160px; margin-left: 12px"
        placeholder="状态筛选"
        allow-clear
        :max-tag-count="1"
      >
        <a-select-option value="uploading">上传中</a-select-option>
        <a-select-option value="stalledUP">停滞做种</a-select-option>
        <a-select-option value="pausedUP">暂停做种</a-select-option>
        <a-select-option value="downloading">下载中</a-select-option>
        <a-select-option value="pausedDL">暂停下载</a-select-option>
        <a-select-option value="error">错误</a-select-option>
      </a-select>
      <a-select
        v-if="torrents.length"
        v-model:value="pathFilter"
        style="width: 180px; margin-left: 12px"
        placeholder="保存路径"
        allow-clear
      >
        <a-select-option v-for="p in savePaths" :key="p" :value="p">{{ p }}</a-select-option>
      </a-select>
      <a-tag v-if="torrents.length" color="blue" style="margin-left: 8px">
        {{ filteredTorrents.length }} / {{ torrents.length }}
      </a-tag>
      <a-button size="small" style="margin-left: auto" @click="groupMappingOpen = true">
        制作组映射
      </a-button>
      <a-button size="small" @click="openDeclFilters">
        过滤规则
      </a-button>
      <a-button
        v-if="selectedHashes.length > 0"
        size="small"
        :loading="batchFetching"
        @click="batchFetchData"
      >
        获取数据 ({{ selectedHashes.length }})
      </a-button>
      <a-button
        v-if="selectedHashes.length > 0"
        size="small"
        :loading="batchQuerying"
        @click="batchQueryCoverage"
      >
        批量查询 ({{ selectedHashes.length }})
      </a-button>
      <a-button
        v-if="selectedHashes.length > 0"
        type="primary"
        size="small"
        :loading="batchPublishing"
        @click="openBatchPublish"
      >
        批量发布 ({{ selectedHashes.length }})
      </a-button>
      <!-- 后台查询进度 -->
      <div v-if="querying" class="query-progress">
        <a-progress
          :percent="queryProgress"
          size="small"
          status="active"
          style="width: 200px"
        />
        <span class="progress-text">{{ queryDone }} / {{ queryTotal }}</span>
      </div>
      <!-- 批量获取数据进度 -->
      <div v-if="batchFetching" class="query-progress">
        <a-progress
          :percent="batchFetchProgress"
          size="small"
          status="active"
          style="width: 200px"
        />
        <span class="progress-text">{{ batchFetchDone }} / {{ selectedHashes.length }}</span>
      </div>
    </div>

    <a-table
      :columns="columns"
      :data-source="pagedTorrents"
      :loading="loading"
      :pagination="{
        current: currentPage,
        pageSize: pageSize,
        total: filteredTorrents.length,
        showSizeChanger: true,
        pageSizeOptions: ['50', '100', '200'],
        showTotal: (total: number) => `共 ${total} 个种子`,
        size: 'small',
      }"
      row-key="info_hash"
      size="small"
      :scroll="{ x: 1430 }"
      :sticky="{ offsetHeader: 48 }"
      :row-class-name="(record: any) => record.metadata_reviewed ? 'row-reviewed' : 'row-unreviewed'"
      :row-selection="{ selectedRowKeys: selectedHashes, onChange: (keys: string[]) => selectedHashes = keys }"
      @change="onTableChange"
    >
      <template #expandedRowRender="{ record }">
        <div class="coverage-expand">
          <template v-if="record.coverage?.sites?.length">
            <div class="coverage-group">
              <span class="coverage-label">🟢 做种中</span>
              <a-tag v-for="s in coverageSitesOf(record, 'green')" :key="s.site_name" color="success" style="margin: 2px">{{ s.site_name }}</a-tag>
              <span v-if="coverageSitesOf(record, 'green').length === 0" class="empty-hint">无</span>
            </div>
            <div class="coverage-group">
              <span class="coverage-label">🟡 可辅种</span>
              <a-tag v-for="s in coverageSitesOf(record, 'yellow')" :key="s.site_name" color="warning" style="margin: 2px">{{ s.site_name }}</a-tag>
              <span v-if="coverageSitesOf(record, 'yellow').length === 0" class="empty-hint">无</span>
            </div>
            <div class="coverage-group">
              <span class="coverage-label">⚪ 未发现</span>
              <span v-if="(record.coverage?.target_count ?? 0) > 0" style="font-size: 12px; color: #999">{{ record.coverage?.target_count }} 站（可转载发布）</span>
              <span v-else class="empty-hint">无</span>
            </div>
          </template>
          <span v-else class="empty-hint">{{ record.queried ? '已查询，暂无已知覆盖数据' : '尚未查询覆盖' }}</span>
        </div>
      </template>
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'source_site'">
          <template v-if="record.source_sites && record.source_sites.length">
            <a-tag v-for="s in record.source_sites" :key="s" color="blue" size="small" style="margin: 1px">{{ s }}</a-tag>
          </template>
          <span v-else style="color: #999; font-size: 12px">未知</span>
        </template>
        <template v-if="column.key === 'size'">
          {{ formatBytes(record.size) }}
        </template>
        <template v-if="column.key === 'state'">
          <a-tag :color="stateColor(record.state)" style="margin: 0">{{ translateQbState(record.state) }}</a-tag>
        </template>
        <template v-if="column.key === 'progress'">
          <a-progress :percent="Math.round(record.progress || 0)" :stroke-color="progressColor(record.progress || 0)" :show-info="false" />
          <span style="font-size: 11px; color: #666">{{ Math.round(record.progress || 0) }}%</span>
        </template>
        <template v-if="column.key === 'uploaded'">
          <span :style="{ color: record.uploaded > 0 ? '#52c41a' : '#999' }">{{ formatBytes(record.uploaded) }}</span>
        </template>
        <template v-if="column.key === 'save_path'">
          <a-tooltip :title="record.save_path">
            <span style="font-size: 11px; color: #666">{{ record.save_path }}</span>
          </a-tooltip>
        </template>
        <template v-if="column.key === 'coverage'">
          <a-tooltip>
            <template #title>
              <div v-if="record.coverage?.sites?.length">
                <div v-for="s in record.coverage.sites" :key="s.site_name" style="display: flex; align-items: center; gap: 4px; margin: 1px 0">
                  <span>{{ coverageEmoji(s.status, s.source) }}</span>
                  <a-tag :color="coverageColor(s.status, s.source)" size="small" style="margin: 0">
                    {{ s.site_name }}
                  </a-tag>
                </div>
              </div>
              <div v-else-if="record.queried" style="color: #999">
                已查询，暂无已知覆盖
              </div>
              <div v-else style="color: #999">尚未查询</div>
              <div v-if="record.coverage?.sites?.length" style="margin-top: 4px; border-top: 1px solid #333; padding-top: 4px; font-size: 11px">
                🟢 做种中 {{ coverageCount(record, 'green') }} · 🟡 可辅种 {{ coverageCount(record, 'yellow') }} · ⚪ 未发现 {{ record.coverage?.target_count ?? 0 }}
              </div>
            </template>
            <div class="coverage-cell">
              <span v-if="coverageCount(record, 'green') > 0" style="color: #52c41a; font-weight: 600">{{ coverageCount(record, 'green') }}</span>
              <span v-if="coverageCount(record, 'green') > 0" style="color: #999">·</span>
              <span v-if="coverageCount(record, 'yellow') > 0" style="color: #faad14; font-weight: 600">{{ coverageCount(record, 'yellow') }}</span>
              <span v-if="coverageCount(record, 'yellow') > 0" style="color: #999">·</span>
              <span style="color: #999">{{ record.coverage?.target_count ?? 0 }}</span>
              <a-tag v-if="!record.queried" color="orange" size="small" class="unqueried-tag">未查</a-tag>
            </div>
          </a-tooltip>
        </template>
        <template v-if="column.key === 'target_count'">
          <a-tag :color="(record.coverage?.target_count ?? 0) > 0 ? 'green' : 'default'">
            {{ record.coverage?.target_count ?? 0 }} 站可转
          </a-tag>
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button
              type="link"
              size="small"
              @click="openReview(record)"
            >
              核对
            </a-button>
            <a-button
              type="link"
              size="small"
              :loading="queryingHash === record.info_hash"
              @click="queryCoverage(record)"
            >
              查询覆盖
            </a-button>
            <a-button
              type="primary"
              size="small"
              :disabled="false"
              @click="startForward(record)"
            >
              转种
            </a-button>
          </a-space>
        </template>
      </template>
    </a-table>

    <CrossSeedPanel
      v-model:open="crossSeedOpen"
      :preset-torrent="presetTorrent"
      @success="onWizardSuccess"
    />

    <MetadataReviewModal
      v-model:open="reviewOpen"
      :info-hash="reviewHash"
      :torrent-name="reviewName"
      @saved="onReviewSaved"
    />

    <a-modal
      v-model:open="sourceSelectOpen"
      title="选择转种源站"
      width="520px"
      :footer="null"
    >
      <div v-if="sourceDetectRecord" style="margin-bottom: 16px">
        <span style="color: #666">种子：</span>
        <span>{{ sourceDetectRecord.name }}</span>
      </div>
      <p style="color: #999; margin-bottom: 16px">
        未自动识别制作组主站，请手动选择数据来源站点：
      </p>
      <a-radio-group v-model:value="selectedSourceSite" style="width: 100%">
        <div
          v-for="c in sourceCandidates"
          :key="c.site_name"
          style="display: flex; align-items: center; padding: 8px 0"
        >
          <a-radio :value="c.site_name" :disabled="!c.has_cookie">
            {{ c.site_name }}
          </a-radio>
          <a-tag v-if="c.torrent_id" color="blue" size="small" style="margin-left: 8px">
            ID: {{ c.torrent_id }}
          </a-tag>
          <a-tag v-if="!c.has_cookie" color="red" size="small" style="margin-left: 4px">
            缺 cookie
          </a-tag>
        </div>
      </a-radio-group>
      <div style="text-align: right; margin-top: 16px">
        <a-button style="margin-right: 8px" @click="sourceSelectOpen = false">取消</a-button>
        <a-button type="primary" :disabled="!selectedSourceSite" @click="confirmSourceSite">
          确定
        </a-button>
      </div>
    </a-modal>

    <a-modal
      v-model:open="batchPublishOpen"
      title="批量发布"
      width="480px"
      :confirm-loading="batchPublishing"
      @ok="doBatchPublish"
    >
      <a-alert
        type="info"
        show-icon
        :message="`将创建 ${selectedHashes.length} 个发布候选`"
        description="系统将自动获取 MediaInfo/截图/简介并发布到目标站，无需逐个核对。"
        style="margin-bottom: 16px"
      />
      <a-form layout="vertical">
        <a-form-item label="源站">
          <a-input v-model:value="batchForm.source_site" placeholder="源站名称" />
        </a-form-item>
        <a-form-item label="目标站">
          <a-select v-model:value="batchForm.target_site" placeholder="选择目标站" show-search>
            <a-select-option
              v-for="site in batchTargetSites"
              :key="site.name"
              :value="site.name"
              :label="site.name"
            >
              {{ site.name }}
            </a-select-option>
          </a-select>
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal
      v-model:open="declFilterOpen"
      title="声明过滤规则"
      width="600px"
      :footer="null"
      destroy-on-close
    >
      <a-alert
        type="info"
        show-icon
        message="过滤规则用于自动从简介中移除 ARDTU/CSAUTO 等工具声明"
        style="margin-bottom: 16px"
      />
      <div v-if="declFilterIsDefault" style="margin-bottom: 12px">
        <a-tag color="orange">当前使用默认规则</a-tag>
      </div>
      <a-list
        :data-source="declFilterPatterns"
        size="small"
        bordered
      >
        <template #renderItem="{ item, index }">
          <a-list-item>
            <div style="display: flex; justify-content: space-between; align-items: center; width: 100%">
              <span style="font-family: monospace; font-size: 13px">{{ item }}</span>
              <a-button type="link" danger size="small" @click="declFilterPatterns.splice(index, 1)">
                删除
              </a-button>
            </div>
          </a-list-item>
        </template>
      </a-list>
      <div style="margin-top: 12px; display: flex; gap: 8px">
        <a-input
          v-model:value="newDeclPattern"
          placeholder="输入过滤关键词（如 ARDTU工具自动发布）"
          @press-enter="addDeclPattern"
        />
        <a-button type="primary" size="small" @click="addDeclPattern">添加</a-button>
      </div>
      <div style="margin-top: 16px; text-align: right">
        <a-button style="margin-right: 8px" @click="declFilterOpen = false">取消</a-button>
        <a-button type="primary" :loading="declFilterSaving" @click="saveDeclFilters">保存</a-button>
      </div>
    </a-modal>

    <a-modal
      v-model:open="groupMappingOpen"
      title="制作组 → 源站映射"
      width="800px"
      :footer="null"
      destroy-on-close
    >
      <div style="margin-bottom: 12px; display: flex; gap: 8px">
        <a-input v-model:value="newMapping.group_name" placeholder="制作组名（如 CMCTV）" style="width: 160px" />
        <a-input v-model:value="newMapping.domain" placeholder="域名（如 springsunday.net）" style="width: 220px" />
        <a-input v-model:value="newMapping.site_name" placeholder="站点名（留空自动匹配）" style="width: 160px" />
        <a-button type="primary" size="small" @click="addMapping">添加</a-button>
      </div>
      <a-table
        :columns="mappingColumns"
        :data-source="mappings"
        :pagination="mappingPagination"
        row-key="id"
        size="small"
        @change="(pag: { current?: number; pageSize?: number }) => { if (pag.current) mappingPagination.current = pag.current; if (pag.pageSize) mappingPagination.pageSize = pag.pageSize }"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'domain'">
            {{ maskDomain(record.domain) }}
          </template>
          <template v-if="column.key === 'matched_site'">
            <a-tag v-if="record.matched_site" color="green">{{ record.matched_site }}</a-tag>
            <a-tag v-else color="red">未匹配</a-tag>
          </template>
          <template v-if="column.key === 'actions'">
            <a-button type="link" size="small" @click="editMapping(record)">编辑</a-button>
            <a-popconfirm v-if="!record.is_builtin" title="确定删除？" @confirm="deleteMapping(record.id)">
              <a-button type="link" danger size="small">删除</a-button>
            </a-popconfirm>
            <a-tooltip v-else title="内置官组不可删除">
              <a-button type="link" danger size="small" disabled>删除</a-button>
            </a-tooltip>
          </template>
        </template>
      </a-table>
    </a-modal>

    <a-modal
      v-model:open="editMappingOpen"
      title="编辑映射"
      width="420px"
      @ok="saveMapping"
    >
      <a-form layout="vertical">
        <a-form-item label="制作组名">
          <a-input v-model:value="editingMapping.group_name" />
        </a-form-item>
        <a-form-item label="域名">
          <a-input v-model:value="editingMapping.domain" />
        </a-form-item>
        <a-form-item label="站点名（手动指定，留空用域名自动匹配）">
          <a-input v-model:value="editingMapping.site_name" placeholder="留空自动匹配" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch, reactive } from 'vue'
import { message } from 'ant-design-vue'
import { publishTorrentsApi, manualForwardApi, type PublishTorrentItem } from '@/api/publish'
import { downloadersApi } from '@/api/downloaders'
import { formatBytes, maskDomain } from '@/utils/format'
import { useEnumLabels } from '@/utils/enumLabels'

const { translateQbState } = useEnumLabels()

function stateColor(state: string): string {
  const colors: Record<string, string> = {
    uploading: 'success',
    stalledUP: 'success',
    forcedUP: 'success',
    pausedUP: 'warning',
    pausedDL: 'warning',
    downloading: 'processing',
    stalledDL: 'processing',
    error: 'error',
    missingFiles: 'error',
  }
  return colors[state] || 'default'
}

function progressColor(pct: number): string {
  if (pct >= 100) return '#52c41a'
  if (pct >= 80) return '#faad14'
  return '#1677ff'
}
import CrossSeedPanel from './CrossSeedPanel.vue'
import MetadataReviewModal from './MetadataReviewModal.vue'

const clients = ref<{ id: number; name: string; type: string }[]>([])
const clientsLoading = ref(false)

const STORAGE_KEY = 'publish_torrents_filters'

interface PersistedFilters {
  client_id?: number
  search?: string
  query_filter?: string
  type_filter?: string
  state_filter?: string[]
  page_size?: number
}

function loadPersistedFilters(): PersistedFilters {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) return JSON.parse(raw) as PersistedFilters
  } catch { /* silent */ }
  return {}
}

function persistFilters() {
  const data: PersistedFilters = {
    client_id: selectedClientId.value,
    search: searchText.value || undefined,
    query_filter: queryFilter.value,
    type_filter: typeFilter.value,
    state_filter: stateFilter.value,
    page_size: pageSize.value,
  }
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(data))
  } catch { /* silent */ }
}

const persisted = loadPersistedFilters()

const selectedClientId = ref<number | undefined>(persisted.client_id)
const torrents = ref<PublishTorrentItem[]>([])
const loading = ref(false)
const searchText = ref(persisted.search || '')
const queryFilter = ref<string | undefined>(persisted.query_filter)
const typeFilter = ref<string | undefined>(persisted.type_filter)
const stateFilter = ref<string[]>(persisted.state_filter || [])
const pathFilter = ref<string | undefined>(undefined)
const savePaths = ref<string[]>([])
const queryingHash = ref('')
const selectedHashes = ref<string[]>([])
let coverageAbortController: AbortController | null = null

// 分页
const currentPage = ref(1)
const pageSize = ref(persisted.page_size || 50)

// 后台查询状态
const querying = ref(false)
const queryDone = ref(0)
const queryTotal = ref(0)

let pollTimer: ReturnType<typeof setInterval> | null = null

watch([selectedClientId, searchText, queryFilter, typeFilter, stateFilter, pageSize], persistFilters)

const crossSeedOpen = ref(false)
const reviewOpen = ref(false)
const reviewHash = ref('')
const reviewName = ref('')
const presetTorrent = ref<{ info_hash: string; name: string; size: number; save_path: string; client_id: number; state: string; source_site?: string; source_site_id?: number; torrent_id?: string } | null>(null)

const columns = [
  { title: '种子名称', dataIndex: 'name', key: 'name', ellipsis: true },
  { title: '做种站点', key: 'source_site', width: 160 },
  { title: '大小', key: 'size', width: 90 },
  { title: '状态', key: 'state', width: 90 },
  { title: '进度', key: 'progress', width: 110, align: 'center' as const },
  { title: '上传量', key: 'uploaded', width: 90 },
  { title: '保存路径', key: 'save_path', width: 150, ellipsis: true },
  { title: '覆盖', key: 'coverage', width: 120, align: 'center' as const },
  { title: '可转', key: 'target_count', width: 100 },
  { title: '操作', key: 'actions', width: 180 },
]

const filteredTorrents = computed(() => {
  let result = torrents.value
  if (searchText.value) {
    const q = searchText.value.toLowerCase()
    result = result.filter(t => t.name.toLowerCase().includes(q))
  }
  if (queryFilter.value === 'queried') {
    result = result.filter(t => t.queried)
  } else if (queryFilter.value === 'unqueried') {
    result = result.filter(t => !t.queried)
  }
  if (typeFilter.value) {
    result = result.filter((t: any) => (t as any).standard_type === typeFilter.value)
  }
  if (stateFilter.value.length > 0) {
    result = result.filter(t => stateFilter.value.includes(t.state))
  }
  if (pathFilter.value) {
    result = result.filter(t => t.save_path === pathFilter.value)
  }
  return result
})

const pagedTorrents = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredTorrents.value.slice(start, start + pageSize.value)
})

const queryProgress = computed(() => {
  if (queryTotal.value === 0) return 0
  return Math.round((queryDone.value / queryTotal.value) * 100)
})

function coverageColor(status: string, source?: string): string {
  if (status === 'confirmed_has' || status === 'probably_has') {
    if (source === 'tracker') return 'green'
    return 'gold'
  }
  return 'default'
}

function coverageEmoji(status: string, source?: string): string {
  if (status === 'confirmed_has' || status === 'probably_has') {
    if (source === 'tracker') return '🟢'
    return '🟡'
  }
  return '⚪'
}

function coverageCount(record: PublishTorrentItem, color: 'green' | 'yellow' | 'white'): number {
  const sites = record.coverage?.sites
  if (!sites?.length) return 0
  return sites.filter((s: { status: string; source: string }) => {
    const c = coverageColor(s.status, s.source)
    if (color === 'green') return c === 'green'
    if (color === 'yellow') return c === 'gold'
    return c === 'default'
  }).length
}

function coverageSitesOf(record: PublishTorrentItem, color: 'green' | 'yellow' | 'white') {
  const sites = record.coverage?.sites
  if (!sites?.length) return []
  return sites.filter((s: { status: string; source: string }) => {
    const c = coverageColor(s.status, s.source)
    if (color === 'green') return c === 'green'
    if (color === 'yellow') return c === 'gold'
    return c === 'default'
  })
}

function coverageSiteStatus(s: { status: string }): string {
  return s.status === 'confirmed_not' ? 'cached_not' : s.status
}

function onTableChange(pag: { current?: number; pageSize?: number }) {
  if (pag.current) currentPage.value = pag.current
  if (pag.pageSize) pageSize.value = pag.pageSize
}

function onClientChange() {
  currentPage.value = 1
  selectedHashes.value = []
  if (coverageAbortController) {
    coverageAbortController.abort()
    coverageAbortController = null
  }
  fetchTorrents()
}

async function fetchClients() {
  clientsLoading.value = true
  try {
    const resp = await downloadersApi.listLight(1, 100)
    const data = resp.data?.data
    clients.value = (data?.items || data || []) as { id: number; name: string; type: string }[]
    if (clients.value.length > 0) {
      const exists = selectedClientId.value && clients.value.some(c => c.id === selectedClientId.value)
      if (!exists) {
        selectedClientId.value = clients.value[0].id
      }
      fetchTorrents()
    }
  } catch { /* ignore */ } finally {
    clientsLoading.value = false
  }
}

async function fetchTorrents() {
  if (!selectedClientId.value) return
  loading.value = true
  try {
    const resp = await publishTorrentsApi.list(selectedClientId.value)
    const data = resp.data?.data
    torrents.value = data?.items || []
    // 提取保存路径列表
    const paths = new Set<string>()
    for (const t of torrents.value) {
      if (t.save_path) paths.add(t.save_path)
    }
    savePaths.value = [...paths].sort()
    pathFilter.value = undefined
    querying.value = data?.querying ?? false
    queryDone.value = data?.query_progress?.done ?? 0
    queryTotal.value = data?.query_progress?.total ?? 0

    if (querying.value) {
      startPolling()
    } else {
      stopPolling()
    }
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    loading.value = false
  }
}

async function pollQueryStatus() {
  if (!selectedClientId.value) return
  try {
    const resp = await publishTorrentsApi.queryStatus(selectedClientId.value)
    const data = resp.data?.data
    querying.value = data?.querying ?? false
    queryDone.value = data?.done ?? 0
    queryTotal.value = data?.total ?? 0

    if (querying.value) {
      // 查询进行中，同时刷新种子数据（覆盖数据在逐步填充）
      await refreshTorrentsSilent()
    } else {
      // 查询完成，最终刷新
      stopPolling()
      await refreshTorrentsSilent()
    }
  } catch { /* silent */ }
}

async function refreshTorrentsSilent() {
  if (!selectedClientId.value) return
  try {
    const resp = await publishTorrentsApi.list(selectedClientId.value)
    torrents.value = resp.data?.data?.items || []
  } catch { /* silent */ }
}

function startPolling() {
  if (pollTimer) return
  pollTimer = setInterval(pollQueryStatus, 5000)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

watch(queryFilter, () => { currentPage.value = 1 })
watch(typeFilter, () => { currentPage.value = 1 })

async function queryCoverage(record: PublishTorrentItem) {
  if (!selectedClientId.value) return
  // 取消上一个正在进行的查询
  if (coverageAbortController) {
    coverageAbortController.abort()
  }
  coverageAbortController = new AbortController()
  queryingHash.value = record.info_hash
  try {
    const resp = await publishTorrentsApi.queryCoverage({
      client_id: selectedClientId.value,
      info_hash: record.info_hash,
      name: record.name,
      size: record.size,
    })
    const result = resp.data?.data
    if (result) {
      record.coverage = {
        has_count: result.has_count,
        total_sites: result.total_sites,
        target_count: result.target_count,
        sites: result.sites,
      }
      record.queried = true
      message.success(`覆盖查询完成：${result.has_count}/${result.total_sites}`)
    }
  } catch (e: unknown) {
    if (e instanceof DOMException && e.name === 'AbortError') return
    message.error((e as Error).message)
  } finally {
    if (queryingHash.value === record.info_hash) {
      queryingHash.value = ''
      coverageAbortController = null
    }
  }
}

const batchFetchProgress = computed(() => {
  if (!selectedHashes.value.length) return 0
  return Math.round(batchFetchDone.value / selectedHashes.value.length * 100)
})

async function batchFetchData() {
  if (selectedHashes.value.length === 0 || !selectedClientId.value) return
  batchFetching.value = true
  batchFetchDone.value = 0
  let ok = 0
  let fail = 0
  for (const hash of selectedHashes.value) {
    const t = torrents.value.find(item => item.info_hash === hash)
    if (!t) continue
    try {
      const resp = await manualForwardApi.startAnalyze({
        client_id: selectedClientId.value,
        info_hash: t.info_hash,
        name: t.name,
        save_path: t.save_path,
        size: t.size,
        fetch_source: 'batch_fetch',
      })
      const taskId = resp.data?.data?.task_id
      if (!taskId) throw new Error('任务创建失败')
      await pollAnalyzeTask(taskId)
      ok++
    } catch {
      fail++
    }
    batchFetchDone.value++
  }
  batchFetching.value = false
  message.success(`获取数据完成: ${ok} 成功, ${fail} 失败`)
}

function pollAnalyzeTask(taskId: number): Promise<void> {
  return new Promise((resolve, reject) => {
    async function poll() {
      try {
        const resp = await manualForwardApi.pollAnalyze(taskId)
        const task = resp.data?.data as Record<string, unknown> | undefined
        if (!task) { resolve(); return }
        const status = task.status as string
        if (status === 'done') { resolve(); return }
        if (status === 'failed') { reject(new Error(task.error as string || '分析失败')); return }
        setTimeout(poll, 2000)
      } catch (e) {
        reject(e)
      }
    }
    setTimeout(poll, 1500)
  })
}

async function batchQueryCoverage() {
  if (selectedHashes.value.length === 0 || !selectedClientId.value) return
  batchQuerying.value = true
  queryingHash.value = '批量查询中...'
  try {
    await publishTorrentsApi.batchQueryCoverage({
      client_id: selectedClientId.value,
      info_hashes: [...selectedHashes.value],
    })
    message.success(`批量查询完成: ${selectedHashes.value.length} 个种子`)
    await fetchTorrents()
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    batchQuerying.value = false
    queryingHash.value = ''
  }
}

function startForward(record: PublishTorrentItem) {
  presetTorrent.value = {
    info_hash: record.info_hash,
    name: record.name,
    size: record.size,
    save_path: record.save_path,
    client_id: selectedClientId.value!,
    state: record.state,
  }
  // 检测源头站
  detectAndOpen(record)
}

async function detectAndOpen(record: PublishTorrentItem) {
  try {
    const resp = await publishTorrentsApi.detectSource({
      info_hash: record.info_hash,
      name: record.name,
    })
    const result = resp.data?.data
    if (result?.source_site) {
      // 自动检测成功（或降级选中）
      presetTorrent.value = {
        ...presetTorrent.value!,
        source_site: result.source_site,
        source_site_id: result.source_site_id,
        torrent_id: result.torrent_id,
      }
      if (!result.auto_detected && result.candidates?.length > 1) {
        // 非自动判断 + 多候选 → 弹选择弹窗
        sourceCandidates.value = result.candidates
        sourceDetectRecord.value = record
        sourceSelectOpen.value = true
        return
      }
    }
    // 直接打开 CrossSeedPanel
    crossSeedOpen.value = true
  } catch {
    // 检测失败 → 直接打开 CrossSeedPanel（用默认源站）
    crossSeedOpen.value = true
  }
}

// 源站选择弹窗
const sourceSelectOpen = ref(false)
const sourceCandidates = ref<{ site_name: string; torrent_id: string; has_cookie: boolean }[]>([])
const sourceDetectRecord = ref<PublishTorrentItem | null>(null)
const selectedSourceSite = ref<string>('')

function confirmSourceSite() {
  const cand = sourceCandidates.value.find(c => c.site_name === selectedSourceSite.value)
  if (cand && presetTorrent.value) {
    presetTorrent.value = {
      ...presetTorrent.value,
      source_site: cand.site_name,
      torrent_id: cand.torrent_id,
    }
  }
  sourceSelectOpen.value = false
  crossSeedOpen.value = true
}

function onWizardSuccess() {
  fetchTorrents()
}

function openReview(record: PublishTorrentItem) {
  reviewHash.value = record.info_hash
  reviewName.value = record.name
  reviewOpen.value = true
}

function onReviewSaved(infoHash: string) {
  const t = torrents.value.find((x: PublishTorrentItem) => x.info_hash === infoHash)
  if (t) {
    ;(t as any).metadata_reviewed = true
  }
}

// --- 批量发布 ---
const batchPublishOpen = ref(false)
const batchPublishing = ref(false)
const batchQuerying = ref(false)
const batchFetching = ref(false)
const batchFetchDone = ref(0)
const batchTargetSites = ref<{ name: string }[]>([])
const batchForm = reactive({ source_site: '', target_site: '' })

async function openBatchPublish() {
  batchForm.source_site = ''
  batchForm.target_site = ''
  // 自动检测源站（用第一个选中种子的制作组）
  if (selectedHashes.value.length > 0 && torrents.value.length > 0) {
    const firstHash = selectedHashes.value[0]
    const torrent = torrents.value.find(t => t.info_hash === firstHash)
    if (torrent) {
      try {
        const resp = await publishTorrentsApi.detectSource({
          info_hash: torrent.info_hash,
          name: torrent.name,
        })
        if (resp.data?.data?.source_site) {
          batchForm.source_site = resp.data.data.source_site
        }
      } catch { /* ignore */ }
    }
  }
  // 加载目标站列表
  try {
    // 用 sitesApi 获取目标站
    const sitesResp = await import('@/api/sites').then(m => m.sitesApi.list(1, 200))
    const data = sitesResp.data?.data
    batchTargetSites.value = ((data?.items || data || []) as { name: string; is_target?: boolean }[])
      .filter(s => s.is_target !== false)
      .map(s => ({ name: s.name }))
  } catch { /* ignore */ }
  batchPublishOpen.value = true
}

async function doBatchPublish() {
  if (!batchForm.source_site || !batchForm.target_site || !selectedClientId.value) {
    message.warning('源站和目标站必填')
    return
  }
  batchPublishing.value = true
  try {
    const items = selectedHashes.value.map(hash => {
      const t = torrents.value.find(t => t.info_hash === hash)
      return {
        info_hash: hash,
        name: t?.name || '',
        size: t?.size || 0,
        save_path: t?.save_path || '',
      }
    })
    const resp = await publishTorrentsApi.batchPublish({
      client_id: selectedClientId.value,
      source_site: batchForm.source_site,
      target_site: batchForm.target_site,
      items,
    })
    const result = resp.data?.data
    if (result) {
      message.success(`已创建 ${result.created} 个发布候选${result.failed > 0 ? `，${result.failed} 个失败` : ''}`)
    }
    batchPublishOpen.value = false
    selectedHashes.value = []
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    batchPublishing.value = false
  }
}

onMounted(fetchClients)

onUnmounted(() => {
  stopPolling()
  if (coverageAbortController) {
    coverageAbortController.abort()
  }
})

// --- 映射管理 ---
const groupMappingOpen = ref(false)
const mappings = ref<Array<Record<string, unknown> & { id: number }>>([])
const editMappingOpen = ref(false)
const editingMapping = ref({ id: 0, group_name: '', domain: '', site_name: '' })
const newMapping = reactive({ group_name: '', domain: '', site_name: '' })

const mappingColumns = [
  { title: '制作组', dataIndex: 'group_name', key: 'group_name', width: 120 },
  { title: '域名', dataIndex: 'domain', key: 'domain', ellipsis: true },
  { title: '匹配站点', key: 'matched_site', width: 120 },
  { title: '操作', key: 'actions', width: 120 },
]
const mappingPagination = reactive({
  current: 1,
  pageSize: 20,
  showSizeChanger: true,
  pageSizeOptions: ['20', '50', '100'],
  showTotal: (total: number) => `共 ${total} 条`,
  size: 'small' as const,
})

async function fetchMappings() {
  try {
    const resp = await publishTorrentsApi.listGroupMappings()
    mappings.value = (resp.data?.data?.items || []) as Array<Record<string, unknown> & { id: number }>
  } catch { /* ignore */ }
}

watch(groupMappingOpen, (val) => {
  if (val) fetchMappings()
})

async function addMapping() {
  if (!newMapping.group_name) return
  try {
    await publishTorrentsApi.createGroupMapping({
      group_name: newMapping.group_name,
      domain: newMapping.domain,
      site_name: newMapping.site_name,
    })
    newMapping.group_name = ''
    newMapping.domain = ''
    newMapping.site_name = ''
    fetchMappings()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

function editMapping(record: Record<string, unknown> & { id: number }) {
  editingMapping.value = {
    id: record.id,
    group_name: record.group_name as string,
    domain: record.domain as string,
    site_name: record.site_name as string || '',
  }
  editMappingOpen.value = true
}

async function saveMapping() {
  try {
    await publishTorrentsApi.updateGroupMapping(editingMapping.value.id, {
      group_name: editingMapping.value.group_name,
      domain: editingMapping.value.domain,
      site_name: editingMapping.value.site_name,
    })
    editMappingOpen.value = false
    fetchMappings()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function deleteMapping(id: number) {
  try {
    await publishTorrentsApi.deleteGroupMapping(id)
    fetchMappings()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

// --- 声明过滤规则管理 ---
const declFilterOpen = ref(false)
const declFilterPatterns = ref<string[]>([])
const declFilterIsDefault = ref(false)
const declFilterSaving = ref(false)
const newDeclPattern = ref('')

async function openDeclFilters() {
  try {
    const resp = await publishTorrentsApi.getDeclarationFilters()
    const data = resp.data?.data
    declFilterPatterns.value = data?.patterns || []
    declFilterIsDefault.value = data?.is_default ?? true
  } catch { /* ignore */ }
  newDeclPattern.value = ''
  declFilterOpen.value = true
}

function addDeclPattern() {
  const pattern = newDeclPattern.value.trim()
  if (pattern && !declFilterPatterns.value.includes(pattern)) {
    declFilterPatterns.value.push(pattern)
    newDeclPattern.value = ''
  }
}

async function saveDeclFilters() {
  declFilterSaving.value = true
  try {
    await publishTorrentsApi.setDeclarationFilters(declFilterPatterns.value)
    message.success('过滤规则已保存')
    declFilterOpen.value = false
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    declFilterSaving.value = false
  }
}
</script>

<style scoped>
.page-toolbar {
  display: flex;
  align-items: center;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 4px;
  position: sticky;
  top: 0;
  z-index: 10;
  background: #fff;
  padding: 8px 0;
}
.coverage-cell {
  font-size: 16px;
  font-weight: 600;
  cursor: default;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.coverage-has {
  color: #52c41a;
}
.coverage-sep {
  color: #d9d9d9;
}
.coverage-total {
  color: #999;
}
.unqueried-tag {
  margin-left: 4px;
  transform: scale(0.85);
}
.query-progress {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}
.progress-text {
  font-size: 12px;
  color: #666;
  white-space: nowrap;
}
.coverage-expand {
  padding: 8px 0;
}
.coverage-group {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 2px;
  margin-bottom: 6px;
}
.coverage-label {
  font-size: 12px;
  color: #666;
  min-width: 80px;
  font-weight: 500;
}
.empty-hint {
  font-size: 12px;
  color: #bbb;
}
</style>

<style>
.row-reviewed {
  background-color: #f6ffed !important;
}
.row-unreviewed {
  background-color: #fffbe6 !important;
}
</style>
