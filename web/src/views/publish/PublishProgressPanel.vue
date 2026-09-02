<template>
  <div class="pp-panel">
    <!-- 进度态（一站多种轮询任务） -->
    <template v-if="progress">
      <a-progress :percent="percent" :format="() => `${progress?.done ?? 0}/${progress?.total ?? 0}`" status="active" />
      <p class="pp-current">
        正在发布 {{ progress.done + 1 }}/{{ progress.total }}
        <span v-if="progress.currentTitle">：{{ progress.currentTitle }}</span>
        <a-spin size="small" style="margin-left: 8px" />
      </p>
    </template>

    <!-- 完成态（一种多站/一站多种共用） -->
    <template v-else>
      <a-alert :type="alertType" show-icon style="margin-bottom: 12px">
        <template #message>
          <span style="font-size: 15px; font-weight: 600">
            发布成功 {{ okCount }}<span v-if="rowMode === 'site'"> 站</span><span v-else> 种</span><span v-if="existCount"> · 已存在 {{ existCount }}</span><span v-if="failCount"> · 失败 {{ failCount }}</span>
          </span>
        </template>
        <template #description>
          逐{{ rowMode === 'site' ? '站' : '种' }}明细（{{ results.length }}）——失败原因见各行/发布日志
        </template>
      </a-alert>
      <div v-for="(r, i) in results" :key="i" class="pp-row">
        <template v-if="rowMode === 'site'">
          <a-tag :color="rowColor(r.status)">{{ r.site || r.title }}</a-tag>
          <span>{{ rowLabel(r.status) }}</span>
        </template>
        <template v-else>
          <span class="pp-title">{{ r.title || r.info_hash }}</span>
          <a-tag :color="rowColor(r.status)" style="margin: 0 8px">{{ rowLabel(r.status) }}</a-tag>
        </template>
        <span v-if="r.message" class="pp-muted">{{ r.message.slice(0, 80) }}</span>
        <a v-if="r.url" :href="r.url" target="_blank" style="font-size: 12px">详情</a>
      </div>
      <div class="pp-actions">
        <a-button @click="$router.push('/publish/logs')">查看发布日志</a-button>
        <a-button type="primary" @click="emit('done')">完成</a-button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
// §59.166 PublishProgressPanel 发布进度/汇总公共组件（一种多站 Modal 完成态 +
// 一站多种弹窗进度态/完成态——§59.163 三分类交互统一）。
export interface PPResultRow {
  info_hash?: string
  title?: string
  site?: string
  status: string
  message?: string
  url?: string
}

const props = defineProps<{
  progress?: { done: number; total: number; currentTitle?: string } | null
  results: PPResultRow[]
  rowMode: 'site' | 'seed'
}>()

const emit = defineEmits<{ (e: 'done'): void }>()

const percent = computed(() => {
  if (!props.progress) return 0
  return Math.round((props.progress.done / Math.max(props.progress.total, 1)) * 100)
})

const okStatuses = ['pushed', 'pushed_existing']
const existStatuses = ['duplicate', 'existing']
const okCount = computed(() => props.results.filter(r => okStatuses.includes(r.status)).length)
const existCount = computed(() => props.results.filter(r => existStatuses.includes(r.status)).length)
const failCount = computed(() => props.results.filter(r => r.status === 'failed').length)
const alertType = computed(() => (failCount.value > 0 ? 'warning' : 'success'))

const STATUS_LABEL: Record<string, string> = {
  pushed: '发布成功', pushed_existing: '已推种', duplicate: '站上已有', existing: '站上已有', failed: '失败',
}
function rowLabel(st: string): string {
  return STATUS_LABEL[st] || st
}
function rowColor(st: string): string {
  if (okStatuses.includes(st)) return 'success'
  if (existStatuses.includes(st)) return 'warning'
  return 'error'
}
</script>

<style scoped>
.pp-current {
  margin: 8px 0 0;
  color: #555;
}
.pp-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  border-bottom: 1px dashed #f0f0f0;
}
.pp-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pp-muted {
  color: #999;
  font-size: 12px;
}
.pp-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 16px;
}
</style>
