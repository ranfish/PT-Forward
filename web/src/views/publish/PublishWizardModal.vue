<template>
  <a-modal
    :open="open"
    :title="null"
    width="1150px"
    :body-style="{ padding: '0' }"
    :footer="null"
    destroy-on-close
    @cancel="handleCancel"
  >
    <div class="wizard-shell">
      <!-- ═══ Header: 标题 + 步骤条 ═══ -->
      <div class="wizard-header">
        <div class="wizard-header-bar">
          <span class="wizard-header-title">手动转发</span>
          <span v-if="selectedTorrent" class="wizard-header-subtitle">
            {{ selectedTorrent.name }}
          </span>
        </div>
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
      </div>

      <!-- ═══ Body: 可滚动内容区 ═══ -->
      <div ref="bodyRef" class="wizard-body">
        <!-- ─── Step 0: 选择种子 ─── -->
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
              style="width: 300px; margin-left: 12px"
              allow-clear
            />
            <a-tag v-if="seededTorrents.length" color="blue" style="margin-left: 8px">
              共 {{ filteredTorrents.length }} 个种子
            </a-tag>
          </div>
          <a-table
            :columns="torrentColumns"
            :data-source="filteredTorrents"
            :loading="torrentsLoading"
            :pagination="{ pageSize: 8, showSizeChanger: false, size: 'small' }"
            row-key="info_hash"
            size="small"
            :row-selection="{ type: 'radio', selectedRowKeys: selectedTorrent ? [selectedTorrent.info_hash] : [], onSelect: (r: unknown) => { selectedTorrent = r as SeededTorrent } }"
            :scroll="{ y: 340 }"
          >
            <template #bodyCell="{ column, record }">
              <template v-if="column.key === 'size'">
                {{ formatBytes(record.size) }}
              </template>
              <template v-if="column.key === 'state'">
                <a-tag :color="qbStateColor(record.state)" style="margin: 0">
                  {{ translateQbState(record.state) }}
                </a-tag>
              </template>
            </template>
          </a-table>
          <a-empty
            v-if="!torrentsLoading && !seededTorrents.length && selectedClientId"
            description="该下载器没有做种种子"
            style="padding: 40px 0"
          />
        </div>

        <!-- ─── Step 1: 核对详情 ─── -->
        <div v-else-if="currentStep === 1" class="step-content">
          <!-- 分析中 loading -->
          <div v-if="analyzing" class="analyzing-state">
            <a-spin size="large" />
            <p style="margin-top: 16px; color: #666">正在分析种子信息（截图/MediaInfo/简介）...</p>
            <p style="color: #999; font-size: 12px">这可能需要 30-60 秒，请耐心等待</p>
          </div>

          <!-- 分析失败 -->
          <a-result
            v-if="!analyzing && analyzeError"
            status="error"
            :title="analyzeError"
            style="padding: 40px 0"
          >
            <template #extra>
              <a-button @click="currentStep = 0">返回重新选择</a-button>
            </template>
          </a-result>

          <!-- 分析成功：核对详情 -->
          <div v-if="!analyzing && analyzeResult" class="review-section">
            <!-- 禁转警告 -->
            <a-alert
              v-if="analyzeResult.forbidden"
              type="error"
              show-icon
              message="该种子禁止转载"
              :description="analyzeResult.forbid_reason as string"
              style="margin-bottom: 16px"
            />

            <!-- 信息摘要卡片 -->
            <div class="info-strip">
              <div class="info-strip-item">
                <span class="info-strip-label">源站</span>
                <a-tag color="blue">{{ analyzeResult.source_site || '-' }}</a-tag>
              </div>
              <div class="info-strip-item">
                <span class="info-strip-label">大小</span>
                <span>{{ formatBytes(selectedTorrent?.size ?? 0) }}</span>
              </div>
              <div class="info-strip-item">
                <span class="info-strip-label">下载器</span>
                <span>{{ getClientName(selectedTorrent?.client_id) }}</span>
              </div>
              <div class="info-strip-item">
                <span class="info-strip-label">Hash</span>
                <span class="mono-text">{{ shortHash(selectedTorrent?.info_hash) }}</span>
              </div>
            </div>

            <a-tabs v-model:active-key="reviewTab" type="card" class="review-tabs">
              <!-- Tab: 主要信息 -->
              <a-tab-pane key="main" tab="主要信息">
                <a-form layout="vertical" class="review-form">
                  <a-form-item label="主标题">
                    <a-input v-model:value="form.title" size="large" />
                  </a-form-item>
                  <a-form-item label="副标题">
                    <a-input v-model:value="form.subtitle" placeholder="中文标题或简介" />
                    <div v-if="subtitleWarning" style="margin-top: 4px">
                      <a-tag color="orange" size="small">{{ subtitleWarning }}</a-tag>
                    </div>
                  </a-form-item>
                  <a-form-item label="标签">
                    <a-select
                      v-model:value="form.tags"
                      mode="tags"
                      style="width: 100%"
                      placeholder="添加标签（禁转/中字/国语/原盘等）"
                      :token-separators="[',']"
                    />
                  </a-form-item>

                  <!-- 标题组件（P2 标题标准化） -->
                  <template v-if="titleComponents">
                    <a-divider style="margin: 8px 0">标题组件</a-divider>
                    <a-row :gutter="12">
                      <a-col :span="6">
                        <a-form-item label="年份">
                          <a-input v-model:value="titleComponents.year" size="small" />
                        </a-form-item>
                      </a-col>
                      <a-col :span="6">
                        <a-form-item label="季集">
                          <a-input v-model:value="titleComponents.season_episode" size="small" placeholder="S01E02" />
                        </a-form-item>
                      </a-col>
                      <a-col :span="6">
                        <a-form-item label="分辨率">
                          <a-input v-model:value="titleComponents.resolution" size="small" placeholder="1080p" />
                        </a-form-item>
                      </a-col>
                      <a-col :span="6">
                        <a-form-item label="媒介">
                          <a-input v-model:value="titleComponents.medium" size="small" placeholder="BluRay" />
                        </a-form-item>
                      </a-col>
                    </a-row>
                    <a-row :gutter="12">
                      <a-col :span="6">
                        <a-form-item label="视频编码">
                          <a-input v-model:value="titleComponents.video_codec" size="small" />
                        </a-form-item>
                      </a-col>
                      <a-col :span="6">
                        <a-form-item label="音频编码">
                          <a-input v-model:value="titleComponents.audio_codec" size="small" />
                        </a-form-item>
                      </a-col>
                      <a-col :span="6">
                        <a-form-item label="HDR">
                          <a-input v-model:value="titleComponents.hdr_format" size="small" placeholder="DoVi/HDR10+" />
                        </a-form-item>
                      </a-col>
                      <a-col :span="6">
                        <a-form-item label="制作组">
                          <a-input v-model:value="titleComponents.release_group" size="small" />
                        </a-form-item>
                      </a-col>
                    </a-row>
                    <a-row :gutter="12">
                      <a-col :span="6">
                        <a-form-item label="片源平台">
                          <a-input v-model:value="titleComponents.source_platform" size="small" placeholder="NF/AMZN" />
                        </a-form-item>
                      </a-col>
                      <a-col :span="6">
                        <a-form-item label="色深">
                          <a-input v-model:value="titleComponents.bit_depth" size="small" placeholder="10bit" />
                        </a-form-item>
                      </a-col>
                      <a-col :span="6">
                        <a-form-item label="发布版本">
                          <a-input v-model:value="titleComponents.release_version" size="small" placeholder="PROPER" />
                        </a-form-item>
                      </a-col>
                      <a-col :span="6">
                        <a-form-item label="标准分类">
                          <a-tag v-if="standardizedParams?.type" color="blue">{{ standardizedParams.type }}</a-tag>
                          <span v-else style="color:#999">未推断</span>
                        </a-form-item>
                      </a-col>
                    </a-row>
                    <a-alert
                      v-if="titleComponents.unrecognized"
                      type="warning"
                      show-icon
                      :message="`标题中有未识别内容: ${titleComponents.unrecognized}`"
                      style="margin-top: 4px"
                    />
                  </template>
                </a-form>
              </a-tab-pane>

              <!-- Tab: 海报与声明 -->
              <a-tab-pane key="poster" :tab="form.poster ? '海报与声明 ✓' : '海报与声明'">
                <a-row :gutter="16">
                  <a-col :span="14">
                    <a-form layout="vertical">
                      <a-form-item label="声明（官组声明/BBCode）">
                        <a-textarea
                          v-model:value="form.statement"
                          :rows="8"
                          placeholder="如 [quote][color=blue]官组作品[/color][/quote]"
                        />
                      </a-form-item>
                      <a-form-item label="海报链接">
                        <a-input v-model:value="form.poster" placeholder="海报图片 URL" />
                      </a-form-item>
                    </a-form>
                  </a-col>
                  <a-col :span="10">
                    <div class="poster-preview-box">
                      <a-image
                        v-if="form.poster"
                        :src="form.poster"
                        :width="180"
                        :height="270"
                        class="poster-preview-img"
                      />
                      <div v-else class="poster-empty">
                        <InboxOutlined style="font-size: 32px; color: #d9d9d9" />
                        <p>暂无海报</p>
                      </div>
                    </div>
                  </a-col>
                </a-row>
              </a-tab-pane>

              <!-- Tab: 截图 -->
              <a-tab-pane key="screenshots" :tab="`截图 (${form.screenshots.length})`">
                <div v-if="form.screenshots.length === 0" class="empty-hint">
                  <InboxOutlined style="font-size: 32px; color: #d9d9d9" />
                  <p>暂无截图</p>
                </div>
                <div v-else class="screenshot-section">
                  <div class="screenshot-grid">
                    <div
                      v-for="(url, i) in form.screenshots"
                      :key="i"
                      class="screenshot-item"
                    >
                      <a-image
                        :src="url"
                        :width="200"
                        :height="113"
                        class="screenshot-img"
                        :preview="{ visible: screenshotPreviewVisible, onVisibleChange: (v: boolean) => screenshotPreviewVisible = v }"
                      />
                      <div class="screenshot-actions">
                        <span class="screenshot-index">#{{ i + 1 }}</span>
                        <a-button
                          type="text"
                          danger
                          size="small"
                          @click="form.screenshots.splice(i, 1)"
                        >
                          <DeleteOutlined />
                        </a-button>
                      </div>
                    </div>
                  </div>
                </div>
              </a-tab-pane>

              <!-- Tab: 简介 -->
              <a-tab-pane key="intro" :tab="form.description ? '简介 ✓' : '简介'">
                <a-form layout="vertical">
                  <a-form-item label="简介正文（BBCode）">
                    <a-textarea
                      v-model:value="form.description"
                      :rows="16"
                      placeholder="资源简介（PTGen 生成的 BBCode）"
                    />
                  </a-form-item>
                  <a-row :gutter="16">
                    <a-col :span="8">
                      <a-form-item label="豆瓣链接">
                        <a-input v-model:value="form.doubanLink" placeholder="https://movie.douban.com/..." />
                      </a-form-item>
                    </a-col>
                    <a-col :span="8">
                      <a-form-item label="IMDb 链接">
                        <a-input v-model:value="form.imdbLink" placeholder="https://www.imdb.com/title/..." />
                      </a-form-item>
                    </a-col>
                    <a-col :span="8">
                      <a-form-item label="TMDb 链接">
                        <a-input v-model:value="form.tmdbLink" placeholder="https://www.themoviedb.org/..." />
                      </a-form-item>
                    </a-col>
                  </a-row>
                </a-form>
              </a-tab-pane>

              <!-- Tab: MediaInfo -->
              <a-tab-pane key="mediainfo" :tab="form.mediaInfo ? 'MediaInfo ✓' : 'MediaInfo'">
                <a-textarea
                  v-model:value="form.mediaInfo"
                  :rows="14"
                  class="mono-font"
                  placeholder="MediaInfo"
                />
                <template v-if="form.bdinfo">
                  <a-divider style="margin: 8px 0">BDInfo</a-divider>
                  <a-textarea
                    v-model:value="form.bdinfo"
                    :rows="10"
                    class="mono-font"
                    placeholder="BDInfo"
                  />
                </template>
              </a-tab-pane>

              <!-- Tab: 已过滤声明 -->
              <a-tab-pane key="filtered" :tab="form.removedDeclarations.length ? `已过滤 (${form.removedDeclarations.length})` : '已过滤声明'">
                <div v-if="form.removedDeclarations.length === 0" class="empty-hint">
                  <CheckCircleFilled style="font-size: 32px; color: #52c41a" />
                  <p>未检测到需要过滤的声明内容</p>
                </div>
                <div v-else>
                  <a-alert
                    type="info"
                    show-icon
                    message="以下声明内容已从简介中自动移除"
                    description="点击「恢复」可将该声明放回简介；点击「删除」可永久移除。"
                    style="margin-bottom: 16px"
                  />
                  <div
                    v-for="(decl, i) in form.removedDeclarations"
                    :key="i"
                    class="decl-item"
                  >
                    <div class="decl-header">
                      <a-tag color="red">已过滤 #{{ i + 1 }}</a-tag>
                      <a-button type="link" size="small" @click="restoreDecl(i)">恢复到简介</a-button>
                      <a-button type="link" danger size="small" @click="form.removedDeclarations.splice(i, 1)">删除</a-button>
                    </div>
                    <pre class="decl-content">{{ decl }}</pre>
                  </div>
                </div>
              </a-tab-pane>
            </a-tabs>
          </div>
        </div>

        <!-- ─── Step 2: 选择站点 ─── -->
        <div v-else-if="currentStep === 2" class="step-content">
          <a-spin :spinning="targetsLoading">
            <!-- 统计卡片 -->
            <div class="site-stats">
              <div class="site-stat-card stat-available">
                <div class="site-stat-num">{{ availableCount }}</div>
                <div class="site-stat-label">可发布</div>
              </div>
              <div class="site-stat-card stat-selected">
                <div class="site-stat-num">{{ selectedTargets.length }}</div>
                <div class="site-stat-label">已选择</div>
              </div>
              <div class="site-stat-card stat-blocked">
                <div class="site-stat-num">{{ blockedCount }}</div>
                <div class="site-stat-label">不可用</div>
              </div>
            </div>

            <!-- 操作栏 -->
            <div class="site-toolbar">
              <a-button size="small" type="primary" ghost @click="selectAllAvailable">
                全选可用
              </a-button>
              <a-button size="small" @click="selectedTargets = []">清空</a-button>
            </div>

            <!-- 站点网格 -->
            <div class="site-grid">
              <a-tooltip
                v-for="site in siteList"
                :key="site.name"
                :title="site.blocked ? site.blockReason : ''"
                :disabled="!site.blocked"
                placement="top"
              >
                <div
                  class="site-btn"
                  :class="{
                    selected: selectedTargets.includes(site.name),
                    blocked: site.blocked,
                  }"
                  @click="toggleSite(site)"
                >
                  <CheckCircleFilled
                    v-if="selectedTargets.includes(site.name)"
                    class="site-btn-check"
                  />
                  <span class="site-btn-name">{{ site.name }}</span>
                  <StopOutlined
                    v-if="site.blocked"
                    class="site-btn-blocked-icon"
                  />
                </div>
              </a-tooltip>
            </div>
          </a-spin>

          <!-- 标题预览 -->
          <div v-if="selectedTargets.length > 0 && titleComponents" class="title-preview-section">
            <a-divider style="margin: 12px 0">标题预览</a-divider>
            <div v-for="site in selectedTargets" :key="site" class="title-preview-item">
              <span class="title-preview-site">{{ site }}</span>
                <span class="title-preview-text" :title="titlePreviews[site] || '加载中...'">
                  {{ titlePreviews[site] || '加载中...' }}
                </span>
              </div>
            </div>
        </div>

        <!-- ─── Step 3: 发布结果 ─── -->
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

          <div v-else class="results-section">
            <!-- 成功头部 -->
            <div class="results-header">
              <CheckCircleFilled class="results-header-icon" />
              <div>
                <div class="results-header-title">
                  已创建发布候选 #{{ submittedCandidateId }}
                </div>
                <div class="results-header-subtitle">
                  候选已进入发布队列，系统将自动处理
                </div>
              </div>
            </div>

            <!-- 发布进度 -->
            <div v-if="candidateStatus" class="progress-section">
              <div class="progress-row">
                <span class="progress-label">发布进度</span>
                <a-progress
                  :percent="publishPercent"
                  :stroke-color="publishPercent === 100 ? '#52c41a' : '#1677ff'"
                  size="small"
                  style="flex: 1; margin: 0 12px"
                />
                <span class="progress-count">
                  {{ candidateStatus.done_count || 0 }} / {{ candidateStatus.total_count || selectedTargets.length }}
                </span>
              </div>
            </div>

            <!-- 目标站点状态卡片 -->
            <div class="result-site-grid">
              <div
                v-for="site in resultSiteStatus"
                :key="site.name"
                class="result-site-card"
                :class="site.status"
              >
                <div class="result-site-card-bar" :class="site.status" />
                <component
                  :is="site.icon"
                  class="result-site-card-icon"
                  :spin="site.status === 'publishing'"
                />
                <div class="result-site-card-name">{{ site.name }}</div>
                <a-tag :color="site.tagColor" size="small" style="margin: 0">
                  {{ site.label }}
                </a-tag>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- ═══ Footer: 固定按钮栏 ═══ -->
      <div class="wizard-footer">
        <div class="wizard-footer-left">
          <!-- Step 1: 校验提示 -->
          <span v-if="currentStep === 1 && !analyzing && analyzeResult?.forbidden" class="footer-warn">
            <WarningFilled /> 该种子禁止转载，无法继续
          </span>
          <!-- Step 2: 已选数量 -->
          <span v-if="currentStep === 2 && selectedTargets.length > 0" class="footer-info">
            已选 {{ selectedTargets.length }} 个目标站点
          </span>
        </div>
        <div class="wizard-footer-right">
          <template v-if="currentStep === 0">
            <a-button @click="handleCancel">取消</a-button>
            <a-button type="primary" :disabled="!selectedTorrent" @click="enterAnalyze">
              下一步：核对详情
            </a-button>
          </template>
          <template v-else-if="currentStep === 1">
            <a-button :disabled="analyzing" @click="currentStep = 0">上一步</a-button>
            <a-button
              type="primary"
              :disabled="analyzing || !!analyzeError || (analyzeResult?.forbidden ?? false)"
              @click="enterSelectSites"
            >
              下一步：选择站点
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
              立即发布到 {{ selectedTargets.length }} 个站点
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
import { ref, computed, watch, onBeforeUnmount, onUnmounted, markRaw } from 'vue'
import { message } from 'ant-design-vue'
import {
  CheckOutlined, CheckCircleFilled, ClockCircleOutlined,
  WarningFilled, DeleteOutlined, StopOutlined, InboxOutlined,
  LoadingOutlined, CloseCircleFilled,
} from '@ant-design/icons-vue'
import { manualForwardApi, publishApi, publishTorrentsApi } from '@/api/publish'
import { downloadersApi } from '@/api/downloaders'
import { useEnumLabels } from '@/utils/enumLabels'
import { formatBytes } from '@/utils/format'

