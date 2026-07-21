<template>
  <div class="step-content">
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
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { CheckCircleFilled, StopOutlined, InfoCircleOutlined } from '@ant-design/icons-vue'
import { publishTorrentsApi } from '@/api/publish'

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

// 标题预览缓存（v-model 双向，主文件也可访问）
const titlePreviews = ref<Record<string, string>>({})

function toggleSite(site: SiteItem) {
  if (site.blocked) return
  const idx = selectedTargets.value.indexOf(site.name)
  if (idx >= 0) {
    selectedTargets.value.splice(idx, 1)
    delete titlePreviews.value[site.name]
  } else {
    selectedTargets.value.push(site.name)
    if (props.titleComponents) {
      loadTitlePreview(site.name)
    }
  }
}

async function loadTitlePreview(siteName: string) {
  if (!props.titleComponents) return
  try {
    const resp = await publishTorrentsApi.previewTitle({
      target_site: siteName,
      title_components: props.titleComponents,
    })
    if (resp.data?.data?.title) {
      titlePreviews.value[siteName] = resp.data.data.title
    }
  } catch { /* silent */ }
}

function selectAllAvailable() {
  selectedTargets.value = props.siteList.filter(s => !s.blocked).map(s => s.name)
}
</script>
