<template>
  <a-modal
    :open="open"
    title="手动转发"
    width="1100px"
    :body-style="{ padding: '0' }"
    :footer="null"
    destroy-on-close
    @cancel="handleCancel"
  >
    <div class="wizard-shell">
      <div class="wizard-steps">
        <div
          v-for="(step, i) in stepLabels"
          :key="i"
          class="wizard-step"
          :class="{ active: currentStep === i, done: currentStep > i }"
        >
          <div class="wizard-step-icon">
            <CheckOutlined v-if="currentStep > i" />
            <span v-else>{{ i + 1 }}</span>
          </div>
          <span class="wizard-step-title">{{ step }}</span>
          <div v-if="i < stepLabels.length - 1" class="wizard-step-line" :class="{ filled: currentStep > i }" />
        </div>
      </div>

      <div class="wizard-body">
        <!-- Step 0: Select Torrent -->
        <div v-if="currentStep === 0" class="step-content">
          <div class="step-toolbar">
            <a-select
              v-model:value="selectedClientId"
              style="width: 260px"
              :loading="clientsLoading"
              placeholder="选择下载器"
              @change="fetchSeededTorrents"
            >
              <a-select-option v-for="c in clients" :key="c.id" :value="c.id">
                {{ c.name }} ({{ c.type }})
              </a-select-option>
            </a-select>
            <a-input-search
              v-if="seededTorrents.length"
              v-model:value="torrentSearch"
              placeholder="搜索种子名称..."
              style="width: 280px; margin-left: 12px"
            />
          </div>
          <a-table
            :columns="torrentColumns"
            :data-source="filteredTorrents"
            :loading="torrentsLoading"
            :pagination="{ pageSize: 10, showSizeChanger: false, size: 'small' }"
            row-key="info_hash"
            size="small"
            :row-selection="{ type: 'radio', selectedRowKeys: selectedTorrent ? [selectedTorrent.info_hash] : [], onSelect: (r: unknown) => { selectedTorrent = r as SeededTorrent } }"
            :scroll="{ y: 360 }"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'size'">
                {{ formatBytes(record.size) }}
              </template>
              <template v-if="column.key === 'state'">
                <a-tag :color="qbStateColor(record.state)">{{ translateQbState(record.state) }}</a-tag>
              </template>
            </template>
          </a-table>
        </div>

        <!-- Step 1: Review Details -->
        <div v-else-if="currentStep === 1" class="step-content">
          <a-spin :spinning="analyzing" tip="正在分析种子信息...">
            <div v-if="!analyzing && analyzeResult" class="review-tabs">
              <a-alert
                v-if="analyzeResult.forbidden"
                type="error"
                show-icon
                message="该种子禁止转载"
                :description="analyzeResult.forbid_reason as string"
                style="margin-bottom: 16px"
              />
              <a-tabs v-model:active-key="reviewTab">
                <a-tab-pane key="main" tab="主要信息">
                  <a-form layout="vertical">
                    <a-form-item label="标题">
                      <a-input v-model:value="form.title" />
                    </a-form-item>
                    <a-form-item label="副标题">
                      <a-input v-model:value="form.subtitle" placeholder="可选" />
                    </a-form-item>
                    <a-row :gutter="16">
                      <a-col :span="12">
                        <a-form-item label="源站">
                          <a-tag color="blue">{{ analyzeResult.source_site || '-' }}</a-tag>
                        </a-form-item>
                      </a-col>
                      <a-col :span="12">
                        <a-form-item label="大小">
                          <span>{{ formatBytes(selectedTorrent?.size ?? 0) }}</span>
                        </a-form-item>
                      </a-col>
                    </a-row>
                  </a-form>
                </a-tab-pane>

                <a-tab-pane v-if="form.screenshots.length" key="screenshots" tab="截图">
                  <div class="screenshot-grid">
                    <div v-for="(url, i) in form.screenshots" :key="i" class="screenshot-item">
                      <a-image :src="url" :width="180" :height="100" class="screenshot-img" />
                      <a-button type="link" danger size="small" @click="form.screenshots.splice(i, 1)">
                        <DeleteOutlined />
                      </a-button>
                    </div>
                  </div>
                </a-tab-pane>

                <a-tab-pane key="mediainfo" tab="MediaInfo">
                  <a-textarea
                    v-model:value="form.mediaInfo"
                    :rows="14"
                    class="mono-font"
                    placeholder="MediaInfo"
                  />
                </a-tab-pane>

                <a-tab-pane key="intro" tab="简介">
                  <a-textarea
                    v-model:value="form.description"
                    :rows="14"
                    placeholder="资源简介"
                  />
                </a-tab-pane>
              </a-tabs>
            </div>
            <a-result
              v-if="!analyzing && analyzeError"
              status="error"
              :title="analyzeError"
              style="padding: 40px 0"
            />
          </a-spin>
        </div>

        <!-- Step 2: Select Sites -->
        <div v-else-if="currentStep === 2" class="step-content">
          <div class="site-selection-header">
            <h3>选择目标站点</h3>
            <p class="hint">绿色=已选，红色=不可用（缺凭证/互斥规则），灰色=可选</p>
          </div>
          <div class="site-selection-actions">
            <a-button size="small" @click="selectAllAvailable">全选可用</a-button>
            <a-button size="small" @click="selectedTargets = []">清空</a-button>
            <span class="selected-count">
              已选 <strong>{{ selectedTargets.length }}</strong> / {{ siteList.length }} 站点
            </span>
          </div>
          <a-spin :spinning="targetsLoading">
            <div class="site-grid">
              <a-tooltip
                v-for="site in siteList"
                :key="site.name"
                :title="site.blocked ? site.blockReason : ''"
                :disabled="!site.blocked"
              >
                <div
                  class="site-btn"
                  :class="{
                    selected: selectedTargets.includes(site.name),
                    blocked: site.blocked,
                  }"
                  @click="toggleSite(site)"
                >
                  <span class="site-btn-name">{{ site.name }}</span>
                  <a-tag
                    v-if="site.blocked"
                    color="red"
                    size="small"
                    class="site-btn-tag"
                  >
                    禁
                  </a-tag>
                  <CheckCircleFilled
                    v-if="selectedTargets.includes(site.name)"
                    class="site-btn-check"
                  />
                </div>
              </a-tooltip>
            </div>
          </a-spin>
        </div>

        <!-- Step 3: Results -->
        <div v-else-if="currentStep === 3" class="step-content">
          <a-result
            v-if="submitError"
            status="error"
            title="发布失败"
            :sub-title="submitError"
            style="padding: 40px 0"
          >
            <template #extra>
              <a-button @click="currentStep = 2">返回重试</a-button>
            </template>
          </a-result>
          <div v-else class="results-container">
            <a-result
              status="success"
              :title="`已创建发布候选 #${submittedCandidateId}`"
              sub-title="候选已进入发布队列，系统将自动处理"
              style="padding: 20px 0"
            />
            <a-card title="目标站点发布状态" size="small" style="margin-top: 16px">
              <div class="result-site-grid">
                <div
                  v-for="site in selectedTargets"
                  :key="site"
                  class="result-site-card"
                >
                  <div class="result-site-card-bar queued" />
                  <ClockCircleOutlined class="result-site-card-icon" />
                  <div class="result-site-card-name">{{ site }}</div>
                  <a-tag color="blue" size="small">排队中</a-tag>
                </div>
              </div>
            </a-card>
          </div>
        </div>
      </div>

      <div class="wizard-footer">
        <div class="wizard-footer-left">
          <span v-if="currentStep === 1 && !analyzing && analyzeResult?.forbidden" class="footer-warn">
            <WarningFilled /> 该种子禁止转载
          </span>
        </div>
        <div class="wizard-footer-right">
          <template v-if="currentStep === 0">
            <a-button @click="handleCancel">取消</a-button>
            <a-button type="primary" :disabled="!selectedTorrent" @click="enterAnalyze">
              下一步
            </a-button>
          </template>
          <template v-else-if="currentStep === 1">
            <a-button :disabled="analyzing" @click="currentStep = 0">上一步</a-button>
            <a-button
              type="primary"
              :disabled="analyzing || !!analyzeError || (analyzeResult?.forbidden ?? false)"
              @click="enterSelectSites"
            >
              下一步
            </a-button>
          </template>
          <template v-else-if="currentStep === 2">
            <a-button @click="currentStep = 1">上一步</a-button>
            <a-button
              type="primary"
              :disabled="selectedTargets.length === 0"
              :loading="submitting"
              @click="doSubmit"
            >
              立即发布
            </a-button>
          </template>
          <template v-else-if="currentStep === 3">
            <a-button v-if="!submitError" @click="handlePublishAnother">再发一个</a-button>
            <a-button type="primary" @click="handleDone">完成</a-button>
          </template>
        </div>
      </div>
    </div>
  </a-modal>
