<template>
  <a-drawer
    :open="open"
    :title="null"
    width="92%"
    placement="right"
    :body-style="{ padding: '0' }"
    :header-style="{ display: 'none' }"
    :mask-closable="!loading"
    :keyboard="!loading"
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
        </div>
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

        <!-- 编辑/预览 -->
        <div v-else class="csp-step-content">
          <!-- §59.20 ⑨: 预览模式 -->
          <template v-if="seedPreviewMode">
            <div ref="previewScrollRef" style="max-width: 1100px">
              <a-typography-title :level="5">发布预览</a-typography-title>

              <!-- §59.86: ① 种子标识（卡片） -->
              <a-card size="small" style="margin-bottom: 12px">
                <template #title><span style="font-size: 14px">① 种子标识</span></template>
                <a-descriptions :column="1" size="small">
                  <a-descriptions-item label="主标题">
                    <span style="font-size: 15px; font-weight: 600">{{ form.title || '—' }}</span>
                  </a-descriptions-item>
                  <a-descriptions-item v-if="form.subtitle" label="副标题">
                    <span style="color: #666">{{ form.subtitle }}</span>
                  </a-descriptions-item>
                </a-descriptions>
              </a-card>

              <!-- §59.86: ② 技术规格（卡片） -->
              <a-card size="small" style="margin-bottom: 12px">
                <template #title><span style="font-size: 14px">② 技术规格</span></template>
              <SeedTechDescriptions :tc="form.titleComponents" :encode="seedEncode" :column="4" />
              </a-card>

              <!-- §59.86: ③ 内容属性（卡片） -->
              <a-card size="small" style="margin-bottom: 12px">
                <template #title><span style="font-size: 14px">③ 内容属性</span></template>
                <div v-if="seedRegionGenre.region.length || seedRegionGenre.genre.length">
                    <span v-if="seedRegionGenre.region.length" style="margin-right: 16px">
                      产地：<a-tag v-for="r in seedRegionGenre.region" :key="r" color="geekblue">{{ r }}</a-tag>
                    </span>
                    <span v-if="seedRegionGenre.genre.length">
                      类型：<a-tag v-for="g in seedRegionGenre.genre" :key="g" color="purple">{{ g }}</a-tag>
                    </span>
                  </div>
                  <div v-else style="color: #999">暂无产地 / 类型数据（需 PTGen 获取）</div>
              </a-card>

              <!-- §59.86: ④ 标签（卡片） -->
              <a-card size="small" style="margin-bottom: 12px">
                <template #title><span style="font-size: 14px">④ 标签</span></template>
                <div v-if="previewTags.length">
                  <a-tag v-for="t in previewTags" :key="t" :color="isRestrictedTag(t) ? 'red' : 'blue'">
                    {{ tagDisplayName(t) }}
                  </a-tag>
                </div>
                <div v-else style="color: #999">未选择标签</div>
              </a-card>

              <a-modal :open="!!previewShotPreview" :footer="null" width="900px" @cancel="previewShotPreview = ''">
                <img v-if="previewShotPreview" :src="previewShotPreview" style="width: 100%" />
              </a-modal>

              <!-- §59.87: ⑤ 发布简介（原⑥，⑤媒体数据已删——冗余；整块渲染=所见即所发） -->
              <a-card size="small" style="margin-bottom: 12px">
                <template #title><span style="font-size: 14px">⑤ 发布简介（最终发布内容）</span></template>
                <template #extra>
                  <a-radio-group v-model:value="previewDescMode" size="small">
                    <a-radio-button value="rendered">渲染效果</a-radio-button>
                    <a-radio-button value="source">BBCode 源码</a-radio-button>
                  </a-radio-group>
                </template>
                <div v-if="previewDescMode === 'rendered' && previewRenderedDesc" style="padding: 12px; background: #fafafa; border-radius: 4px; line-height: 1.8" v-html="previewRenderedDesc"></div>
                <pre v-else-if="previewDescMode === 'source' && previewDescSource" style="background: #f5f5f5; padding: 12px; border-radius: 4px; font-size: 12px; max-height: 500px; overflow: auto; white-space: pre-wrap">{{ previewDescSource }}</pre>
                <div v-else style="color: #999">暂无简介数据</div>
              </a-card>

              <!-- §59.86: ⑦ 校验状态（卡片） -->
              <a-card size="small">
                <template #title><span style="font-size: 14px">⑦ 校验状态</span></template>
              <a-alert
                :type="seedMissingFields.length === 0 ? 'success' : 'warning'"
                show-icon
                :message="seedMissingFields.length === 0 ? '✓ 9 必需字段齐全，已自动审核' : `⚠ 仍缺 ${seedMissingFields.length} 个字段：${seedMissingFields.join(', ')}`"
              />
              </a-card>
            </div>
          </template>

          <!-- 编辑模式 -->
          <template v-else>
          <a-tabs v-model:active-key="activeTab">
            <!-- Tab 1: 种子详情 -->
            <a-tab-pane key="detail" tab="种子详情">
              <a-descriptions :column="3" bordered size="small" style="max-width: 900px">
                  <a-descriptions-item label="主标题" :span="3">{{ form.title || '—' }}</a-descriptions-item>
                  <a-descriptions-item label="副标题" :span="3">{{ form.subtitle || '—' }}</a-descriptions-item>
              </a-descriptions>
              <!-- §59.135/§59.136: 技术规格表——与预览②同一组件同一 column（5行×4列 视觉同步） -->
              <SeedTechDescriptions :tc="form.titleComponents" :encode="seedEncode" :column="4" style="max-width: 900px; margin-top: 16px" />
                <!-- §59.75: 产地/类型（PTGen 源归一只读展示——发布映射消费 canonical） -->
                <a-form-item v-if="seedRegionGenre.region.length || seedRegionGenre.genre.length" label="产地 / 类型" style="max-width: 900px; margin-top: 16px">
                  <span v-if="seedRegionGenre.region.length" style="margin-right: 16px">
                    产地：<a-tag v-for="r in seedRegionGenre.region" :key="r" color="geekblue">{{ r }}</a-tag>
                  </span>
                  <span v-if="seedRegionGenre.genre.length">
                    类型：<a-tag v-for="g in seedRegionGenre.genre" :key="g" color="purple">{{ g }}</a-tag>
                  </span>
                </a-form-item>
                <!-- §59.26: 标签（可编辑，供发布使用） -->
                <a-form-item label="标签" style="max-width: 900px; margin-top: 16px">
                  <TagSelector v-model="form.tags" :display-labels="form.tagLabels" />
                </a-form-item>
                <div v-if="seedMissingFields.length > 0" style="margin-top: 12px; padding: 8px 12px; background: #fffbe6; border-radius: 4px; font-size: 13px">
                  <span style="color: #faad14">⚠ 缺失字段：</span>{{ seedMissingFields.join(', ') }}
                </div>
                <div v-else-if="seedReviewed" style="margin-top: 12px; padding: 8px 12px; background: #f6ffed; border-radius: 4px; font-size: 13px; color: #52c41a">
                  ✓ 已审核（9 必需字段齐全）
                </div>
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
                  <a-textarea v-model:value="form.statement" :auto-size="{ minRows: 3, maxRows: 25 }" placeholder="源站官组声明（只读）" disabled />
                </a-form-item>
              </a-form>
            </a-tab-pane>

            <!-- Tab 3: 视频截图 -->
            <a-tab-pane key="screenshots" tab="视频截图">
              <div style="margin-bottom: 12px; display: flex; gap: 8px">
                <a-button :loading="refreshing === 'screenshots'" @click="doRefresh('screenshots')">{{ seedIsLocal ? '重新获取截图' : '从源站重新获取截图' }}</a-button>
                <a-button :loading="refreshing === 'rehost_screenshots'" :disabled="form.screenshots.length === 0" @click="doRefresh('rehost_screenshots')">一键转存到图床</a-button>
              </div>
              <ScreenshotManager
                ref="shotManagerRef"
                v-model:screenshots="form.screenshots"
                :screenshot-in-desc="form.screenshotInDesc"
                @update:screenshot-in-desc="form.screenshotInDesc = $event"
              />
            </a-tab-pane>

            <!-- Tab 4: 简介详情 -->
            <a-tab-pane key="intro" tab="简介详情">
              <div style="margin-bottom: 8px; display: flex; gap: 8px">
                <!-- §59.45: 简介重获是数据源修复动作（与 Tab3/Tab5 同性质），维护模式放开 -->
                <a-button :loading="refreshing === 'intro'" @click="doRefresh('intro')">重新获取简介（PTGen）</a-button>
              </div>
              <a-textarea v-model:value="form.description" :rows="36" placeholder="BBCode 简介正文" style="font-family: monospace" />
              <!-- §59.20: maintenanceOnly 模式下外部链接只读展示 -->
              <div v-if="form.doubanLink || form.imdbLink || form.tmdbLink" style="margin-top: 12px">
                <span style="color: #999; font-size: 12px; margin-right: 12px">外部链接：</span>
                <a v-if="form.doubanLink" :href="form.doubanLink" target="_blank" style="margin-right: 8px; font-size: 12px">豆瓣</a>
                <a v-if="form.imdbLink" :href="form.imdbLink" target="_blank" style="margin-right: 8px; font-size: 12px">IMDb</a>
                <a v-if="form.tmdbLink" :href="form.tmdbLink" target="_blank" style="font-size: 12px">TMDb</a>
              </div>
            </a-tab-pane>

            <!-- Tab 5: 媒体信息 -->
            <a-tab-pane key="mediainfo" tab="媒体信息">
              <div style="margin-bottom: 8px; display: flex; gap: 8px">
                <!-- §59.36: MI 重获是数据源修复动作（与 Tab3 截图同性质），维护模式放开 -->
                <a-button :loading="refreshing === 'mediainfo'" @click="doRefresh('mediainfo')">{{ seedIsLocal ? '重新获取 MediaInfo' : '从源站重新获取 MediaInfo' }}</a-button>
              </div>
              <!-- §59.36: 维护模式 MI 只读展示，重获走上方按钮（数据修复动作） -->
              <a-textarea v-model:value="form.mediaInfo" :rows="36" placeholder="MediaInfo 文本" style="font-family: monospace; font-size: 12px" disabled />
              <a-form-item v-if="form.bdinfo" label="BDInfo" style="margin-top: 12px">
                <a-textarea v-model:value="form.bdinfo" :rows="10" style="font-family: monospace; font-size: 12px" disabled />
              </a-form-item>
            </a-tab-pane>

            <!-- §59.20 Tab 6: 已过滤声明（只读预览） -->
            <a-tab-pane key="filtered" tab="已过滤声明">
              <a-alert
                type="info" show-icon style="margin-bottom: 12px"
                message="以下声明将在发布时被自动过滤。可在「发布规则 → 声明过滤规则」中管理过滤模式。"
              />
              <div v-if="filteredDeclarations.length > 0">
                <div v-for="(item, idx) in filteredDeclarations" :key="idx" style="margin-bottom: 8px; padding: 8px; background: #fafafa; border-radius: 4px; border-left: 3px solid #ff4d4f">
                  <div style="font-size: 12px; color: #ff4d4f; margin-bottom: 4px">命中模式: {{ item.pattern }}</div>
                  <pre style="margin: 0; font-size: 12px; white-space: pre-wrap; color: #666">{{ item.text }}</pre>
                </div>
              </div>
              <a-empty v-else description="当前简介无匹配的过滤声明" />
            </a-tab-pane>
           </a-tabs>
          </template><!-- v-else (editing mode) -->
        </div>

      </div>

      <!-- ═══ Footer ═══ -->
      <div class="csp-footer">
        <div class="csp-footer-left"></div>
        <div class="csp-footer-right">
          <a-button @click="handleClose">取消</a-button>
          <!-- §59.20 ⑨: 预览模式 → 返回编辑 + 确认完成 -->
          <template v-if="seedPreviewMode">
            <a-button @click="backToEdit">返回编辑</a-button>
            <a-tooltip :title="previewScrolled ? '' : '请完整浏览预览内容（滚动到底）后确认'">
              <a-button type="primary" :disabled="!previewScrolled" @click="confirmDone">确认完成</a-button>
            </a-tooltip>
          </template>
          <!-- 编辑模式 → 预览按钮 -->
          <template v-else>
            <a-button type="primary" :loading="saving" :disabled="loading || !!loadError" @click="saveOnly">
              预览
            </a-button>
          </template>
        </div>
      </div>
    </div>
  </a-drawer>
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted, nextTick } from 'vue'
import { message } from 'ant-design-vue'
import { manualForwardApi, publishTorrentsApi, seedConfigApi } from '@/api/publish'
import type { SeedDetail } from '@/api/publish'
import { parseBBCode } from '@/utils/bbcode'
import { tagDisplayName as tagDisplayNameCommon } from '@/utils/tagDisplay'
import { TAG_GROUPS } from '@/generated/dict'
import TagSelector from './TagSelector.vue'
import SeedTechDescriptions from './SeedTechDescriptions.vue'
import ScreenshotManager from './ScreenshotManager.vue'

