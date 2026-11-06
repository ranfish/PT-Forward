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
    <div class="mdp-shell">
      <!-- ═══ Header ═══ -->
      <div class="mdp-header">
        <div class="mdp-header-left">
          <span class="mdp-header-title">音乐详情</span>
          <a-tag v-if="music" color="purple">{{ music.artist || '未知艺术家' }}</a-tag>
          <a-tag v-if="detail?.site_name" color="blue">{{ detail.site_name }}</a-tag>
        </div>
        <div class="mdp-header-right">
          <!-- §59.475: 审核按钮——发布流程依赖 reviewed（音乐门=title+artist+album） -->
          <a-tag v-if="isReviewed" color="green" style="margin-right: 8px">✓ 已审核</a-tag>
          <a-button
            v-else
            type="primary"
            size="small"
            :loading="reviewLoading"
            @click="confirmReview"
          >
            确认完成
          </a-button>
          <a-button size="small" @click="handleClose">关闭</a-button>
        </div>
      </div>

      <!-- ═══ Body ═══ -->
      <div class="mdp-body">
        <!-- 空态防御（入口层保证 music 在场才开·此处兜底） -->
        <a-result v-if="!music" status="warning" title="无音乐元数据（该种子不是音乐种或获取未完成）">
          <template #extra>
            <a-button @click="handleClose">关闭</a-button>
          </template>
        </a-result>

        <div v-else class="mdp-content">
          <!-- ═══ ① 头部通用槽：标题·来源站·大小·InfoHash·封面 ═══ -->
          <a-card size="small" style="margin-bottom: 12px">
            <div class="mdp-head-slot">
              <div class="mdp-head-main">
                <a-descriptions :column="1" size="small">
                  <a-descriptions-item label="标题">
                    <span style="font-size: 15px; font-weight: 600">{{ detail?.title || preset?.name || '—' }}</span>
                  </a-descriptions-item>
                  <a-descriptions-item v-if="detail?.subtitle" label="副标题">
                    <span style="color: #666">{{ detail.subtitle }}</span>
                  </a-descriptions-item>
                  <a-descriptions-item label="来源站">{{ detail?.site_name || '—' }}</a-descriptions-item>
                  <a-descriptions-item label="大小">{{ formatBytes(preset?.size ?? 0) }}</a-descriptions-item>
                  <a-descriptions-item label="InfoHash">
                    <span style="font-family: monospace; font-size: 12px">{{ preset?.info_hash || detail?.info_hash || '—' }}</span>
                  </a-descriptions-item>
                  <a-descriptions-item v-if="detail?.fetched_at" label="获取时间">
                    {{ detail.fetched_at.replace('T', ' ').slice(0, 19) }}
                  </a-descriptions-item>
                </a-descriptions>
              </div>
              <div v-if="detail?.poster" class="mdp-head-poster">
                <a-image :src="detail.poster" :width="180" :fallback="fallbackPoster" style="border-radius: 4px" />
              </div>
            </div>
          </a-card>

          <!-- ═══ ② 基本信息 ═══ -->
          <a-card size="small" style="margin-bottom: 12px">
            <template #title><div class="mdp-card-title" style="background: #13c2c2">② 基本信息</div></template>
            <a-descriptions :column="3" size="small">
              <a-descriptions-item label="艺术家">{{ music.artist || '—' }}</a-descriptions-item>
              <a-descriptions-item label="专辑">{{ music.album || '—' }}</a-descriptions-item>
              <a-descriptions-item label="年份">{{ music.year || '—' }}</a-descriptions-item>
              <a-descriptions-item label="发行类型">{{ music.release_type || '—' }}</a-descriptions-item>
              <a-descriptions-item label="流派">{{ music.genre || '—' }}</a-descriptions-item>
              <a-descriptions-item label="版本">
                <span :style="music.edition ? '' : 'color: #999'">{{ music.edition || '—（B3 预留）' }}</span>
              </a-descriptions-item>
              <a-descriptions-item label="厂牌">
                <span :style="music.label ? '' : 'color: #999'">{{ music.label || '—（B2 预留）' }}</span>
              </a-descriptions-item>
              <a-descriptions-item label="唱片编号">
                <span :style="music.catalogue_no ? '' : 'color: #999'">{{ music.catalogue_no || '—（B2 预留）' }}</span>
              </a-descriptions-item>
            </a-descriptions>
          </a-card>

          <!-- ═══ ③ 规格 ═══ -->
          <a-card size="small" style="margin-bottom: 12px">
            <template #title><div class="mdp-card-title" style="background: #1d39c4">③ 规格</div></template>
            <a-descriptions :column="3" size="small">
              <a-descriptions-item label="格式">{{ music.format || '—' }}</a-descriptions-item>
              <a-descriptions-item label="整/分轨">{{ music.audio_mode || '—' }}</a-descriptions-item>
              <a-descriptions-item label="媒介">{{ music.media || '—' }}</a-descriptions-item>
              <a-descriptions-item label="位深">
                <span :style="music.bit_depth ? '' : 'color: #999'">{{ music.bit_depth || '—（B1 预留）' }}</span>
              </a-descriptions-item>
              <a-descriptions-item label="采样率">
                <span :style="music.sample_rate ? '' : 'color: #999'">{{ music.sample_rate || '—（B1 预留）' }}</span>
              </a-descriptions-item>
              <a-descriptions-item label="Log 得分">
                <template v-if="music.log_score">{{ music.log_score }} / 100</template>
                <span v-else style="color: #999">—</span>
              </a-descriptions-item>
              <a-descriptions-item label="Log">
                <a-tag v-if="music.has_log" color="green" style="margin: 0">有</a-tag>
                <span v-else style="color: #999">—</span>
              </a-descriptions-item>
              <a-descriptions-item label="Cue">
                <a-tag v-if="music.has_cue" color="green" style="margin: 0">有</a-tag>
                <span v-else style="color: #999">—</span>
              </a-descriptions-item>
            </a-descriptions>
          </a-card>

          <!-- ═══ ④ 转载信息（信号区） ═══ -->
          <a-card size="small" style="margin-bottom: 12px">
            <template #title><div class="mdp-card-title" style="background: #fa8c16">④ 转载信息</div></template>
            <a-descriptions :column="2" size="small">
              <a-descriptions-item label="转载规则">
                <a-tag v-if="music.share_rule" :color="shareRuleColor(music.share_rule)" style="margin: 0">{{ music.share_rule }}</a-tag>
                <span v-else style="color: #999">—</span>
              </a-descriptions-item>
              <a-descriptions-item label="其它版本">
                <span v-if="music.other_versions">{{ music.other_versions }}</span>
                <span v-else style="color: #999">—</span>
              </a-descriptions-item>
              <a-descriptions-item label="来自" :span="2">
                <a v-if="music.origin" :href="music.origin" target="_blank" rel="noopener" style="word-break: break-all; font-size: 12px">{{ music.origin }}</a>
                <span v-else style="color: #999">—</span>
              </a-descriptions-item>
            </a-descriptions>
          </a-card>

          <!-- ═══ ⑤ 长内容（折叠） ═══ -->
          <a-card size="small" style="margin-bottom: 12px">
            <template #title><div class="mdp-card-title" style="background: #595959">⑤ 长内容</div></template>
            <a-collapse :default-active-key="longActiveKeys" ghost>
              <a-collapse-panel v-if="music.track_list" key="track" header="曲目列表">
                <pre class="mdp-pre">{{ music.track_list }}</pre>
              </a-collapse-panel>
              <a-collapse-panel v-if="music.log_text" key="log" header="抓轨 Log">
                <pre class="mdp-pre">{{ music.log_text }}</pre>
              </a-collapse-panel>
              <a-collapse-panel v-if="detail?.description" key="desc" header="简介">
                <pre class="mdp-pre mdp-desc">{{ detail.description }}</pre>
              </a-collapse-panel>
            </a-collapse>
            <div v-if="!music.track_list && !music.log_text && !detail?.description" style="color: #999; padding: 8px 0">无长内容</div>
          </a-card>

          <!-- ═══ ⑥ 标签 ═══ -->
          <a-card size="small" style="margin-bottom: 12px">
            <template #title><div class="mdp-card-title" style="background: #722ed1">⑥ 标签</div></template>
            <div v-if="music.tags?.length">
              <a-tag v-for="t in music.tags" :key="t" color="purple" style="margin: 2px">{{ t }}</a-tag>
            </div>
            <div v-else style="color: #999">无标签</div>
          </a-card>
        </div>
      </div>
    </div>
  </a-drawer>