</template>

<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { message } from 'ant-design-vue'
import {
  CheckOutlined, CheckCircleFilled, ClockCircleOutlined,
  WarningFilled, DeleteOutlined,
} from '@ant-design/icons-vue'
import { manualForwardApi, publishApi } from '@/api/publish'
import { downloadersApi } from '@/api/downloaders'
import { useEnumLabels } from '@/utils/enumLabels'
import { formatBytes } from '@/utils/format'

const { translateQbState } = useEnumLabels()

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{
  'update:open': [value: boolean]
  success: []
}>()

const stepLabels = ['选择种子', '核对详情', '选择站点', '完成']
const currentStep = ref(0)

// --- Step 0: Select Torrent ---
interface SeededTorrent {
  info_hash: string
  name: string
  size: number
  save_path: string
  upload_speed: number
  seeders: number
  state: string
  client_id: number
}

const clients = ref<{ id: number; name: string; type: string }[]>([])
const clientsLoading = ref(false)
const selectedClientId = ref<number | undefined>(undefined)
const seededTorrents = ref<SeededTorrent[]>([])
const torrentsLoading = ref(false)
const selectedTorrent = ref<SeededTorrent | null>(null)
const torrentSearch = ref('')

const filteredTorrents = computed(() => {
  if (!torrentSearch.value) return seededTorrents.value
  const q = torrentSearch.value.toLowerCase()
  return seededTorrents.value.filter(t => t.name.toLowerCase().includes(q))
})

