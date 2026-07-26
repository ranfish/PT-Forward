<template>
  <a-drawer
    :open="open"
    :title="null"
    width="92%"
    placement="right"
    :body-style="{ padding: '0' }"
    :header-style="{ display: 'none' }"
    destroy-on-close
    @close="handleClose"
  >
    <div class="csp-shell">
      <!-- ═══ Header ═══ -->
      <div class="csp-header">
        <div class="csp-header-left">
          <span class="csp-header-title">转种发布</span>
          <a-tag v-if="selectedTorrent" color="blue">{{ selectedTorrent.name }}</a-tag>
        </div>
        <div class="csp-header-right">
          <!-- 源站切换 -->
          <a-select
            v-if="cachedSites.length > 0"
            v-model:value="currentSourceSite"
            size="small"
            style="width: 200px"
            placeholder="选择源站"
            @change="onSourceSiteChange"
          >
            <a-select-option v-for="s in cachedSites" :key="s.site_name" :value="s.site_name">
              {{ s.site_name }}
              <CheckCircleFilled v-if="s.reviewed" style="color: #52c41a; margin-left: 4px" />
            </a-select-option>
          </a-select>
        </div>
      </div>

      <!-- ═══ Steps ═══ -->
      <div class="csp-steps">
        <a-steps :current="currentStep" size="small" style="padding: 8px 24px">
          <a-step title="编辑详情" />
          <a-step title="参数预览" />
          <a-step title="选择站点" />
          <a-step title="发布结果" />
        </a-steps>
      </div>

      <!-- ═══ Body ═══ -->
      <div ref="bodyRef" class="csp-body">
        <!-- Loading -->
        <div v-if="loading" class="csp-loading">
          <a-spin size="large" />
          <p style="margin-top: 16px; color: #666">{{ loadingText }}</p>
          <a-progress v-if="loadingProgress > 0" :percent="loadingProgress" status="active" style="max-width: 400px; margin-top: 12px" />
        </div>

        <!-- Error -->
        <a-result v-else-if="loadError" status="error" :title="loadError">
          <template #extra>
            <a-button @click="handleClose">关闭</a-button>
          </template>
        </a-result>

        <!-- Step 0: 编辑详情（5 Tab） -->
        <div v-else-if="currentStep === 0" class="csp-step-content">
          <a-alert v-if="forbiddenFlag" type="error" show-icon style="margin-bottom: 16px"
            :message="`⛔ 禁止转载：${forbiddenFlag}`" description="该种子被源站标记为禁止转载，无法继续发布。" />

          <a-tabs v-model:activeKey="activeTab">
            <!-- Tab 1: 种子详情 -->
            <a-tab-pane key="detail" tab="种子详情">
              <a-form layout="vertical" style="max-width: 800px">
                <a-row :gutter="16">
                  <a-col :span="12">
                    <a-form-item label="主标题（英文）">
                      <a-input v-model:value="form.title" placeholder="English.Title" />
                    </a-form-item>
                  </a-col>
                  <a-col :span="12">
                    <a-form-item label="中文副标题">
                      <a-input v-model:value="form.subtitle" placeholder="中文名" />
                      <div v-if="subtitleWarning" style="color: #faad14; font-size: 12px; margin-top: 4px">{{ subtitleWarning }}</div>
                    </a-form-item>
                  </a-col>
                </a-row>
                <a-row :gutter="16">
                  <a-col :span="6">
                    <a-form-item label="分辨率">
                      <a-input v-model:value="form.titleComponents.resolution" placeholder="1080p" />
                    </a-form-item>
                  </a-col>
                  <a-col :span="6">
                    <a-form-item label="视频编码">
                      <a-input v-model:value="form.titleComponents.video_codec" placeholder="x265" />
                    </a-form-item>
                  </a-col>
                  <a-col :span="6">
                    <a-form-item label="音频编码">
                      <a-input v-model:value="form.titleComponents.audio_codec" placeholder="AC3" />
                    </a-form-item>
                  </a-col>
                  <a-col :span="6">
                    <a-form-item label="媒介">
                      <a-input v-model:value="form.titleComponents.medium" placeholder="BluRay" />
                    </a-form-item>
                  </a-col>
                </a-row>
                <a-row :gutter="16">
                  <a-col :span="6">
                    <a-form-item label="制作组">
                      <a-input v-model:value="form.titleComponents.release_group" placeholder="-GROUP" />
                    </a-form-item>
                  </a-col>
                  <a-col :span="6">
                    <a-form-item label="HDR">
                      <a-input v-model:value="form.titleComponents.hdr" placeholder="HDR" />
                    </a-form-item>
                  </a-col>
                  <a-col :span="6">
                    <a-form-item label="年份/季集">
                      <a-input v-model:value="form.titleComponents.year" placeholder="2024 / S01E01" />
                    </a-form-item>
                  </a-col>
                  <a-col :span="6">
                    <a-form-item label="版本信息">
                      <a-input v-model:value="form.titleComponents.edition_info" placeholder="Remux" />
                    </a-form-item>
                  </a-col>
                </a-row>
                <a-form-item label="标签">
                  <TagSelector v-model="form.tags" />
                </a-form-item>
              </a-form>
            </a-tab-pane>

            <!-- Tab 2: 海报与声明 -->
            <a-tab-pane key="poster" tab="海报与声明">
              <a-form layout="vertical" style="max-width: 800px">
                <a-form-item label="海报 URL">
                  <div style="display: flex; gap: 8px; align-items: flex-start">
                    <a-input v-model:value="form.poster" placeholder="https://..." style="flex: 1" />
                    <a-button :loading="refreshing === 'poster'" @click="doRefresh('poster')">重新获取</a-button>
                  </div>
                  <a-image v-if="form.poster" :src="form.poster" :width="120" style="margin-top: 8px" />
                </a-form-item>
                <a-form-item label="声明">
                  <a-textarea v-model:value="form.statement" :rows="3" placeholder="免责声明等" />
                </a-form-item>
                <a-form-item label="豆瓣 / IMDb / TMDb">
                  <a-row :gutter="8">
                    <a-col :span="8"><a-input v-model:value="form.doubanLink" placeholder="豆瓣链接" /></a-col>
                    <a-col :span="8"><a-input v-model:value="form.imdbLink" placeholder="IMDb 链接" /></a-col>
                    <a-col :span="8"><a-input v-model:value="form.tmdbLink" placeholder="TMDb 链接" /></a-col>
                  </a-row>
                </a-form-item>
              </a-form>
            </a-tab-pane>

            <!-- Tab 3: 视频截图 -->
            <a-tab-pane key="screenshots" tab="视频截图">
              <div style="margin-bottom: 12px; display: flex; gap: 8px">
                <a-button :loading="refreshing === 'screenshots'" @click="doRefresh('screenshots')">重新获取截图（mpv）</a-button>
                <a-button :loading="refreshing === 'rehost_screenshots'" :disabled="form.screenshots.length === 0" @click="doRefresh('rehost_screenshots')">一键转存到图床</a-button>
              </div>
              <ScreenshotManager
                v-model:screenshots="form.screenshots"
                :screenshot-in-desc="form.screenshotInDesc"
                @update:screenshot-in-desc="form.screenshotInDesc = $event"
              />
            </a-tab-pane>

            <!-- Tab 4: 简介详情 -->
            <a-tab-pane key="intro" tab="简介详情">
              <div style="margin-bottom: 8px; display: flex; gap: 8px">
                <a-button :loading="refreshing === 'intro'" @click="doRefresh('intro')">重新获取简介（PTGen）</a-button>
              </div>
              <a-textarea v-model:value="form.description" :rows="20" placeholder="BBCode 简介正文" style="font-family: monospace" />
            </a-tab-pane>

            <!-- Tab 5: 媒体信息 -->
            <a-tab-pane key="mediainfo" tab="媒体信息">
              <div style="margin-bottom: 8px; display: flex; gap: 8px">
                <a-button :loading="refreshing === 'mediainfo'" @click="doRefresh('mediainfo')">重新获取 MediaInfo</a-button>
              </div>
              <a-textarea v-model:value="form.mediaInfo" :rows="20" placeholder="MediaInfo 文本" style="font-family: monospace; font-size: 12px" />
              <a-form-item v-if="form.bdinfo" label="BDInfo" style="margin-top: 12px">
                <a-textarea v-model:value="form.bdinfo" :rows="10" style="font-family: monospace; font-size: 12px" />
              </a-form-item>
            </a-tab-pane>
          </a-tabs>
        </div>

        <!-- Step 1: 参数预览 -->
        <div v-else-if="currentStep === 1" class="csp-step-content">
          <PublishFieldPreview
            :target-site="previewTargetSite"
            :mode="previewMode"
            :fields="previewFields"
            :completeness="previewCompleteness"
            :loading="previewLoading"
            error=""
          />
        </div>

        <!-- Step 2: 选择站点 -->
        <div v-else-if="currentStep === 2" class="csp-step-content">
          <WizardStepSelectTargets
            v-model="selectedTargets"
            :site-list="siteList"
            :targets-loading="targetsLoading"
            :anonymous="form.anonymous"
            :title-components="form.titleComponents"
            :info-hash="selectedTorrent?.info_hash || ''"
            :mode="analyzeResult?.last_merge_mode || 'ptgen_first'"
            @update:anonymous="form.anonymous = $event"
          />
        </div>

        <!-- Step 3: 发布结果 -->
        <div v-else-if="currentStep === 3" class="csp-step-content">
          <WizardStepResult
            :submit-error="submitError"
            :submitted-candidate-id="submittedCandidateId"
            :candidate-status="candidateStatus"
            :selected-targets="selectedTargets"
            :result-records="resultRecords"
            @back="handleClose"
          />
        </div>
      </div>

      <!-- ═══ Footer ═══ -->
      <div class="csp-footer">
        <div class="csp-footer-left">
          <a-button v-if="currentStep > 0" @click="prevStep">上一步</a-button>
        </div>
        <div class="csp-footer-right">
          <a-button @click="handleClose">取消</a-button>
          <a-button v-if="currentStep === 0" type="primary" :disabled="!canProceed" :loading="saving" @click="nextStep">
            保存并预览
          </a-button>
          <a-button v-else-if="currentStep === 1" type="primary" @click="nextStep">下一步</a-button>
          <a-button v-else-if="currentStep === 2" type="primary" :loading="submitting" :disabled="selectedTargets.length === 0" @click="doSubmit">
            立即发布（{{ selectedTargets.length }} 站）
          </a-button>
        </div>
      </div>
    </div>
  </a-drawer>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { message } from 'ant-design-vue'