const { translateQbState } = useEnumLabels()

const props = defineProps<{
  open: boolean
  presetTorrent?: {
    info_hash: string
    name: string
    size: number
    save_path: string
    client_id: number
    state: string
    source_site?: string
    source_site_id?: number
    torrent_id?: string
  } | null
  presetClientId?: number
}>()
const emit = defineEmits<{
  'update:open': [value: boolean]
  success: []
}>()

const stepLabels = ['选择种子', '核对详情', '选择站点', '完成']
const currentStep = ref(0)
const bodyRef = ref<HTMLElement>()

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

function getClientName(id?: number): string {
  if (!id) return '-'
  const c = clients.value.find(c => c.id === id)
  return c ? c.name : String(id)
}

function shortHash(hash?: string): string {
  if (!hash) return '-'
  return hash.length > 12 ? hash.slice(0, 8) + '…' : hash
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
  poster_url?: string
  douban_link?: string
  imdb_link?: string
  tmdb_link?: string
  removed_declarations?: string[]
  title_components?: Record<string, string>
  standardized_params?: Record<string, string>
  bdinfo?: string
}

const analyzing = ref(false)
const analyzeResult = ref<AnalyzeResult | null>(null)
const analyzeError = ref('')
const reviewTab = ref('main')
const screenshotPreviewVisible = ref(false)

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
})