const torrentColumns = [
  { title: '种子名称', dataIndex: 'name', key: 'name', ellipsis: true },
  { title: '大小', key: 'size', width: 90 },
  { title: '状态', key: 'state', width: 90 },
]

function qbStateColor(state: string): string {
  const map: Record<string, string> = {
    uploadingUP: 'green', stalledUP: 'cyan', pausedUP: 'orange',
    queuedUP: 'blue', checkingUP: 'geekblue', forcedUP: 'green',
  }
  return map[state] || 'default'
}

// --- Step 1: Analyze & Review ---
interface AnalyzeResult {
  source_site?: string
  source_site_id?: number
  title?: string
  subtitle?: string
  media_info?: string
  description?: string
  screenshots?: string[]
  forbidden?: boolean
  forbid_reason?: string
  blocked_targets?: string[]
}

const analyzing = ref(false)
const analyzeResult = ref<AnalyzeResult | null>(null)
const analyzeError = ref('')
const reviewTab = ref('main')

const form = ref({
  title: '',
  subtitle: '',
  mediaInfo: '',
  description: '',
  screenshots: [] as string[],
})

let pollTimer: ReturnType<typeof setTimeout> | null = null

onBeforeUnmount(() => {
  if (pollTimer !== null) clearTimeout(pollTimer)
})

async function fetchClients() {
  clientsLoading.value = true
  try {
    const resp = await downloadersApi.list(1, 100)
    const data = resp.data?.data
    clients.value = (data?.items || data || []) as { id: number; name: string; type: string }[]
    if (clients.value.length > 0 && !selectedClientId.value) {
      selectedClientId.value = clients.value[0].id
      fetchSeededTorrents()
    }
  } catch { /* ignore */ } finally {
    clientsLoading.value = false
  }
}

async function fetchSeededTorrents() {
  if (!selectedClientId.value) return
  torrentsLoading.value = true
  try {
    const resp = await manualForwardApi.seededTorrents(selectedClientId.value)
    seededTorrents.value = (resp.data?.data || []) as SeededTorrent[]
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    torrentsLoading.value = false
  }
}