import { CheckCircleFilled } from '@ant-design/icons-vue'
import { manualForwardApi, publishDataApi, publishApi } from '@/api/publish'
import type { ManualForwardSubmitRequest, PreviewField, PreviewCompleteness, PublishResultRecord } from '@/api/types'
import TagSelector from './TagSelector.vue'
import ScreenshotManager from './ScreenshotManager.vue'
import PublishFieldPreview from './PublishFieldPreview.vue'
import WizardStepSelectTargets from './WizardStepSelectTargets.vue'
import WizardStepResult from './WizardStepResult.vue'

interface PresetTorrent {
  info_hash: string
  name: string
  size: number
  save_path: string
  client_id: number
  state?: string
  source_site?: string
  source_site_id?: number
}

const props = defineProps<{
  open: boolean
  presetTorrent?: PresetTorrent | null
}>()

const emit = defineEmits<{
  (e: 'update:open', val: boolean): void
  (e: 'success'): void
}>()

// --- State ---
const currentStep = ref(0)
const activeTab = ref('detail')
const loading = ref(false)
const loadingText = ref('')
const loadingProgress = ref(0)
const loadError = ref('')
const saving = ref(false)
const submitting = ref(false)
const refreshing = ref('')
const bodyRef = ref<HTMLElement>()