</template>

<script setup lang="ts">
// §59.469 附六: MDP（MusicDetailPanel）——音乐详情抽屉。
// 只读单页无 tab·契约 22 字段（§59.467 全景图）六分区渲染；
// 纯渲染零请求（detail prop=入口层 probeMusicSeed 结果直接传入——
// 音乐场景总请求恒 1 次·§59.469 附六方案 a）。
import { computed, ref } from 'vue'
import { message } from 'ant-design-vue'
import { formatBytes } from '@/utils/format'
import { seedConfigApi, type SeedDetail } from '@/api/publish'

const props = defineProps<{
  open: boolean
  detail: SeedDetail | null
  preset: { info_hash: string; name: string; size: number; client_id: number } | null
}>()

const emit = defineEmits<{ (e: 'update:open', v: boolean): void; (e: 'reviewed'): void }>()

const music = computed(() => props.detail?.music ?? null)

// 长内容折叠：曲目默认展开（快速浏览核心）·log/简介默认收起
const longActiveKeys = computed(() => (music.value?.track_list ? ['track'] : []))

// 转载规则着色：欢迎=绿·禁转族=红·其它=橙（信号区视觉分级）
function shareRuleColor(rule: string): string {
  if (rule.includes('欢迎')) return 'green'
  if (rule.includes('禁') || rule.includes('不得') || rule.includes('谢绝')) return 'red'
  return 'orange'
}