const titleComponents = ref<Record<string, string> | null>(null)
const standardizedParams = ref<Record<string, string> | null>(null)
const titlePreviews = ref<Record<string, string>>({})

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
  selectedTorrent.value = null
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
          form.value.poster = r.poster_url || ''
          form.value.doubanLink = r.douban_link || ''
          form.value.imdbLink = r.imdb_link || ''
          form.value.tmdbLink = r.tmdb_link || ''
          form.value.removedDeclarations = (r as Record<string, unknown>).removed_declarations as string[] || []
          form.value.bdinfo = (r as Record<string, unknown>).bdinfo as string || ''
          titleComponents.value = (r as Record<string, unknown>).title_components as Record<string, string> || null
          standardizedParams.value = (r as Record<string, unknown>).standardized_params as Record<string, string> || null
          // 如果有预设源站（从源头站检测器），覆盖后端自动检测的源站
          if (props.presetTorrent?.source_site) {
            r.source_site = props.presetTorrent.source_site
            r.source_site_id = props.presetTorrent.source_site_id
          }
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

const availableCount = computed(() => siteList.value.filter(s => !s.blocked).length)
const blockedCount = computed(() => siteList.value.filter(s => s.blocked).length)

// 副标题校验提示
const subtitleWarning = computed(() => {
  if (!form.value.subtitle) return ''
  const sub = form.value.subtitle
  // 禁止全角标点
  if (/[\uFF00-\uFFEF]/.test(sub)) return '副标题含全角符号，部分站点可能拒绝'
  // 家园/红叶：禁止重复英文片名
  if (form.value.title && sub.includes(form.value.title.split(' ')[0])) {
    return '副标题包含主标题英文名，家园/红叶可能拒绝'
  }
  return ''
})

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
    delete titlePreviews.value[site.name]
  } else {
    selectedTargets.value.push(site.name)
    if (titleComponents.value) {
      loadTitlePreview(site.name)
    }
  }
}