const selectedTorrent = ref<PresetTorrent | null>(null)
const analyzeResult = ref<Record<string, any> | null>(null)
const cachedSites = ref<Array<{ id: number; site_name: string; torrent_id: string; reviewed: boolean; fetched_at: string; title: string; subtitle: string }>>([])
const currentSourceSite = ref<string>('')

const form = ref({
  title: '',
  subtitle: '',
  mediaInfo: '',
  description: '',
  screenshots: [] as string[],
  statement: '',
  poster: '',
  doubanLink: '',
  imdbLink: '',
  tmdbLink: '',
  tags: [] as string[],
  removedDeclarations: [] as string[],
  bdinfo: '',
  anonymous: false,
  screenshotInDesc: false,
  titleComponents: {} as Record<string, string>,
})

// Preview
const previewTargetSite = ref('')
const previewMode = ref('ptgen_first')
const previewFields = ref<PreviewField[]>([])
const previewCompleteness = ref<PreviewCompleteness | null>(null)
const previewLoading = ref(false)

// Target sites
interface SiteItem { name: string; domain: string; blocked: boolean; blockReason: string }
const siteList = ref<SiteItem[]>([])
const selectedTargets = ref<string[]>([])
const targetsLoading = ref(false)

// Submit results
interface CandidateStatus { status: string; total_count: number; done_count: number; fail_count: number }
const submitError = ref('')
const submittedCandidateId = ref(0)
const candidateStatus = ref<CandidateStatus | null>(null)
const resultRecords = ref<Record<string, PublishResultRecord>>({})
let pollTimer: ReturnType<typeof setTimeout> | null = null
let candidatePollTimer: ReturnType<typeof setInterval> | null = null

