<template>
  <div class="tag-selector">
    <!-- 已选标签 -->
    <div v-if="modelValue.length" class="selected-tags">
      <a-tag
        v-for="tag in modelValue"
        :key="tag"
        closable
        :color="isStandardTag(tag) ? 'blue' : 'default'"
        @close="removeTag(tag)"
      >
        {{ tagLabel(tag) }}
      </a-tag>
    </div>
    <div v-else class="empty-hint">未选择标签</div>

    <!-- 自定义标签输入 -->
    <a-input
      v-model:value="customInput"
      placeholder="输入自定义标签后回车（如 禁转/首发）"
      style="margin-top: 8px"
      @press-enter="addCustomTag"
    >
      <template #suffix>
        <a-button size="small" type="link" :disabled="!customInput.trim()" @click="addCustomTag">
          添加
        </a-button>
      </template>
    </a-input>

    <!-- 38 个标准 media_tag 分组多选 -->
    <a-collapse v-model:active-key="activeGroups" :bordered="false" ghost style="margin-top: 8px">
      <a-collapse-panel
        v-for="group in tagGroups"
        :key="group.name"
        :header="`${group.name}（${group.tags.length}）`"
      >
        <div class="tag-grid">
          <div
            v-for="tag in group.tags"
            :key="tag.key"
            class="tag-btn"
            :class="{ selected: modelValue.includes(tag.key) }"
            @click="toggleTag(tag.key)"
          >
            <CheckCircleFilled v-if="modelValue.includes(tag.key)" class="tag-btn-check" />
            <span class="tag-btn-label">{{ tag.label }}</span>
            <span v-if="tag.aliases" class="tag-btn-aliases">{{ tag.aliases }}</span>
          </div>
        </div>
      </a-collapse-panel>
    </a-collapse>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { CheckCircleFilled } from '@ant-design/icons-vue'
import { TAG_GROUPS } from '@/generated/dict'

// §59.35 P4: tag 分组数据源切换 generated/dict.ts（dict/tag.json 唯一真相源，
// 原前端 38 组硬编码副本删除）
const tagGroups = TAG_GROUPS

// 所有标准 tag key（用于判断是否标准）
const allStandardKeys = computed(() => tagGroups.flatMap(g => g.tags.map(t => t.key)))

const props = defineProps<{
  modelValue: string[]
}>()
const emit = defineEmits<{
  'update:modelValue': [value: string[]]
}>()

const customInput = ref('')
const activeGroups = ref<string[]>(['HDR/色彩'])  // 默认展开第一组

function isStandardTag(tag: string): boolean {
  return allStandardKeys.value.includes(tag)
}

// §59.32: 已选区显示通用标签文字（label），自定义标签显示原文
function tagLabel(tag: string): string {
  for (const g of tagGroups) {
    for (const t of g.tags) {
      if (t.key === tag) return t.label
    }
  }
  return tag
}

function toggleTag(key: string) {
  const current = [...props.modelValue]
  const idx = current.indexOf(key)
  if (idx >= 0) {
    current.splice(idx, 1)
  } else {
    current.push(key)
  }
  emit('update:modelValue', current)
}

function removeTag(tag: string) {
  const current = props.modelValue.filter(t => t !== tag)
  emit('update:modelValue', current)
}

function addCustomTag() {
  const tag = customInput.value.trim()
  if (!tag) return
  if (props.modelValue.includes(tag)) {
    customInput.value = ''
    return
  }
  emit('update:modelValue', [...props.modelValue, tag])
  customInput.value = ''
}
</script>

<style scoped>
.tag-selector {
  width: 100%;
}
.selected-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  min-height: 24px;
  padding: 6px 8px;
  background: #fafafa;
  border: 1px solid #f0f0f0;
  border-radius: 4px;
}
.empty-hint {
  color: #999;
  font-size: 12px;
  padding: 6px 8px;
}
.tag-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 6px;
}
.tag-btn {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 8px;
  border: 1px solid #d9d9d9;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.2s;
  font-size: 12px;
  background: #fff;
}
.tag-btn:hover {
  border-color: #1677ff;
  background: #f0f5ff;
}
.tag-btn.selected {
  border-color: #1677ff;
  background: #e6f4ff;
  color: #1677ff;
}
.tag-btn-check {
  color: #1677ff;
  font-size: 12px;
}
.tag-btn-label {
  font-weight: 600;
}
.tag-btn-aliases {
  color: #999;
  font-size: 11px;
  margin-left: auto;
}
</style>
