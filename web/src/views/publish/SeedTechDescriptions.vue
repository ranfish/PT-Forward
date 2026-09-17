<template>
  <!-- §59.226 附二十一: 三分区表格——作品信息(PTGen 优先族)/发布规格(标题原样族)/
       技术参数(MI 铁证族)。分区表头三色视觉区分（审核者一眼分区）。
       主/副标题由调用方自行展示（Tab1 ①种子标识卡片 / 预览①种子标识）。 -->
  <!-- §59.246: 组件回裸三分区（单一职责——卡片包装+序号归调用方语境：
       Tab1"② 技术规格"/预览②各自包装，两区序号统一） -->
  <div class="std-sections">
    <!-- 分区一：作品信息（蓝） -->
    <a-descriptions
      :column="4" bordered size="small" :title="undefined"
      class="std-zone std-zone-work" layout="horizontal"
    >
      <template #title>
        <div class="zone-header zone-work">作品信息</div>
      </template>
      <a-descriptions-item label="片名" :span="2">{{ tc.chinese_title || tc.chinese_prefix || '—' }}</a-descriptions-item>
      <a-descriptions-item label="类别" :span="2">
        <template v-if="genreList.length">
          <a-tag v-for="g in genreList" :key="g" color="purple">{{ g }}</a-tag>
        </template>
        <template v-else>—</template>
      </a-descriptions-item>
      <a-descriptions-item label="译名" :span="4">{{ tc.english_title || tc.main_title || '—' }}</a-descriptions-item>
      <a-descriptions-item label="类型">{{ categoryLabel(tc.category) }}</a-descriptions-item>
      <a-descriptions-item label="年份">{{ tc.year || '—' }}</a-descriptions-item>
      <a-descriptions-item label="季集">{{ tc.season_episode || '—' }}</a-descriptions-item>
      <a-descriptions-item label="产地">
        <template v-if="regionList.length">
          <a-tag v-for="r in regionList" :key="r" color="geekblue">{{ r }}</a-tag>
        </template>
        <template v-else>—</template>
      </a-descriptions-item>
    </a-descriptions>

    <!-- 分区二：发布规格（绿） -->
    <a-descriptions :column="4" bordered size="small" class="std-zone">
      <template #title>
        <div class="zone-header zone-spec">发布规格</div>
      </template>
      <a-descriptions-item label="制作组">{{ tc.release_group || '—' }}</a-descriptions-item>
      <a-descriptions-item label="片源">{{ tc.source_type || '—' }}</a-descriptions-item>
      <a-descriptions-item label="分辨率">{{ resolutionDisplay }}</a-descriptions-item>
      <a-descriptions-item label="规格">{{ tc.specification || (encode ? 'Encode' : '—') }}</a-descriptions-item>
      <a-descriptions-item label="媒介">{{ mediumDisplay }}</a-descriptions-item>
      <a-descriptions-item label="分发方">
        {{ tc.source_platform || '—' }}
        <a-tooltip v-if="PLATFORM_FULLNAMES[tc.source_platform]" :title="PLATFORM_FULLNAMES[tc.source_platform]">
          <InfoCircleOutlined style="color: #999; margin-left: 4px" />
        </a-tooltip>
      </a-descriptions-item>
      <a-descriptions-item label="版本">{{ tc.edition_info || '—' }}</a-descriptions-item>
      <a-descriptions-item label="地区码">{{ tc.region_code || '—' }}</a-descriptions-item>
    </a-descriptions>

    <!-- 分区三：技术参数（橙） -->
    <a-descriptions :column="4" bordered size="small" class="std-zone">
      <template #title>
        <div class="zone-header zone-tech">技术参数</div>
      </template>
      <a-descriptions-item label="视频编码">{{ videoCodecDisplay }}</a-descriptions-item>
      <a-descriptions-item label="HDR">{{ tc.hdr === 'SDR' ? 'SDR' : (tc.hdr || '—') }}</a-descriptions-item>
      <a-descriptions-item label="bit">{{ tc.bit_depth || '—' }}</a-descriptions-item>
      <a-descriptions-item label="帧率">{{ tc.frame_rate || '—' }}</a-descriptions-item>
      <a-descriptions-item label="音频编码">{{ tc.audio_codec || '—' }}</a-descriptions-item>
      <a-descriptions-item label="声道">{{ tc.audio_channels || '—' }}</a-descriptions-item>
      <a-descriptions-item label="音频技术">{{ tc.audio_technology || '—' }}</a-descriptions-item>
      <a-descriptions-item label="音轨数">{{ tc.audio_tracks || '—' }}</a-descriptions-item>
    </a-descriptions>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { InfoCircleOutlined } from '@ant-design/icons-vue'
import { CATEGORY_LABELS, PLATFORM_FULLNAMES } from '@/generated/dict'

const props = withDefaults(defineProps<{
  tc: Record<string, string>
  encode?: boolean
  column?: number
  genre?: string[]   // §59.226 #6④: 类别（◎类别原子词——分区一）
  region?: string[]  // §59.226: 产地（◎产地——分区一）
}>(), {
  encode: false,
  column: 4,
  genre: () => [],
  region: () => [],
})

const genreList = computed(() => props.genre || [])
const regionList = computed(() => props.region || [])

// §59.226 附十一: 4K/8K 展示层归一（渲染层转换——存储保 4K 供验证层等价语义）
const resolutionDisplay = computed(() => {
  const r = (props.tc.resolution || '').toUpperCase()
  if (r === '4K') return '2160p'
  if (r === '8K') return '4320p'
  return props.tc.resolution || '—'
})

// §59.226 附四: 媒介 canonical 单点——后端 medium_canonical 出值（前端
// siteMediumDisplay 规则副本废除——§59.166 漂移四处教训）
const mediumDisplay = computed(() => props.tc.medium_canonical || '—')

// §59.226 附十二: 视频编码形态按媒介+铁证（后端已按 V1.05 形态出值——
// 前端纯展示；空值 encode 布尔兜底展示旧值兼容）
const videoCodecDisplay = computed(() => props.tc.video_codec || '—')

const categoryLabel = (key: string) => CATEGORY_LABELS[key] || key || '—'
</script>

<style scoped>
.std-sections :deep(.ant-descriptions) {
  margin-bottom: 8px;
}
/* §59.226 附二十一: 三分区表头颜色视觉区分 */
.std-sections :deep(.ant-descriptions-header) {
  margin-bottom: 4px;
}
.zone-header {
  font-size: 13px;
  font-weight: 600;
  padding: 2px 10px;
  border-radius: 3px;
  display: inline-block;
  color: #fff;
}
/* 作品信息（蓝）——PTGen 优先族 */
.zone-work {
  background: #1677ff;
}
/* 发布规格（绿）——标题原样族 */
.zone-spec {
  background: #389e0d;
}
/* 技术参数（橙）——MI 铁证族 */
.zone-tech {
  background: #d46b08;
}
.std-sections :deep(.ant-descriptions-item-label) {
  width: 88px;
  min-width: 88px;
}
</style>