// --- Computed ---
const forbiddenFlag = computed(() => analyzeResult.value?.forbidden || '')
const canProceed = computed(() => !!selectedTorrent.value && !forbiddenFlag.value)

const subtitleWarning = computed(() => {
  if (!form.value.subtitle) return ''
  const sub = form.value.subtitle
  const warnings: string[] = []
  if (/[\uFF00-\uFFEF]/.test(sub)) warnings.push('含全角符号')
  const firstChar = sub.charAt(0)
  if (!/[\u4e00-\u9fff]/.test(firstChar)) warnings.push('未以中文开头')
  if (/转自|转载自|来源[:：]/.test(sub)) warnings.push('包含转载来源')
  return warnings.join('； ')
})

// --- Lifecycle ---
watch(() => props.open, (val) => {
  if (val) {
    resetPanel()
    if (props.presetTorrent) {
      selectedTorrent.value = props.presetTorrent
      enterAnalyze()
    }
  }
})

watch(currentStep, () => {
  if (bodyRef.value) bodyRef.value.scrollTop = 0
})

function resetPanel() {
  stopCandidatePoll()
  currentStep.value = 0
  activeTab.value = 'detail'
  selectedTorrent.value = null
  analyzeResult.value = null
  cachedSites.value = []
  currentSourceSite.value = ''
  siteList.value = []
  selectedTargets.value = []
  submittedCandidateId.value = 0
  candidateStatus.value = null
  resultRecords.value = {}
  previewFields.value = []
  previewCompleteness.value = null
  previewLoading.value = false
  submitError.value = ''
  form.value = {
    title: '', subtitle: '', mediaInfo: '', description: '', screenshots: [],
    statement: '', poster: '', doubanLink: '', imdbLink: '', tmdbLink: '',
    tags: [], removedDeclarations: [], bdinfo: '', anonymous: false, screenshotInDesc: false,
    titleComponents: {},
  }
}

function handleClose() {
  emit('update:open', false)
}

// --- Analyze ---
async function enterAnalyze() {
  if (!selectedTorrent.value) return
  loading.value = true
  loadingText.value = '正在分析种子信息...'
  loadingProgress.value = 0
  loadError.value = ''

  try {
    // 先查缓存站点
    const csResp = await publishDataApi.cachedSites(selectedTorrent.value.info_hash)
    cachedSites.value = csResp.data?.data?.sites || []
    if (cachedSites.value.length > 0 && !currentSourceSite.value) {
      currentSourceSite.value = cachedSites.value[0].site_name
    }

    const t = selectedTorrent.value
    const resp = await manualForwardApi.startAnalyze({
      client_id: t.client_id,
      info_hash: t.info_hash,
      name: t.name,
      save_path: t.save_path,
      size: t.size,
      source_site: t.source_site || currentSourceSite.value,
      source_torrent_id: t.source_site_id ? String(t.source_site_id) : undefined,
    })
    const taskId = resp.data?.data?.taskId
    if (!taskId) {
      loadError.value = '分析任务创建失败'
      loading.value = false
      return
    }
    pollAnalyze(taskId)
  } catch (e: unknown) {
    loadError.value = (e as Error).message
    loading.value = false
  }
}

function pollAnalyze(taskId: number) {
  async function poll() {
    try {
      const resp = await manualForwardApi.pollAnalyze(taskId)
      const task = resp.data?.data as Record<string, any> | undefined
      if (!task) return
      if (task.status === 'done' && task.result) {
        const r = task.result as Record<string, any>
        analyzeResult.value = r
        fillForm(r)
        loading.value = false
        loadingProgress.value = 0
      } else if (task.status === 'failed') {
        loadError.value = task.error || '分析失败'
        loading.value = false
      } else {
        loadingProgress.value = task.progress || 0
        loadingText.value = task.progress_text || '正在分析...'
        pollTimer = setTimeout(poll, 2000)
      }
    } catch (e: unknown) {
      loadError.value = (e as Error).message
      loading.value = false
    }
  }
  pollTimer = setTimeout(poll, 1500)
}