async function loadTitlePreview(siteName: string) {
  if (!titleComponents.value) return
  try {
    const resp = await publishTorrentsApi.previewTitle({
      target_site: siteName,
      title_components: titleComponents.value,
    })
    if (resp.data?.data?.title) {
      titlePreviews.value[siteName] = resp.data.data.title
    }
  } catch { /* silent */ }
}

function selectAllAvailable() {
  selectedTargets.value = siteList.value.filter(s => !s.blocked).map(s => s.name)
}

// --- Step 3: Submit & Results ---
const submitting = ref(false)
const submitError = ref('')
const submittedCandidateId = ref(0)
const candidateStatus = ref<{ done_count?: number; fail_count?: number; total_count?: number; publish_status?: string } | null>(null)
let candidatePollTimer: ReturnType<typeof setInterval> | null = null

const publishPercent = computed(() => {
  if (!candidateStatus.value || !candidateStatus.value.total_count) return 0
  const done = candidateStatus.value.done_count || 0
  const total = candidateStatus.value.total_count
  return Math.round((done / total) * 100)
})

interface ResultSiteStatus {
  name: string
  status: 'queued' | 'publishing' | 'done' | 'failed' | 'skipped'
  label: string
  tagColor: string
  icon: ReturnType<typeof markRaw>
}

