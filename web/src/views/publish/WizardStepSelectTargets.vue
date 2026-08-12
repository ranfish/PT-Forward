<template>
  <div class="step-content step-content-with-preview">
    <div class="step-left">
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
          <!-- v0.0.255 §56.29 匿名发布 toggle（全局） -->
          <a-switch
            v-model:checked="anonymous"
            size="small"
            style="margin-left: 12px"
          />
          <span style="margin-left: 6px; font-size: 12px; color: #666">匿名发布</span>
          <a-tooltip title="勾选后所有目标站都以匿名身份发布（不支持的站点自动忽略此字段）">
            <InfoCircleOutlined style="margin-left: 4px; color: #999; cursor: help" />
          </a-tooltip>
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
                previewed: site.name === previewTarget,
                blocked: site.blocked,
              }"
              @click="onSiteClick(site)"
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

    <!-- v0.0.256 §56.24 字段预览面板（右侧） -->
    <div class="step-right">
      <a-divider style="margin: 0 0 8px" orientation="left">
        <span style="font-size: 12px; color: #666">字段预览</span>
      </a-divider>
      <div v-if="!previewTarget" class="preview-empty">
        <p style="color: #999; text-align: center; padding: 40px 0">
          点击上方站点查看该站字段预览
        </p>
      </div>
      <PublishFieldPreview
        v-else
        :target-site="previewTarget"
        :mode="previewData?.mode || ''"
        :fields="previewData?.fields || []"
        :completeness="previewData?.completeness || null"
        :loading="previewLoading"
        :error="previewError"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { CheckCircleFilled, StopOutlined, InfoCircleOutlined } from '@ant-design/icons-vue'
import { publishTorrentsApi, manualForwardApi } from '@/api/publish'
import type { PreviewResponse } from '@/api/types'
import PublishFieldPreview from './PublishFieldPreview.vue'

// v0.0.256: 转载向导表单状态（主文件 + 子组件共享，类型导出供其他子组件复用）
export interface WizardFormState {
  title: string
  subtitle: string
  mediaInfo: string
  description: string
  screenshots: string[]
  statement: string
  poster: string
  doubanLink: string
  imdbLink: string
  tmdbLink: string
  tags: string[]
  removedDeclarations: string[]
  bdinfo: string
  anonymous: boolean
}

export interface SiteItem {
  name: string
  domain: string
  blocked: boolean
  blockReason: string
}

const props = defineProps<{
  siteList: SiteItem[]
  targetsLoading: boolean
  anonymous: boolean
  titleComponents: Record<string, string> | null
  infoHash: string  // v0.0.256 §56.24 预览 API 需要
  mode: string  // ptgen_first / detail_first
}>()

const emit = defineEmits<{
  'update:anonymous': [value: boolean]
}>()

// v-model:anonymous 双向绑定（避免 vue/no-mutating-props 报错）
const anonymous = computed({
  get: () => props.anonymous,
  set: (v: boolean) => emit('update:anonymous', v),
})

const selectedTargets = defineModel<string[]>({ default: () => [] })

const availableCount = computed(() => props.siteList.filter(s => !s.blocked).length)
const blockedCount = computed(() => props.siteList.filter(s => s.blocked).length)

// 标题预览缓存
const titlePreviews = ref<Record<string, string>>({})

// v0.0.256 §56.24 字段预览 state
const previewTarget = ref<string>('')  // 当前预览的目标站
const previewData = ref<PreviewResponse | null>(null)
const previewLoading = ref(false)
const previewError = ref('')

function onSiteClick(site: SiteItem) {
  toggleSite(site)
  // 选中可发布站时，自动切换预览到该站
  if (!site.blocked && selectedTargets.value.includes(site.name)) {
    loadFieldPreview(site.name)
  }
}

async function loadFieldPreview(siteName: string) {
  if (!props.infoHash) {
    previewError.value = '缺少 info_hash，无法预览'
    return
  }
  previewTarget.value = siteName
  previewLoading.value = true
  previewError.value = ''
  previewData.value = null
  try {
    const resp = await manualForwardApi.previewFields({
      infoHash: props.infoHash,
      targetSite: siteName,
      mode: props.mode || 'ptgen_first',
    })
    previewData.value = (resp.data?.data as unknown as PreviewResponse) || null
  } catch (e: unknown) {
    previewError.value = (e as Error).message
  } finally {
    previewLoading.value = false
  }
}

function toggleSite(site: SiteItem) {
  if (site.blocked) return
  const idx = selectedTargets.value.indexOf(site.name)
  if (idx >= 0) {
    selectedTargets.value.splice(idx, 1)
    delete titlePreviews.value[site.name]
    // 如果取消的是当前预览站，切到第一个选中站
    if (previewTarget.value === site.name) {
      previewTarget.value = selectedTargets.value[0] || ''
      if (previewTarget.value) {
        loadFieldPreview(previewTarget.value)
      } else {
        previewData.value = null
      }
    }
  } else {
    selectedTargets.value.push(site.name)
    if (props.titleComponents) {
      loadTitlePreview(site.name)
    }
    // 第一个选中的站自动加载预览
    if (!previewTarget.value) {
      loadFieldPreview(site.name)
    }
  }
}

async function loadTitlePreview(siteName: string) {
  if (!props.titleComponents) return
  try {
    const resp = await publishTorrentsApi.previewTitle({
      targetSite: siteName,
      titleComponents: props.titleComponents,
    })
    const title = resp.data?.data?.title
    if (title) {
      titlePreviews.value[siteName] = title
    } else {
      titlePreviews.value[siteName] = '—'
    }
  } catch (e: unknown) {
    titlePreviews.value[siteName] = `错误: ${(e as Error).message}`
  }
}

function selectAllAvailable() {
  selectedTargets.value = props.siteList.filter(s => !s.blocked).map(s => s.name)
  if (props.titleComponents && selectedTargets.value.length > 0) {
    loadAllTitlePreviews()
  }
  if (selectedTargets.value.length > 0 && !previewTarget.value) {
    loadFieldPreview(selectedTargets.value[0])
  }
}

async function loadAllTitlePreviews() {
  if (!props.titleComponents || selectedTargets.value.length === 0) return
  try {
    const resp = await publishTorrentsApi.previewTitleBatch({
      targetSites: selectedTargets.value,
      titleComponents: props.titleComponents,
    })
    const results = resp.data?.data?.results
    if (results) {
      for (const [site, title] of Object.entries(results)) {
        titlePreviews.value[site] = title || '—'
      }
    }
  } catch { /* ignore */ }
}

// 外部 infoHash 变化时清空预览（切到新种子）
watch(() => props.infoHash, () => {
  previewTarget.value = ''
  previewData.value = null
  previewError.value = ''
})
</script>

<style scoped>
.step-content-with-preview {
  display: flex;
  gap: 16px;
}
.step-left {
  flex: 1;
  min-width: 0;
}
.step-right {
  width: 360px;
  flex-shrink: 0;
}
.preview-empty {
  background: #fafafa;
  border: 1px dashed #d9d9d9;
  border-radius: 6px;
}
.site-btn.previewed {
  border-color: #722ed1;
  background: #f9f0ff;
}
</style>