function fillForm(r: Record<string, any>) {
  form.value.title = r.title || ''
  form.value.subtitle = r.subtitle || ''
  form.value.mediaInfo = r.media_info || ''
  form.value.description = r.description || ''
  form.value.screenshots = r.screenshots || []
  form.value.poster = r.poster_url || r.poster || ''
  form.value.statement = r.statement || ''
  form.value.doubanLink = r.douban_link || ''
  form.value.imdbLink = r.imdb_link || ''
  form.value.tmdbLink = r.tmdb_link || ''
  form.value.tags = r.tags || []
  form.value.bdinfo = r.bdinfo || ''
  form.value.titleComponents = r.title_components || {}
  if (r.removed_declarations) {
    form.value.removedDeclarations = r.removed_declarations
  }
}

// --- Source site switch ---
async function onSourceSiteChange() {
  if (!selectedTorrent.value || !currentSourceSite.value) return
  // Re-analyze with new source site
  selectedTorrent.value.source_site = currentSourceSite.value
  await enterAnalyze()
}

// --- Refresh ---
async function doRefresh(type: string) {
  if (!selectedTorrent.value) return
  refreshing.value = type
  try {
    const payload: { type: string; name: string; save_path?: string; info_hash?: string; site_name?: string; screenshots?: string[] } = {
      type,
      name: selectedTorrent.value.name,
      save_path: selectedTorrent.value.save_path,
      info_hash: selectedTorrent.value.info_hash,
      site_name: currentSourceSite.value || selectedTorrent.value.source_site || '',
    }
    if (type === 'rehost_screenshots') {
      payload.screenshots = form.value.screenshots
    }
    const resp = await manualForwardApi.refresh(payload)
    const data = (resp.data?.data || {}) as Record<string, unknown>
    if (type === 'poster') {
      if (data.poster) form.value.poster = data.poster as string
      if (data.douban_link) form.value.doubanLink = data.douban_link as string
      if (data.imdb_link) form.value.imdbLink = data.imdb_link as string
      if (data.tmdb_link) form.value.tmdbLink = data.tmdb_link as string
    } else if (type === 'intro') {
      if (data.description) form.value.description = data.description as string
      if (data.subtitle) form.value.subtitle = data.subtitle as string
    } else if (type === 'mediainfo') {
      if (data.mediainfo) form.value.mediaInfo = data.mediainfo as string
    } else if (type === 'screenshots') {
      if (data.screenshots) form.value.screenshots = data.screenshots as string[]
    } else if (type === 'rehost_screenshots') {
      if (data.screenshots) form.value.screenshots = data.screenshots as string[]
    }
    message.success(`${type} 刷新成功`)
  } catch (e: unknown) {
    message.error(`刷新失败: ${(e as Error).message}`)
  } finally {
    refreshing.value = ''
  }
}

// --- Step navigation ---
async function nextStep() {
  if (currentStep.value === 0) {
    // Save to DB before proceeding
    await saveToDB()
    // Load preview
    await loadPreview()
    currentStep.value = 1
  } else if (currentStep.value === 1) {
    await enterSelectSites()
    currentStep.value = 2
  }
}

function prevStep() {
  if (currentStep.value > 0) currentStep.value--
}

async function saveToDB() {
  if (!selectedTorrent.value) return
  // 直接从 cachedSites 找到 metadata ID，避免脆弱的搜索
  const site = cachedSites.value.find(s => s.site_name === (currentSourceSite.value || selectedTorrent.value?.source_site))
  if (!site || !site.id) return
  saving.value = true
  try {
    await publishDataApi.saveSeedData(site.id, {
      title: form.value.title,
      subtitle: form.value.subtitle,
      description: form.value.description,
      screenshots: JSON.stringify(form.value.screenshots),
      poster: form.value.poster,
      mediainfo: form.value.mediaInfo,
      tags: JSON.stringify(form.value.tags),
    })
  } catch { /* silent */ } finally {
    saving.value = false
  }
}

