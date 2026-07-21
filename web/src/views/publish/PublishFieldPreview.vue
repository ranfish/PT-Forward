<template>
  <div class="field-preview">
    <!-- 头部：目标站 + 完整度 -->
    <div class="preview-header">
      <div class="preview-header-title">
        <a-tag color="blue">{{ targetSite }}</a-tag>
        <span class="preview-mode">{{ modeLabel }}</span>
      </div>
      <div v-if="completeness" class="completeness-bar">
        <a-progress
          :percent="completeness.percent"
          :stroke-color="completenessColor"
          size="small"
          style="width: 120px"
        />
        <span class="completeness-text" :style="{ color: completenessColor }">
          {{ completeness.filled }}/{{ completeness.total }}
          <span v-if="completeness.missing > 0" style="color: #ff4d4f">
            （缺 {{ completeness.missing }}）
          </span>
        </span>
      </div>
    </div>

    <!-- 加载中 -->
    <div v-if="loading" class="preview-loading">
      <a-spin size="small" />
      <span style="margin-left: 8px; color: #999">加载字段预览...</span>
    </div>

    <!-- 加载失败 -->
    <a-alert
      v-else-if="error"
      type="warning"
      show-icon
      :message="error"
      style="margin: 8px 0"
    />

    <!-- 字段列表 -->
    <div v-else-if="fields.length" class="field-list">
      <div
        v-for="field in fields"
        :key="field.standard_key"
        class="field-item"
        :class="{ 'field-missing': field.missing, 'field-edited': field.is_user_edited }"
      >
        <div class="field-label">
          <span class="field-name">{{ field.label }}</span>
          <span v-if="field.required" class="field-required">*</span>
          <!-- 来源徽标（4 色） -->
          <span class="field-source" :class="`source-${field.source}`">
            {{ sourceLabel(field.source) }}
          </span>
        </div>
        <div class="field-value">
          <!-- 缺失（必填空） -->
          <span v-if="field.missing" class="value-missing">
            ⚠ 必填字段缺失
          </span>
          <!-- 有值：显示 target_value（reverse mapping 后） -->
          <template v-else>
            <span class="value-text" :title="field.target_value">
              {{ truncate(field.target_value, 80) }}
            </span>
            <!-- diff：用户编辑或 mapping 改变时显示原值 -->
            <span
              v-if="field.is_user_edited || (field.source_value && field.source_value !== field.target_value)"
              class="value-diff"
              :title="`原始: ${field.source_value}`"
            >
              ← {{ truncate(field.source_value, 30) }}
            </span>
          </template>
        </div>
      </div>
    </div>

    <!-- 空状态 -->
    <a-empty
      v-else
      description="暂无字段预览数据"
      :image="Empty.PRESENTED_IMAGE_SIMPLE"
      style="padding: 20px 0"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Empty } from 'ant-design-vue'
import type { PreviewField, PreviewCompleteness } from '@/api/types'

const props = defineProps<{
  targetSite: string
  mode: string
  fields: PreviewField[]
  completeness: PreviewCompleteness | null
  loading: boolean
  error: string
}>()

const modeLabel = computed(() => {
  switch (props.mode) {
    case 'ptgen_first': return 'PTGen 优先'
    case 'detail_first': return '详情页优先'
    default: return props.mode || '-'
  }
})

const completenessColor = computed(() => {
  if (!props.completeness) return '#1677ff'
  const p = props.completeness.percent
  if (p >= 90) return '#52c41a'
  if (p >= 70) return '#faad14'
  return '#ff4d4f'
})

function sourceLabel(source: string): string {
  switch (source) {
    case 'ptgen': return 'PTGen'
    case 'detail': return '详情'
    case 'local': return '本地'
    case 'user': return '用户'
    default: return source || '-'
  }
}

function truncate(s: string, n: number): string {
  if (!s) return ''
  return s.length > n ? s.slice(0, n) + '...' : s
}
</script>

<style scoped>
.field-preview {
  border: 1px solid #f0f0f0;
  border-radius: 6px;
  padding: 12px;
  background: #fafafa;
  font-size: 12px;
  max-height: 600px;
  overflow-y: auto;
}
.preview-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
  padding-bottom: 8px;
  border-bottom: 1px solid #f0f0f0;
}
.preview-header-title {
  display: flex;
  align-items: center;
  gap: 6px;
}
.preview-mode {
  color: #999;
  font-size: 11px;
}
.completeness-bar {
  display: flex;
  align-items: center;
  gap: 6px;
}
.completeness-text {
  font-size: 11px;
  font-weight: 600;
}
.preview-loading {
  padding: 20px 0;
  text-align: center;
}
.field-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.field-item {
  padding: 6px 8px;
  border-radius: 4px;
  background: #fff;
  border: 1px solid #f0f0f0;
  transition: all 0.2s;
}
.field-item.field-missing {
  background: #fff2f0;
  border-color: #ffccc7;
}
.field-item.field-edited {
  background: #f6ffed;
  border-color: #b7eb8f;
}
.field-label {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-bottom: 2px;
}
.field-name {
  font-weight: 600;
  color: #333;
}
.field-required {
  color: #ff4d4f;
}
.field-source {
  margin-left: auto;
  padding: 0 6px;
  border-radius: 8px;
  font-size: 10px;
  color: #fff;
}
.source-ptgen { background: #1677ff; }
.source-detail { background: #722ed1; }
.source-local { background: #52c41a; }
.source-user { background: #fa8c16; }
.field-value {
  color: #555;
  word-break: break-all;
}
.value-text {
  color: #333;
}
.value-missing {
  color: #ff4d4f;
  font-weight: 600;
}
.value-diff {
  display: inline-block;
  margin-left: 6px;
  color: #999;
  font-size: 11px;
  font-style: italic;
}
</style>