const resultSiteStatus = computed<ResultSiteStatus[]>(() => {
  return selectedTargets.value.map(name => {
    const status = inferSiteStatus(name)
    const cfg: Record<string, { label: string; tagColor: string; icon: ReturnType<typeof markRaw> }> = {
      queued:      { label: '排队中', tagColor: 'blue',   icon: markRaw(ClockCircleOutlined) },
      publishing:  { label: '发布中', tagColor: 'processing', icon: markRaw(LoadingOutlined) },
      done:        { label: '已完成', tagColor: 'green',  icon: markRaw(CheckCircleFilled) },
      failed:      { label: '失败',   tagColor: 'red',    icon: markRaw(CloseCircleFilled) },
      skipped:     { label: '已跳过', tagColor: 'default',icon: markRaw(StopOutlined) },
    }
    const c = cfg[status] || cfg.queued
    return { name, status, label: c.label, tagColor: c.tagColor, icon: c.icon }
  })
})

function inferSiteStatus(_name: string): ResultSiteStatus['status'] {
  if (!candidateStatus.value) return 'queued'
  if (candidateStatus.value.publish_status === 'done') return 'done'
  if (candidateStatus.value.publish_status === 'failed') return 'failed'
  if (candidateStatus.value.publish_status === 'skipped') return 'skipped'
  if (candidateStatus.value.publish_status === 'publishing') return 'publishing'
  return 'queued'
}

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
      subtitle: form.value.subtitle,
      description: form.value.description,
      media_info: form.value.mediaInfo,
      screenshots: form.value.screenshots,
      poster: form.value.poster,
      statement: form.value.statement,
      douban_link: form.value.doubanLink,
      imdb_link: form.value.imdbLink,
      tmdb_link: form.value.tmdbLink,
      tags: form.value.tags,
      title_components: titleComponents.value || undefined,
      bdinfo: form.value.bdinfo || undefined,
      target_sites: selectedTargets.value,
    })
    submittedCandidateId.value =
      (resp.data?.data as unknown as { candidate_id?: number })?.candidate_id || 0
    currentStep.value = 3
    message.success('发布候选已创建')
    startCandidatePoll()
  } catch (e: unknown) {
    submitError.value = (e as Error).message
    currentStep.value = 3
  } finally {
    submitting.value = false
  }
}