// §59.54: 截图管理器引用（转存前快照）
const shotManagerRef = ref<InstanceType<typeof ScreenshotManager> | null>(null)

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
  // §59.141: 直开预览——发布页"预览种子"按钮（加载完自动 saveOnly 进预览, 幂等）
  initialPreview?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:open', val: boolean): void
  (e: 'success'): void
}>()

// --- State ---
const activeTab = ref('detail')
const loading = ref(false)
const loadingText = ref('')
const loadingProgress = ref(0)
const loadError = ref('')
const saving = ref(false)
const refreshing = ref('')
const bodyRef = ref<HTMLElement>()

const selectedTorrent = ref<PresetTorrent | null>(null)
// 源站名（loadSeedDetail 回填；refresh/saveOnly 的 siteName 参数源）
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
  // §59.75: 产地/类型（label 形态只读展示）
  region: [] as string[],
  genre: [] as string[],
  // §59.106: 标签显示名（后端 tag_labels; null=用 TagSelector 本地映射）
  tagLabels: null as string[] | null,
})


// --- Computed ---


function resetPanel() {
  activeTab.value = 'detail'
  selectedTorrent.value = null
  currentSourceSite.value = ''
  seedPreviewMode.value = false
  previewRenderedDesc.value = ''
  form.value = {
    title: '', subtitle: '', mediaInfo: '', description: '', screenshots: [],
    statement: '', poster: '', doubanLink: '', imdbLink: '', tmdbLink: '',
    tags: [], removedDeclarations: [], bdinfo: '', anonymous: false, screenshotInDesc: false,
    titleComponents: {},
    region: [],
    genre: [],
    tagLabels: null,
  }
}