async function enterAnalyze() {
  if (!selectedTorrent.value) return
  currentStep.value = 1
  analyzing.value = true
  analyzeError.value = ''
  analyzeResult.value = null

  try {
    const startResp = await manualForwardApi.startAnalyze({
      client_id: selectedTorrent.value.client_id,
      info_hash: selectedTorrent.value.info_hash,
      name: selectedTorrent.value.name,
      save_path: selectedTorrent.value.save_path,
    })
    const respData = startResp.data?.data as Record<string, unknown> | undefined
    const taskId = Number(respData?.task_id ?? respData?.taskId)
    if (!taskId) {
      analyzeError.value = '未返回 task_id'
      analyzing.value = false
      return
    }

    const poll = async () => {
      try {
        const pollResp = await manualForwardApi.pollAnalyze(taskId)
        const task = pollResp.data?.data as
          { status: string; result?: Record<string, unknown>; error?: string }
          | undefined
        if (task?.status === 'completed') {
          const r = (task.result ?? {}) as AnalyzeResult
          analyzeResult.value = r
          form.value.title = r.title || selectedTorrent.value?.name || ''
          form.value.subtitle = r.subtitle || ''
          form.value.mediaInfo = r.media_info || ''
          form.value.description = r.description || ''
          form.value.screenshots = r.screenshots || []
          analyzing.value = false
        } else if (task?.status === 'failed') {
          analyzeError.value = task.error || '分析失败'
          analyzing.value = false
        } else {
          pollTimer = setTimeout(poll, 2000)
        }
      } catch (e: unknown) {
        analyzeError.value = (e as Error).message
        analyzing.value = false
      }
    }
    pollTimer = setTimeout(poll, 1500)
  } catch (e: unknown) {
    analyzeError.value = (e as Error).message
    analyzing.value = false
  }
}

// --- Step 2: Select Sites ---
interface SiteItem {
  name: string
  domain: string
  blocked: boolean
  blockReason: string
}

const siteList = ref<SiteItem[]>([])
const selectedTargets = ref<string[]>([])
const targetsLoading = ref(false)

async function enterSelectSites() {
  currentStep.value = 2
  selectedTargets.value = []
  targetsLoading.value = true
  try {
    const blockedTargets = (analyzeResult.value?.blocked_targets as string[]) || []
    const resp = await manualForwardApi.eligibleTargets({
      source_site: analyzeResult.value?.source_site || '',
      blocked_targets: blockedTargets,
    })
    const raw = (resp.data?.data || []) as unknown[]
    siteList.value = raw.map((item) => {
      const obj = item as Record<string, unknown>
      const blocked = !!obj.blocked
      const name = obj.name as string
      let blockReason = ''
      if (blocked) {
        if (blockedTargets.includes(name)) {
          blockReason = '互斥规则：源站与该站不可互转'
        } else {
          blockReason = '该站点缺少 cookie 或 passkey'
        }
      }
      return {
        name,
        domain: obj.domain as string || obj.base_url as string || '',
        blocked,
        blockReason,
      }
    })
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    targetsLoading.value = false
  }
}

function toggleSite(site: SiteItem) {
  if (site.blocked) return
  const idx = selectedTargets.value.indexOf(site.name)
  if (idx >= 0) {
    selectedTargets.value.splice(idx, 1)
  } else {
    selectedTargets.value.push(site.name)
  }
}

function selectAllAvailable() {
  selectedTargets.value = siteList.value.filter(s => !s.blocked).map(s => s.name)
}

// --- Step 3: Submit & Results ---
const submitting = ref(false)
const submitError = ref('')
const submittedCandidateId = ref(0)

async function doSubmit() {
  if (!selectedTorrent.value || selectedTargets.value.length === 0) return
  submitting.value = true
  submitError.value = ''
  try {
    const resp = await manualForwardApi.submit({
      client_id: selectedTorrent.value.client_id,
      info_hash: selectedTorrent.value.info_hash,
      name: selectedTorrent.value.name,
      save_path: selectedTorrent.value.save_path,
      source_site: analyzeResult.value?.source_site,
      source_site_id: analyzeResult.value?.source_site_id,
      title: form.value.title,
      description: form.value.description,
      media_info: form.value.mediaInfo,
      screenshots: form.value.screenshots,
      target_sites: selectedTargets.value,
    })
    submittedCandidateId.value =
      (resp.data?.data as unknown as { candidate_id?: number })?.candidate_id || 0
    currentStep.value = 3
    message.success('发布候选已创建')
  } catch (e: unknown) {
    submitError.value = (e as Error).message
    currentStep.value = 3
  } finally {
    submitting.value = false
  }
}

// --- Modal lifecycle ---
watch(() => props.open, (val) => {
  if (val) {
    resetWizard()
    fetchClients()
  }
})

function resetWizard() {
  currentStep.value = 0
  selectedTorrent.value = null
  analyzeResult.value = null
  analyzeError.value = ''
  siteList.value = []
  selectedTargets.value = []
  submitError.value = ''
  submittedCandidateId.value = 0
  form.value = { title: '', subtitle: '', mediaInfo: '', description: '', screenshots: [] }
  reviewTab.value = 'main'
}