function startCandidatePoll() {
  if (candidatePollTimer) clearInterval(candidatePollTimer)
  candidatePollTimer = setInterval(async () => {
    if (!submittedCandidateId.value) return
    try {
      const resp = await publishApi.getCandidate(submittedCandidateId.value)
      const c = resp.data?.data as unknown as Record<string, unknown> | undefined
      if (c) {
        candidateStatus.value = {
          publish_status: c.publish_status as string,
          total_count: selectedTargets.value.length,
          done_count: c.publish_status === 'done' ? selectedTargets.value.length : 0,
          fail_count: c.publish_status === 'failed' ? 1 : 0,
        }
        if (c.publish_status === 'done' || c.publish_status === 'failed') {
          stopCandidatePoll()
        }
      }
    } catch { /* silent */ }
  }, 3000)
}

function stopCandidatePoll() {
  if (candidatePollTimer) {
    clearInterval(candidatePollTimer)
    candidatePollTimer = null
  }
}

// --- Modal lifecycle ---
watch(() => props.open, (val) => {
  if (val) {
    resetWizard()
    if (props.presetTorrent) {
      selectedTorrent.value = {
        info_hash: props.presetTorrent.info_hash,
        name: props.presetTorrent.name,
        size: props.presetTorrent.size,
        save_path: props.presetTorrent.save_path,
        client_id: props.presetTorrent.client_id,
        upload_speed: 0,
        seeders: 0,
        state: props.presetTorrent.state,
      }
      selectedClientId.value = props.presetTorrent.client_id
      currentStep.value = 1
      enterAnalyze()
    } else {
      fetchClients()
    }
  }
})

