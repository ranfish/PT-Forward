<template>
  <div class="tag-selector">
    <!-- 已选标签（只读展示；删除在弹窗内） -->
    <div v-if="modelValue.length" class="selected-tags">
      <a-tag
        v-for="tag in modelValue"
        :key="tag"
        :color="isRestrictedTag(tag) ? 'red' : isStandardTag(tag) ? 'blue' : 'default'"
      >
        {{ tagLabel(tag) }}
      </a-tag>
    </div>
    <div v-else class="empty-hint">未选择标签</div>

    <!-- §59.85: 编辑标签按钮弹窗形态（折叠面板收进弹窗，Tab1 不再常驻展开） -->
    <a-button size="small" style="margin-top: 8px" @click="modalOpen = true">
      <EditOutlined /> 编辑标签
    </a-button>
    <a-modal
      v-model:open="modalOpen"
      title="编辑标签"
      width="760px"
      ok-text="确定"
      cancel-text="取消"
      :z-index="1100"
    >
    <!-- 自定义标签输入 -->
    <a-input
      v-model:value="customInput"
      placeholder="输入自定义标签后回车（如 禁转/首发）"
      style="margin-bottom: 8px"
      @press-enter="addCustomTag"
    >
      <template #suffix>
        <a-button size="small" type="link" :disabled="!customInput.trim()" @click="addCustomTag">
          添加
        </a-button>
      </template>
    </a-input>

    <!-- 已选标签（弹窗内可删除） -->
    <div v-if="modelValue.length" class="selected-tags" style="margin-bottom: 8px">
      <a-tag
        v-for="tag in modelValue"
        :key="tag"
        closable
        :color="isRestrictedTag(tag) ? 'red' : isStandardTag(tag) ? 'blue' : 'default'"
        @close="removeTag(tag)"
      >
        {{ tagLabel(tag) }}
      </a-tag>
    </div>

    <!-- 标准标签分组多选（dict 56 词条 9 分组） -->
    <a-collapse v-model:active-key="activeGroups" :bordered="false" ghost>
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
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { CheckCircleFilled, EditOutlined } from '@ant-design/icons-vue'
import { TAG_GROUPS } from '@/generated/dict'
import { tagDisplayName as tagDisplayNameCommon } from '@/utils/tagDisplay'

// §59.35 P4: tag 分组数据源切换 generated/dict.ts（dict/tag.json 唯一真相源，
// 原前端 38 组硬编码副本删除）
const tagGroups = TAG_GROUPS

// 所有标准 tag key（用于判断是否标准）
const allStandardKeys = computed(() => tagGroups.flatMap(g => g.tags.map(t => t.key)))

const props = defineProps<{
  modelValue: string[]
  displayLabels?: string[] | null // §59.106: 后端权威显示名（索引对齐 modelValue; null=本地 dict 映射）
}>()
const emit = defineEmits<{
  'update:modelValue': [value: string[]]
}>()

const customInput = ref('')
const activeGroups = ref<string[]>(['HDR/色彩'])  // 默认展开第一组
const modalOpen = ref(false)
// §59.85: 禁转类标签红色（easy-upload getTagType 借鉴）
const isRestrictedTag = (t: string): boolean =>
  t === '禁转' || t === 'tag.禁转' || t === '限转' || t === 'tag.限转' || t === 'no_transfer'


function isStandardTag(tag: string): boolean {
  return allStandardKeys.value.includes(tag)
}

// §59.32: 已选区显示通用标签文字（label），自定义标签显示原文
function tagLabel(tag: string): string {
  // §59.110: 公共单点（utils/tagDisplay）——后端权威优先/本地 dict/原文兜底
  return tagDisplayNameCommon(tag, props.modelValue, props.displayLabels)
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