function handleCancel() {
  emit('update:open', false)
}

function handlePublishAnother() {
  resetWizard()
  fetchClients()
}

function handleDone() {
  emit('success')
  emit('update:open', false)
}

// silence unused import for publishApi (reserved for candidate polling)
void publishApi
</script>

<style scoped>
.wizard-shell {
  display: flex;
  flex-direction: column;
  height: 72vh;
}

/* --- Steps --- */
.wizard-steps {
  display: flex;
  align-items: center;
  padding: 20px 32px 12px;
  flex-shrink: 0;
}
.wizard-step {
  display: flex;
  align-items: center;
  flex-shrink: 0;
}
.wizard-step-icon {
  width: 28px;
  height: 28px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  font-weight: 600;
  background: #f0f0f0;
  color: #999;
  transition: all 0.3s;
}
.wizard-step.active .wizard-step-icon {
  background: #1677ff;
  color: #fff;
}
.wizard-step.done .wizard-step-icon {
  background: #52c41a;
  color: #fff;
}
.wizard-step-title {
  margin-left: 8px;
  font-size: 14px;
  color: #999;
  white-space: nowrap;
}
.wizard-step.active .wizard-step-title {
  color: #1677ff;
  font-weight: 600;
}
.wizard-step.done .wizard-step-title {
  color: #52c41a;
}
.wizard-step-line {
  width: 40px;
  height: 2px;
  background: #e8e8e8;
  margin: 0 12px;
  transition: background 0.3s;
}
.wizard-step-line.filled {
  background: #52c41a;
}

/* --- Body --- */
.wizard-body {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 0 32px;
  min-height: 0;
}
.step-content {
  padding-bottom: 16px;
}
.step-toolbar {
  margin-bottom: 12px;
}

/* --- Review Tabs --- */
.review-tabs :deep(.ant-tabs-content) {
  padding-top: 8px;
}
.mono-font {
  font-family: 'Courier New', 'Consolas', monospace;
  font-size: 12px;
}
.screenshot-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
.screenshot-item {
  position: relative;
  text-align: center;
}
.screenshot-img {
  border-radius: 4px;
  border: 1px solid #e8e8e8;
  object-fit: cover;
}

/* --- Site Selection --- */
.site-selection-header h3 {
  margin: 0 0 4px;
  font-size: 16px;
}
.site-selection-header .hint {
  margin: 0 0 12px;
  font-size: 12px;
  color: #999;
}
.site-selection-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
}
.selected-count {
  margin-left: 8px;
  font-size: 13px;
  color: #666;
}
.site-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.site-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 8px 16px;
  border: 2px solid #e8e8e8;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
  user-select: none;
  background: #fafafa;
}
.site-btn:hover:not(.blocked) {
  border-color: #1677ff;
}
.site-btn.selected {
  border-color: #52c41a;
  background: #f6ffed;
}
.site-btn.blocked {
  border-color: #ffa39e;
  background: #fff1f0;
  cursor: not-allowed;
  opacity: 0.7;
}
.site-btn-name {
  font-size: 14px;
}
.site-btn.selected .site-btn-name {
  font-weight: 600;
  color: #389e0d;
}
.site-btn-tag {
  margin-left: 2px;
  transform: scale(0.85);
}
.site-btn-check {
  color: #52c41a;
  font-size: 16px;
}

/* --- Results --- */
.results-container {
  padding: 8px 0;
}
.result-site-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
  gap: 12px;
}
.result-site-card {
  text-align: center;
  padding: 16px 8px 12px;
  border: 1px solid #f0f0f0;
  border-radius: 6px;
  position: relative;
  overflow: hidden;
}
.result-site-card-bar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 4px;
}
.result-site-card-bar.queued { background: #1677ff; }
.result-site-card-bar.success { background: #52c41a; }
.result-site-card-bar.failed { background: #ff4d4f; }
.result-site-card-icon {
  font-size: 28px;
  color: #1677ff;
}
.result-site-card-name {
  font-size: 14px;
  font-weight: 600;
  margin: 8px 0 6px;
}

/* --- Footer --- */
.wizard-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 32px;
  border-top: 1px solid #f0f0f0;
  flex-shrink: 0;
}
.wizard-footer-left {
  flex: 1;
}
.footer-warn {
  color: #ff4d4f;
  font-size: 13px;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.wizard-footer-right {
  display: flex;
  gap: 8px;
}
.wizard-footer-right button {
  min-width: 80px;
}
</style>