watch(currentStep, () => {
  if (bodyRef.value) bodyRef.value.scrollTop = 0
})

function resetWizard() {
  stopCandidatePoll()
  currentStep.value = 0
  selectedTorrent.value = null
  analyzeResult.value = null
  analyzeError.value = ''
  siteList.value = []
  selectedTargets.value = []
  submitError.value = ''
  submittedCandidateId.value = 0
  candidateStatus.value = null
  form.value = { title: '', subtitle: '', mediaInfo: '', description: '', screenshots: [], statement: '', poster: '', doubanLink: '', imdbLink: '', tmdbLink: '', tags: [], removedDeclarations: [], bdinfo: '' }
  titleComponents.value = null
  standardizedParams.value = null
  reviewTab.value = 'main'
}

function handleCancel() {
  emit('update:open', false)
}

function handlePublishAnother() {
  resetWizard()
  fetchClients()
}

function restoreDecl(idx: number) {
  const decl = form.value.removedDeclarations.splice(idx, 1)[0]
  if (decl) {
    form.value.description = '[quote]' + decl + '[/quote]\n' + form.value.description
  }
}

function handleDone() {
  emit('success')
  emit('update:open', false)
}

onUnmounted(() => {
  stopCandidatePoll()
})
</script>

<style scoped>
.wizard-shell {
  display: flex;
  flex-direction: column;
  height: 75vh;
}

