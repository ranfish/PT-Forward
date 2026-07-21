<template>
  <div class="step-content">
    <a-result
      v-if="submitError"
      status="error"
      title="发布失败"
      :sub-title="submitError"
      style="padding: 40px 0"
    >
      <template #extra>
        <a-button @click="emit('back')">返回重试</a-button>
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
          <!-- v0.0.255 §56.30: 加种状态显示 -->
          <div v-if="site.seeded === true" class="result-site-card-seed seeded">
            <CheckCircleFilled /> 已加种
          </div>
          <div v-else-if="site.seeded === false && site.seedError" class="result-site-card-seed failed">
            <WarningFilled /> 加种失败
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, markRaw } from 'vue'
import {
  CheckCircleFilled, WarningFilled, ClockCircleOutlined,
  LoadingOutlined, CloseCircleFilled, StopOutlined,
} from '@ant-design/icons-vue'
import type { PublishResultRecord } from '@/api/types'

export interface CandidateStatus {
  done_count?: number
  fail_count?: number
  total_count?: number
  publish_status?: string
}

interface ResultSiteStatus {
  name: string
  status: 'queued' | 'publishing' | 'done' | 'failed' | 'skipped'
  label: string
  tagColor: string
  icon: ReturnType<typeof markRaw>
  seeded?: boolean
  seedError?: string
}

const props = defineProps<{
  submitError: string
  submittedCandidateId: number
  candidateStatus: CandidateStatus | null
  selectedTargets: string[]
  resultRecords: Record<string, PublishResultRecord>
}>()

const emit = defineEmits<{
  back: []
}>()

const publishPercent = computed(() => {
  if (!props.candidateStatus || !props.candidateStatus.total_count) return 0
  const done = props.candidateStatus.done_count || 0
  const total = props.candidateStatus.total_count
  return Math.round((done / total) * 100)
})

function inferSiteStatus(_name: string): ResultSiteStatus['status'] {
  if (!props.candidateStatus) return 'queued'
  if (props.candidateStatus.publish_status === 'done') return 'done'
  if (props.candidateStatus.publish_status === 'failed') return 'failed'
  if (props.candidateStatus.publish_status === 'skipped') return 'skipped'
  if (props.candidateStatus.publish_status === 'publishing') return 'publishing'
  return 'queued'
}

const resultSiteStatus = computed<ResultSiteStatus[]>(() => {
  return props.selectedTargets.map(name => {
    const status = inferSiteStatus(name)
    const cfg: Record<string, { label: string; tagColor: string; icon: ReturnType<typeof markRaw> }> = {
      queued:      { label: '排队中', tagColor: 'blue',   icon: markRaw(ClockCircleOutlined) },
      publishing:  { label: '发布中', tagColor: 'processing', icon: markRaw(LoadingOutlined) },
      done:        { label: '已完成', tagColor: 'green',  icon: markRaw(CheckCircleFilled) },
      failed:      { label: '失败',   tagColor: 'red',    icon: markRaw(CloseCircleFilled) },
      skipped:     { label: '已跳过', tagColor: 'default',icon: markRaw(StopOutlined) },
    }
    const c = cfg[status] || cfg.queued
    // v0.0.255 §56.30: 从 resultRecords 取该站的加种状态
    const record = props.resultRecords[name]
    return {
      name, status, label: c.label, tagColor: c.tagColor, icon: c.icon,
      seeded: record?.seeded,
      seedError: record?.seed_error,
    }
  })
})
</script>
