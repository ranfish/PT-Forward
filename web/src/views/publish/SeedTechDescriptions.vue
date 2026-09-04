<template>
  <!-- §59.135: 技术规格表公共组件——Tab1 与预览②引用同一份（展示同件，§59.116 定案落地）。
       主/副标题由调用方自行展示（Tab1 span=3 行 / 预览①种子标识卡片）。 -->
  <a-descriptions :column="column" bordered size="small">
    <a-descriptions-item label="中文名">{{ tc.chinese_title || tc.chinese_prefix || '—' }}</a-descriptions-item>
    <a-descriptions-item label="译名">{{ tc.english_title || tc.main_title || '—' }}</a-descriptions-item>
    <a-descriptions-item label="季集">{{ tc.season_episode || '—' }}</a-descriptions-item>
    <a-descriptions-item label="年份">{{ tc.year || '—' }}</a-descriptions-item>
    <a-descriptions-item label="制作组">{{ tc.release_group || '—' }}</a-descriptions-item>
    <a-descriptions-item label="类型">{{ categoryLabel(tc.category) }}</a-descriptions-item>
    <!-- §59.82: 站点媒介合成——v1.05 source_type×specification 二维 → 站点单选视角 -->
    <a-descriptions-item label="媒介(站点)">{{ siteMediumDisplay }}</a-descriptions-item>
    <a-descriptions-item label="片源">{{ tc.source_type || '—' }}</a-descriptions-item>
    <!-- §59.34: v1.05 Encode 规格为空——规格栏 Encode 派生兜底（后端真相源） -->
    <a-descriptions-item label="规格">{{ specDisplay }}</a-descriptions-item>
    <a-descriptions-item label="分发方">
      {{ tc.source_platform || '—' }}
      <a-tooltip v-if="PLATFORM_FULLNAMES[tc.source_platform]" :title="PLATFORM_FULLNAMES[tc.source_platform]">
        <InfoCircleOutlined style="color: #999; margin-left: 4px" />
      </a-tooltip>
    </a-descriptions-item>
    <a-descriptions-item label="分辨率">{{ tc.resolution || '—' }}</a-descriptions-item>
    <a-descriptions-item label="视频编码">{{ tc.video_codec || '—' }}</a-descriptions-item>
    <a-descriptions-item label="HDR">{{ tc.hdr || '—' }}</a-descriptions-item>
    <a-descriptions-item label="bit">{{ tc.bit_depth || '—' }}</a-descriptions-item>
    <a-descriptions-item label="音频编码">{{ tc.audio_codec || '—' }}</a-descriptions-item>
    <a-descriptions-item label="声道">{{ tc.audio_channels || '—' }}</a-descriptions-item>
    <a-descriptions-item label="音频技术">{{ tc.audio_technology || '—' }}</a-descriptions-item>
    <a-descriptions-item label="音轨数">{{ tc.audio_tracks || '—' }}</a-descriptions-item>
    <a-descriptions-item label="版本">{{ tc.edition_info || '—' }}</a-descriptions-item>
    <a-descriptions-item label="地区码">{{ tc.region_code || '—' }}</a-descriptions-item>
  </a-descriptions>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { InfoCircleOutlined } from '@ant-design/icons-vue'
import { CATEGORY_LABELS, PLATFORM_FULLNAMES } from '@/generated/dict'

const props = withDefaults(defineProps<{
  tc: Record<string, string>
  encode?: boolean
  column?: number
}>(), {
  encode: false,
  column: 3,
})

// §59.87: 站点媒介合成（§59.83 v1.05 片源写法区分——连字符=原盘媒介, 无连字符=压制）
const siteMediumDisplay = computed(() => {
  const tc = props.tc
  const spec = (tc.specification || '').toLowerCase()
  const st = tc.source_type || ''
  if (spec === 'remux') return 'Remux'
  if (spec === 'web-dl' || spec === 'webdl') return 'WEB-DL'
  if (spec === 'webrip') return 'WEBRip'
  if (spec === 'hdtv') return 'HDTV'
  if (spec === 'uhdtv') return 'UHDTV'
  if (spec === 'bdrip' || spec === 'dvdrip') return 'Encode'
  if (st === 'UHD Blu-ray' || st === 'Blu-ray' || st === '3D Blu-ray') {
    return st + ' 原盘'
  }
  if (st === 'UHD BluRay' || st === 'BluRay' || st === '3D BluRay') {
    return 'Encode'
  }
  if (st.includes('DVD') && !st.includes('Rip')) return 'DVD'
  if (props.encode) return 'Encode'
  return '—'
})

// §59.34: Encode 派生兜底——规格空且 encode=true → 显示 Encode
const specDisplay = computed(() =>
  props.tc.specification || (props.encode ? 'Encode' : '—')
)

// §59.35 P3: 分级 label——Layer 1 字典优先（generated/dict.ts，与后端同源），
// 扩展分类（adapter 源站直传的 category.mv/game/software 等）本地兜底
const extendedCategoryLabels: Record<string, string> = {
  'category.mv': 'MV',
  'category.audiobook': '有声读物',
  'category.ebook': '电子书',
  'category.game': '游戏',
  'category.software': '软件',
}
function categoryLabel(v?: string): string {
  if (!v) return '—'
  return CATEGORY_LABELS[v] || extendedCategoryLabels[v] || v
}
</script>