/* ═══ Header ═══ */
.wizard-header {
  flex-shrink: 0;
  border-bottom: 1px solid #f0f0f0;
}
.wizard-header-bar {
  display: flex;
  align-items: baseline;
  gap: 12px;
  padding: 16px 28px 0;
}
.wizard-header-title {
  font-size: 18px;
  font-weight: 700;
}
.wizard-header-subtitle {
  font-size: 13px;
  color: #999;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 700px;
}
.wizard-steps {
  display: flex;
  align-items: center;
  padding: 12px 28px 16px;
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

/* ═══ Body ═══ */
.wizard-body {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 16px 28px;
  min-height: 0;
}
.step-content {
  padding-bottom: 16px;
}
.step-toolbar {
  margin-bottom: 12px;
  display: flex;
  align-items: center;
}

/* ─── 分析中 ─── */
.analyzing-state {
  text-align: center;
  padding: 60px 0;
}

/* ─── 信息摘要 ─── */
.info-strip {
  display: flex;
  gap: 24px;
  padding: 12px 16px;
  background: #fafafa;
  border-radius: 6px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.info-strip-item {
  display: flex;
  align-items: center;
  gap: 6px;
}
.info-strip-label {
  font-size: 12px;
  color: #999;
}
.mono-text {
  font-family: 'Courier New', monospace;
  font-size: 12px;
  color: #666;
}

/* ─── Tab 样式 ─── */
.review-tabs :deep(.ant-tabs-nav) {
  margin-bottom: 16px;
}
.review-form {
  max-width: 800px;
}
.empty-hint {
  text-align: center;
  padding: 40px 0;
  color: #999;
}
.poster-preview-box {
  text-align: center;
  padding: 16px;
  background: #fafafa;
  border-radius: 6px;
  min-height: 300px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.poster-preview-img {
  border-radius: 6px;
  border: 1px solid #e8e8e8;
  object-fit: cover;
}
.poster-empty {
  color: #999;
}
.poster-empty p {
  margin-top: 8px;
}
.decl-item {
  margin-bottom: 16px;
  border: 1px solid #ffa39e;
  border-radius: 6px;
  overflow: hidden;
}
.decl-header {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 8px 12px;
  background: #fff1f0;
}
.decl-content {
  padding: 12px;
  margin: 0;
  font-size: 12px;
  font-family: 'Courier New', monospace;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 200px;
  overflow-y: auto;
  background: #fafafa;
}
.title-preview-section {
  margin-top: 8px;
}
.title-preview-item {
  display: flex;
  align-items: baseline;
  gap: 8px;
  padding: 4px 0;
  border-bottom: 1px solid #f5f5f5;
}
.title-preview-site {
  font-size: 12px;
  font-weight: 600;
  color: #1677ff;
  white-space: nowrap;
  min-width: 60px;
}
.title-preview-text {
  font-size: 12px;
  font-family: 'Courier New', monospace;
  color: #666;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}
.mono-font {
  font-family: 'Courier New', 'Consolas', monospace;
  font-size: 12px;
}

/* ─── 截图 ─── */
.screenshot-section {
  padding: 4px 0;
}
.screenshot-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
}
.screenshot-item {
  position: relative;
  border: 1px solid #e8e8e8;
  border-radius: 6px;
  overflow: hidden;
  transition: box-shadow 0.2s;
}
.screenshot-item:hover {
  box-shadow: 0 2px 8px rgba(0,0,0,0.12);
}
.screenshot-img {
  display: block;
  border-radius: 4px;
  object-fit: cover;
}
.screenshot-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 2px 8px;
  background: #fafafa;
}
.screenshot-index {
  font-size: 12px;
  color: #999;
}

/* ─── 选站统计 ─── */
.site-stats {
  display: flex;
  gap: 16px;
  margin-bottom: 16px;
}
.site-stat-card {
  flex: 1;
  text-align: center;
  padding: 16px;
  border-radius: 8px;
  border: 1px solid #f0f0f0;
}
.site-stat-num {
  font-size: 28px;
  font-weight: 700;
  line-height: 1.2;
}
.site-stat-label {
  font-size: 13px;
  color: #999;
  margin-top: 4px;
}
.stat-available .site-stat-num { color: #1677ff; }
.stat-selected .site-stat-num { color: #52c41a; }
.stat-blocked  .site-stat-num { color: #ff4d4f; }

.site-toolbar {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
}

/* ─── 站点按钮网格 ─── */
.site-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.site-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 10px 18px;
  border: 2px solid #e8e8e8;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  user-select: none;
  background: #fafafa;
  min-width: 120px;
  justify-content: center;
}
.site-btn:hover:not(.blocked) {
  border-color: #1677ff;
  transform: translateY(-1px);
  box-shadow: 0 2px 6px rgba(22,119,255,0.15);
}
.site-btn.selected {
  border-color: #52c41a;
  background: #f6ffed;
}
.site-btn.blocked {
  border-color: #ffa39e;
  background: #fff1f0;
  cursor: not-allowed;
  opacity: 0.65;
}
.site-btn-name {
  font-size: 14px;
}
.site-btn.selected .site-btn-name {
  font-weight: 600;
  color: #389e0d;
}
.site-btn.blocked .site-btn-name {
  color: #cf1322;
  text-decoration: line-through;
}
.site-btn-check {
  color: #52c41a;
  font-size: 16px;
}
.site-btn-blocked-icon {
  color: #ff4d4f;
  font-size: 14px;
}

/* ─── 发布结果 ─── */
.results-section {
  padding: 8px 0;
}
.results-header {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 20px;
  background: linear-gradient(135deg, #f6ffed 0%, #d9f7be 100%);
  border-radius: 8px;
  margin-bottom: 20px;
}
.results-header-icon {
  font-size: 48px;
  color: #52c41a;
}
.results-header-title {
  font-size: 18px;
  font-weight: 700;
}
.results-header-subtitle {
  font-size: 13px;
  color: #666;
  margin-top: 4px;
}
.progress-section {
  margin-bottom: 20px;
}
.progress-row {
  display: flex;
  align-items: center;
}
.progress-label {
  font-size: 13px;
  font-weight: 600;
  white-space: nowrap;
}
.progress-count {
  font-size: 13px;
  color: #666;
  white-space: nowrap;
}
.result-site-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 12px;
}
.result-site-card {
  text-align: center;
  padding: 16px 8px 12px;
  border: 1px solid #f0f0f0;
  border-radius: 8px;
  position: relative;
  overflow: hidden;
  transition: transform 0.2s;
}
.result-site-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 2px 8px rgba(0,0,0,0.08);
}
.result-site-card-bar {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 4px;
}
.result-site-card-bar.queued { background: #1677ff; }
.result-site-card-bar.publishing { background: #722ed1; }
.result-site-card-bar.done { background: #52c41a; }
.result-site-card-bar.failed { background: #ff4d4f; }
.result-site-card-bar.skipped { background: #d9d9d9; }
.result-site-card-icon {
  font-size: 32px;
  margin-bottom: 4px;
}
.result-site-card.queued .result-site-card-icon { color: #1677ff; }
.result-site-card.publishing .result-site-card-icon { color: #722ed1; }
.result-site-card.done .result-site-card-icon { color: #52c41a; }
.result-site-card.failed .result-site-card-icon { color: #ff4d4f; }
.result-site-card.skipped .result-site-card-icon { color: #bfbfbf; }
.result-site-card-name {
  font-size: 14px;
  font-weight: 600;
  margin: 6px 0 6px;
}

/* ═══ Footer ═══ */
.wizard-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 28px;
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
.footer-info {
  color: #52c41a;
  font-size: 13px;
  font-weight: 500;
}
.wizard-footer-right {
  display: flex;
  gap: 8px;
}
.wizard-footer-right button {
  min-width: 80px;
}
</style>
