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
        {{ tag }}
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

// v0.0.256 §56.22 38 个 media_tag standard_keys（按类别分组）
// 数据源：DB standard_keys 表 + titleparser/data/standard_keys.json
interface MediaTag {
  key: string   // standard_key（如 hdr10）
  label: string // 显示名（如 "HDR10"）
  aliases: string // 别名提示（如 "HDR 10"）
}

interface TagGroup {
  name: string
  tags: MediaTag[]
}

const tagGroups: TagGroup[] = [
  {
    name: 'HDR/色彩',
    tags: [
      { key: 'hdr10', label: 'HDR10', aliases: 'HDR 10' },
      { key: 'hdr10_plus', label: 'HDR10+', aliases: '' },
      { key: 'dolby_vision', label: 'Dolby Vision', aliases: 'DV/DoVi' },
      { key: 'hdr_vivid', label: 'HDR Vivid', aliases: '国标 HDR' },
      { key: 'hlg', label: 'HLG', aliases: '' },
      { key: '10_bit', label: '10bit', aliases: '10-bit 色深' },
    ],
  },
  {
    name: '音频编码',
    tags: [
      { key: 'dolby_atmos', label: 'Dolby Atmos', aliases: '全景声' },
      { key: 'dts_x', label: 'DTS:X', aliases: '' },
      { key: 'lossless', label: '无损', aliases: '' },
      { key: 'lossy', label: '有损', aliases: '' },
    ],
  },
  {
    name: '语言音轨',
    tags: [
      { key: 'chinese_audio', label: '国语', aliases: '普通话/国配' },
      { key: 'cantonese_audio', label: '粤语', aliases: '' },
      { key: 'japanese_audio', label: '日语', aliases: '原声' },
      { key: 'korean_audio', label: '韩语', aliases: '' },
      { key: 'original_audio', label: '原声', aliases: '' },
      { key: 'dubbed', label: '配音', aliases: 'Dub' },
    ],
  },
  {
    name: '字幕',
    tags: [
      { key: 'chinese_subtitle', label: '中字', aliases: 'CHS/简繁' },
      { key: 'english_subtitle', label: '英字', aliases: 'ENG' },
      { key: 'hardcoded_subs', label: '硬字幕', aliases: '硬字' },
      { key: 'encoded_subs', label: '内嵌字幕', aliases: '' },
      { key: 'external_subtitles', label: '外挂字幕', aliases: '' },
      { key: 'subtitles_include', label: '含字幕', aliases: '' },
    ],
  },
  {
    name: '版本类型',
    tags: [
      { key: 'diy', label: 'DIY', aliases: '' },
      { key: 'scene', label: 'Scene', aliases: 'Scene Release' },
      { key: 'remux', label: 'Remux', aliases: '' },
      { key: 'internal', label: 'Internal', aliases: 'iNTERNAL' },
      { key: 'exclusive', label: '独占', aliases: '专属' },
      { key: 'retail', label: 'Retail', aliases: '' },
      { key: 'web_release', label: 'WEB Release', aliases: '' },
      { key: 'promotional', label: '宣传版', aliases: 'Promo' },
      { key: 'hybrid', label: 'Hybrid', aliases: '' },
    ],
  },
  {
    name: '特别版',
    tags: [
      { key: 'special_edition', label: '特别版', aliases: 'Special Edition' },
      { key: 'director_s_cut', label: '导演剪辑', aliases: "Director's Cut" },
      { key: 'anniversary_edition', label: '纪念版', aliases: 'Anniversary' },
      { key: 'criterion', label: 'Criterion', aliases: 'CC' },
      { key: 'the_criterion_collection', label: 'CC 收藏', aliases: 'Criterion Collection' },
      { key: '4k_remaster', label: '4K Remaster', aliases: '4K 重制' },
      { key: '4k_restoration', label: '4K Restoration', aliases: '4K 修复' },
    ],
  },
  {
    name: '其他',
    tags: [
      { key: 'commentary', label: '评论音轨', aliases: 'Commentary' },
      { key: 'complete', label: '完结', aliases: '全集' },
    ],
  },
]

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