function handleClose() {
  emit('update:open', false)
}

// --- Lifecycle ---
watch(() => props.open, (val) => {
  if (val) {
    resetPanel()
    if (props.presetTorrent) {
      selectedTorrent.value = props.presetTorrent
      fillFormFromPreset()
      loadDeclPatterns()
    }
  }
})

function fillFormFromPreset() {
  const t = props.presetTorrent
  if (!t) return
  form.value = {
    title: t.name || '',
    subtitle: '',
    mediaInfo: '',
    description: '',
    screenshots: [],
    statement: '',
    poster: '',
    doubanLink: '', imdbLink: '', tmdbLink: '',
    tags: [],
    removedDeclarations: [],
    bdinfo: '',
    anonymous: false,
    screenshotInDesc: false,
    titleComponents: {},
    region: [],
    genre: [],
    tagLabels: null,
  }
  loading.value = false
  // §59.20: maintenanceOnly 模式从后端加载已存 metadata
  if (t.info_hash) {
    loadSeedDetail(t.info_hash)
  }
}

// §59.20: 从 GET /publish/seeds/:info_hash 加载种子配置
async function loadSeedDetail(infoHash: string) {
  loading.value = true
  loadError.value = ''
  try {
    const resp = await seedConfigApi.getSeed(infoHash, String(props.presetTorrent?.client_id || ''))
    const d: SeedDetail | undefined = resp.data?.data
    if (d) {
      form.value.title = d.title || form.value.title
      form.value.subtitle = d.subtitle || ''
      form.value.mediaInfo = d.mediainfo || ''
      form.value.description = d.description || ''
      form.value.screenshots = d.screenshots || []
      form.value.statement = d.statement || ''
      form.value.poster = d.poster || ''
      form.value.bdinfo = d.bdinfo || ''
      form.value.doubanLink = d.douban_url || ''
      form.value.imdbLink = d.imdb_url || ''
      form.value.tmdbLink = d.tmdb_url || ''
      // 18 TechProfile 字段 → titleComponents（Tab 1 只读展示）
      form.value.titleComponents = {
        main_title: d.main_title || '',
        season_episode: d.season_episode || '',
        year: d.year || '',
        release_group: d.release_group || '',
        chinese_prefix: d.chinese_prefix || '',
        resolution: d.resolution || '',
        video_codec: d.video_codec || '',
        audio_codec: d.audio_codec || '',
        audio_channels: d.audio_channels || '',
        audio_technology: d.audio_tech || '',
        audio_tracks: d.audio_tracks ? String(d.audio_tracks) : '', // §59.107: Tab1 组装漏映射（预览有 Tab1 无实锤）
        hdr: d.hdr || '',
        bit_depth: d.bit_depth || '',
        source_type: d.source_type || '',
        specification: d.specification || '',
        source_platform: d.source_platform || '',
        edition_info: d.edition_info || '',
        region_code: d.region_code || '',
        category: d.category || '',
        form: d.form || '',
        medium: d.medium || '', // §59.108: 编辑表单媒介输入框（曾恒空）
      }
      // 状态
      seedMissingFields.value = d.missing_fields || []
      // §59.75: 产地/类型（labels 只读展示）
      form.value.region = d.region?.labels || []
      form.value.genre = d.genre?.labels || []
      seedReviewed.value = d.reviewed || false
      seedEncode.value = d.encode ?? false
      seedIsLocal.value = (d as any).is_local ?? true
      currentSourceSite.value = d.site_name || ''
      // §59.26: 标签（获取时推断，编辑时可修正）
      form.value.tags = d.tags || []
      // §59.106: 显示名优先用后端 tag_labels（dict bundle 无关——新词条旧缓存页面不显示代码）
      form.value.tagLabels = d.tag_labels || null
      // §59.141: 直开预览——发布页"预览种子"入口（数据加载完幂等保存进预览）
      if (props.initialPreview) {
        await saveOnly()
      }
    }
  } catch (e: unknown) {
    loadError.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

// §59.20: 种子配置页状态
const seedMissingFields = ref<string[]>([])
// §59.75: 产地/类型展示（label 形态）
const seedRegionGenre = computed(() => ({
  region: form.value.region || [],
  genre: form.value.genre || [],
}))
const seedReviewed = ref(false)
// §59.34: Encode 派生标识（后端真相源；组件 SeedTechDescriptions 消费）
const seedEncode = ref(false)
const seedIsLocal = ref(true) // §59.21: 默认 true（向后兼容）
// §59.20 ⑨: 预览模式（保存即预览）
const seedPreviewMode = ref(false)
const previewRenderedDesc = ref('')
// §59.81: 发布预览增强——全字段/标签着色/分段渲染/源码切换
const previewFieldsData = ref<Record<string, unknown>>({})
// §59.92: 预览完成门槛——滚动到底才可确认（30px 容差）。
// 滚动发生在 drawer .ant-drawer-body（预览容器自身不滚动，scroll 不冒泡）
// → 进入预览时对滚动祖先 addEventListener，离开时摘除。
const previewScrolled = ref(false)
const previewScrollRef = ref<HTMLElement | null>(null)
let previewScrollHost: HTMLElement | null = null

function onPreviewScroll(e: Event) {
  const el = e.target as HTMLElement
  if (el.scrollTop + el.clientHeight >= el.scrollHeight - 30) {
    previewScrolled.value = true
  }
}

function findScrollHost(el: HTMLElement | null): HTMLElement | null {
  let sc = el
  while (sc && sc !== document.body) {
    const st = getComputedStyle(sc)
    if (/(auto|scroll)/.test(st.overflowY)) return sc
    sc = sc.parentElement
  }
  return null
}

function attachPreviewScrollGate() {
  nextTick(() => {
    const host = findScrollHost(previewScrollRef.value)
    if (host) {
      host.addEventListener('scroll', onPreviewScroll, { passive: true })
      previewScrollHost = host
      // §59.101: 键盘滚动支持——焦点必须落在滚动容器上（↑↓/Home/End 作用于
      // 焦点元素所在滚动层; drawer 焦点陷阱下焦点不在滚动宿主→键盘滚不动实锤）
      host.tabIndex = -1
      host.focus({ preventScroll: true })
      if (host.scrollHeight <= host.clientHeight + 30) {
        previewScrolled.value = true
      }
    } else {
      previewScrolled.value = true
    }
  })
}

function detachPreviewScrollGate() {
  previewScrollHost?.removeEventListener('scroll', onPreviewScroll)
  previewScrollHost = null
}

const previewShotPreview = ref('')
const previewDescMode = ref<'rendered' | 'source'>('rendered')
const previewStatement = ref('')
const previewDescSource = ref('')

const previewTags = computed<string[]>(() => {
  const t = previewFieldsData.value.tags
  if (Array.isArray(t)) return t as string[]
  // 字符串形态（JSON）解析；畸形回退空
  if (typeof t === 'string' && t.startsWith('[')) {
    try { return JSON.parse(t) as string[] } catch { return [] }
  }
  return []
})
// §59.81: 禁转类标签红色（easy-upload getTagType 借鉴）
const isRestrictedTag = (t: string): boolean =>
  t === '禁转' || t === 'tag.禁转' || t === '限转' || t === 'tag.限转' || t === 'no_transfer'
// 标签显示名（dict label 优先）
// §59.110: 公共单点（utils/tagDisplay）——后端 tag_labels 权威优先
const previewTagLabels = computed<string[] | null>(() => {
  const tl = previewFieldsData.value.tag_labels
  return Array.isArray(tl) ? tl : null
})
const tagDisplayName = (t: string): string =>
  tagDisplayNameCommon(t, previewTags.value, previewTagLabels.value)
const previewStatementHTML = computed(() => parseBBCode(previewStatement.value))

// §59.20: 已过滤声明 Tab 预览
const declPatterns = ref<string[]>([])
const filteredDeclarations = computed(() => {
  if (!form.value.description || declPatterns.value.length === 0) return []
  const results: Array<{ pattern: string; text: string }> = []
  const quoteRe = /\[quote\]([\s\S]*?)\[\/quote\]/g
  let match: RegExpExecArray | null
  while ((match = quoteRe.exec(form.value.description)) !== null) {
    const blockText = match[1].trim()
    for (const pattern of declPatterns.value) {
      if (blockText.toLowerCase().includes(pattern.toLowerCase())) {
        results.push({ pattern, text: blockText })
        break
      }
    }
  }
  return results
})

async function loadDeclPatterns() {
  try {
    const resp = await publishTorrentsApi.getDeclarationFilters()
    declPatterns.value = resp.data?.data?.patterns || []
  } catch { /* silent */ }
}

// --- Refresh ---
// §59.51: 后台截图任务——启动 + 2s 轮询 + 会话一致性校验
let capturePollTimer: ReturnType<typeof setInterval> | null = null

async function startScreenshotCaptureTask() {
  if (!selectedTorrent.value) return
  refreshing.value = 'screenshots'
  const taskName = selectedTorrent.value.name
  try {
    await manualForwardApi.startScreenshotCapture({
      name: taskName,
      savePath: selectedTorrent.value.save_path || '',
      clientId: String(selectedTorrent.value.client_id || ''),
      infoHash: selectedTorrent.value.info_hash,
      siteName: currentSourceSite.value || selectedTorrent.value.source_site || '',
    })
    message.info('截图中…（约 1-2 分钟）')
    capturePollTimer = setInterval(async () => {
      try {
        const resp = await manualForwardApi.screenshotCaptureProgress()
        const st = resp.data?.data
        if (!st || st.active) return
        if (capturePollTimer) { clearInterval(capturePollTimer); capturePollTimer = null }
        refreshing.value = ''
        if (st.status === 'done' && st.screenshots && st.screenshots.length > 0) {
          // §59.51 遗漏5: 会话一致性——用户可能已切换到另一个种子
          if (selectedTorrent.value && selectedTorrent.value.name === taskName) {
            form.value.screenshots = st.screenshots
            message.success(`截图完成（${st.screenshots.length} 张）`)
          } else {
            message.info(`「${taskName.slice(0, 30)}」截图已完成，重新打开编辑器可见`)
          }
        } else {
          message.error('截图失败: ' + (st.error || '未知错误'))
        }
      } catch {
        // 轮询单次失败忽略（网络抖动），下轮继续
      }
    }, 2000)
  } catch (e: unknown) {
    refreshing.value = ''
    message.error(`截图任务启动失败: ${(e as Error).message}`)
  }
}

async function doRefresh(type: string) {
  if (!selectedTorrent.value) return
  // §59.51: 本地截图走后台任务（长任务轮询），源站截图保持旧同步链
  if (type === 'screenshots' && seedIsLocal.value) {
    await startScreenshotCaptureTask()
    return
  }
  refreshing.value = type
  try {
    const payload: { type: string; name: string; savePath?: string; infoHash?: string; siteName?: string; screenshots?: string[]; clientId?: string } = {
      type,
      name: selectedTorrent.value.name,
      savePath: selectedTorrent.value.save_path,
      infoHash: selectedTorrent.value.info_hash,
      siteName: currentSourceSite.value || selectedTorrent.value.source_site || '',
      clientId: String(selectedTorrent.value.client_id || ''),
    }
    if (type === 'rehost_screenshots') {
      payload.screenshots = form.value.screenshots
      // §59.54: 转存前快照（恢复引用按钮的还原源）
      const sm = shotManagerRef.value as unknown as { snapshotBeforeRehost?: () => void } | null
      sm?.snapshotBeforeRehost?.()
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


// §59.20: maintenanceOnly 模式保存——调 PUT /publish/seeds/:info_hash → 保存+预览
async function saveOnly() {
  if (!selectedTorrent.value?.info_hash) {
    message.error('缺少 info_hash')
    return
  }
  saving.value = true
  try {
    const resp = await seedConfigApi.putSeed(selectedTorrent.value.info_hash, {
      poster: form.value.poster,
      screenshots: form.value.screenshots,
      description: form.value.description,
      tags: form.value.tags,
      siteName: currentSourceSite.value || undefined,
    })
    const result = resp.data?.data
    if (result) {
      seedReviewed.value = result.reviewed || false
      seedMissingFields.value = result.missing_fields || []
      // §59.28 C（方案A ④）: 服务端渲染的完整描述（声明+致谢+海报+正文+截图）
      if (result.rendered_description) {
        previewRenderedDesc.value = parseBBCode(result.rendered_description)
        previewDescSource.value = result.rendered_description
      } else {
        previewRenderedDesc.value = parseBBCode(form.value.description)
        previewDescSource.value = form.value.description
      }
      // §59.81: 预览增强素材——全字段/标签/产地类型/声明段
      previewFieldsData.value = { ...result } as Record<string, unknown>
      previewStatement.value = (result as { statement?: string }).statement || form.value.statement
      // §59.28 C（方案A ②）: 标准化重组标题回填预览
      if (result.reassembled_title) {
        form.value.title = result.reassembled_title
      }
      // §59.135: PUT result 技术字段回填 form.titleComponents（与 GET 同源 §59.103）——
      // Tab1 与预览②同读一份（预览只消费 Tab 数据）；顺带修保存后 Tab1 不刷新
      const r = result as Record<string, any>
      form.value.titleComponents = {
        ...form.value.titleComponents,
        main_title: r.main_title || '',
        season_episode: r.season_episode || '',
        year: r.year || '',
        release_group: r.release_group || '',
        chinese_prefix: r.chinese_prefix || '',
        resolution: r.resolution || '',
        video_codec: r.video_codec || '',
        audio_codec: r.audio_codec || '',
        audio_channels: r.audio_channels || '',
        audio_technology: r.audio_tech || '',
        audio_tracks: r.audio_tracks ? String(r.audio_tracks) : '',
        hdr: r.hdr || '',
        bit_depth: r.bit_depth || '',
        source_type: r.source_type || '',
        specification: r.specification || '',
        source_platform: r.source_platform || '',
        edition_info: r.edition_info || '',
        region_code: r.region_code || '',
        category: r.category || '',
      }
      seedEncode.value = r.encode ?? seedEncode.value
      if (r.region?.labels) form.value.region = r.region.labels
      if (r.genre?.labels) form.value.genre = r.genre.labels
    }
    seedPreviewMode.value = true
    // §59.92: 完成门槛——挂滚动监听 + 内容不足直接放开
    previewScrolled.value = false
    attachPreviewScrollGate()
  } catch (e: unknown) {
    message.error('保存失败: ' + (e as Error).message)
  } finally {
    saving.value = false
  }
}

// §59.20 ⑨: 确认完成——数据已在预览时存好，直接关闭
function confirmDone() {
  emit('success')
  emit('update:open', false)
}

// §59.20 ⑨: 返回编辑
function backToEdit() {
  seedPreviewMode.value = false
  previewScrolled.value = false
  detachPreviewScrollGate()
}



// §59.51: 组件卸载清轮询
onUnmounted(() => {
  if (capturePollTimer) { clearInterval(capturePollTimer); capturePollTimer = null }
  detachPreviewScrollGate()
})
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