async function loadPreview() {
  if (!selectedTorrent.value) return
  previewLoading.value = true
  try {
    const resp = await manualForwardApi.previewFields({
      info_hash: selectedTorrent.value.info_hash,
      target_site: '',
      mode: analyzeResult.value?.last_merge_mode || 'ptgen_first',
    })
    const data = resp.data?.data as unknown as Record<string, unknown> | undefined
    if (data) {
      previewFields.value = (data.fields as PreviewField[]) || []
      previewCompleteness.value = (data.completeness as PreviewCompleteness) || null
      previewMode.value = (data.mode as string) || 'ptgen_first'
    }
  } catch { /* silent */ } finally {
    previewLoading.value = false
  }
}

async function enterSelectSites() {
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
        blockReason = blockedTargets.includes(name) ? '互斥规则' : '缺少 cookie/passkey'
      }
      return { name, domain: (obj.domain as string) || '', blocked, blockReason }
    })
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    targetsLoading.value = false
  }
}

// --- Submit ---
async function doSubmit() {
  if (!selectedTorrent.value || selectedTargets.value.length === 0) return
  submitting.value = true
  try {
    const t = selectedTorrent.value
    const req: ManualForwardSubmitRequest = {
      client_id: t.client_id,
      info_hash: t.info_hash,
      title: t.name,
      source_site: analyzeResult.value?.source_site || t.source_site || '',
      source_site_id: t.source_site_id || 0,
      description: form.value.description,
      media_info: form.value.mediaInfo,
      screenshots: form.value.screenshots,
      target_sites: selectedTargets.value,
      subtitle: form.value.subtitle,
      statement: form.value.statement,
      poster: form.value.poster,
      douban_link: form.value.doubanLink,
      imdb_link: form.value.imdbLink,
      tmdb_link: form.value.tmdbLink,
      tags: form.value.tags,
      bdinfo: form.value.bdinfo,
      anonymous: form.value.anonymous,
      screenshot_in_desc: form.value.screenshotInDesc,
      title_components: form.value.titleComponents,
    }
    const resp = await manualForwardApi.submit(req)
    const candId = (resp.data?.data as unknown as Record<string, unknown>)?.candidate_id as number
    if (candId) {
      submittedCandidateId.value = candId
      currentStep.value = 3
      startCandidatePoll(candId)
    }
  } catch (e: unknown) {
    message.error(`发布失败: ${(e as Error).message}`)
  } finally {
    submitting.value = false
  }
}

function startCandidatePoll(candidateId: number) {
  candidatePollTimer = setInterval(async () => {
    try {
      const resp = await publishApi.getCandidate(candidateId)
      const c = resp.data?.data
      if (!c) return
      const status = c.publish_status as string
      candidateStatus.value = {
        status,
        total_count: selectedTargets.value.length,
        done_count: status === 'done' ? selectedTargets.value.length : 0,
        fail_count: status === 'failed' ? 1 : 0,
      }
      if (status === 'done' || status === 'failed') {
        stopCandidatePoll()
        await fetchResultRecords()
      }
    } catch { /* silent */ }
  }, 3000)
}

async function fetchResultRecords() {
  if (!submittedCandidateId.value) return
  try {
    const resp = await publishApi.listResults({ page: 1, pageSize: 100, candidateId: submittedCandidateId.value })
    const items = (resp.data?.data?.items || []) as PublishResultRecord[]
    const map: Record<string, PublishResultRecord> = {}
    for (const r of items) map[r.target_site] = r
    resultRecords.value = map
  } catch { /* silent */ }
}

function stopCandidatePoll() {
  if (candidatePollTimer) {
    clearInterval(candidatePollTimer)
    candidatePollTimer = null
  }
}
</script>

<style scoped>
.csp-shell {
  display: flex;
  flex-direction: column;
  height: 100vh;
}
.csp-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 24px;
  border-bottom: 1px solid #f0f0f0;
}
.csp-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}
.csp-header-title {
  font-size: 16px;
  font-weight: 600;
}
.csp-steps {
  border-bottom: 1px solid #f0f0f0;
}
.csp-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 24px;
}
.csp-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80px 0;
}
.csp-step-content {
  max-width: 1200px;
}
.csp-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 24px;
  border-top: 1px solid #f0f0f0;
  background: #fff;
}
.csp-footer-right {
  display: flex;
  gap: 8px;
}
</style>