const fallbackPoster = 'data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIzNjAiIGhlaWdodD0iMzYwIj48cmVjdCB3aWR0aD0iMTAwJSIgaGVpZ2h0PSIxMDAlIiBmaWxsPSIjZjVmNWY1Ii8+PHRleHQgeD0iNTAlIiB5PSI1MCUiIGZvbnQtc2l6ZT0iMTQiIGZpbGw9IiM5OTkiIHRleHQtYW5jaG9yPSJtaWRkbGUiIGRvbWluYW50LWJhc2VsaW5lPSJtaWRkbGUiPua0u+eCuTwvdGV4dD48L3N2Zz4='

function handleClose() {
  emit('update:open', false)
}

// §59.475: 确认完成=审核（PUT reviewed=true·音乐门=title+artist+album）
const reviewLoading = ref(false)
const localReviewed = ref(false)
const isReviewed = computed(() => props.detail?.reviewed || localReviewed.value)

async function confirmReview() {
  const hash = props.preset?.info_hash
  if (!hash) return
  reviewLoading.value = true
  try {
    await seedConfigApi.putSeed(hash, { reviewed: true })
    localReviewed.value = true
    message.success('审核完成')
    emit('reviewed')
  } catch (e: unknown) {
    message.error('审核失败: ' + (e as Error).message)
  } finally {
    reviewLoading.value = false
  }
}
</script>

<style scoped>
.mdp-shell {
  display: flex;
  flex-direction: column;
  height: 100vh;
}
.mdp-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 24px;
  border-bottom: 1px solid #f0f0f0;
}
.mdp-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}
.mdp-header-title {
  font-size: 16px;
  font-weight: 600;
}
.mdp-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 24px;
}
.mdp-content {
  max-width: 1200px;
}
.mdp-head-slot {
  display: flex;
  gap: 24px;
  align-items: flex-start;
}
.mdp-head-main {
  flex: 1;
  min-width: 0;
}
.mdp-head-poster {
  flex-shrink: 0;
}
.mdp-card-title {
  font-size: 13px;
  font-weight: 600;
  padding: 2px 10px;
  border-radius: 3px;
  display: inline-block;
  color: #fff;
}
.mdp-pre {
  margin: 0;
  padding: 8px 12px;
  background: #fafafa;
  border: 1px solid #f0f0f0;
  border-radius: 4px;
  font-size: 12px;
  font-family: monospace;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 480px;
  overflow-y: auto;
}
.mdp-desc {
  font-family: inherit;
}
</style>
