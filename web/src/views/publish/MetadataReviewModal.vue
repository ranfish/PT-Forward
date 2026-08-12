<template>
  <a-modal
    v-model:open="visible"
    :title="`核对详情 — ${meta?.title || torrentName || infoHash?.substring(0, 12)}`"
    width="800px"
    :footer="null"
    @cancel="handleClose"
  >
    <a-spin :spinning="loading">
      <a-form layout="vertical">
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="标题">
              <a-input v-model:value="form.title" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="副标题">
              <a-input v-model:value="form.subtitle" />
            </a-form-item>
          </a-col>
        </a-row>

        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item label="类型">
              <a-select v-model:value="form.standard_type" style="width: 100%">
                <a-select-option value="category.movie">电影</a-select-option>
                <a-select-option value="category.tv_series">电视剧</a-select-option>
                <a-select-option value="category.animation">动漫</a-select-option>
                <a-select-option value="category.documentaries">纪录片</a-select-option>
                <a-select-option value="category.tv_shows">综艺</a-select-option>
                <a-select-option value="category.music">音乐</a-select-option>
                <a-select-option value="category.sports">体育</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="核对状态">
              <a-tag :color="form.reviewed ? 'green' : 'orange'">
                {{ form.reviewed ? '已核对' : '未核对' }}
              </a-tag>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="来源">
              <a-tag>{{ form.fetch_source || '-' }}</a-tag>
            </a-form-item>
          </a-col>
        </a-row>

        <a-form-item label="标签">
          <a-input v-model:value="form.tags" placeholder='["中字","官组"]' />
        </a-form-item>

        <a-form-item label="IMDb">
          <a-input v-model:value="form.imdb_url" placeholder="IMDb 链接" />
        </a-form-item>

        <a-form-item label="截图">
          <div v-if="screenshotList.length" class="screenshot-list">
            <a-image
              v-for="(url, i) in screenshotList"
              :key="i"
              :src="url"
              :width="120"
              :height="68"
              class="screenshot-item"
            />
          </div>
          <a-empty v-else description="无截图" :image="simpleImage" />
        </a-form-item>

        <a-form-item label="简介">
          <a-textarea v-model:value="form.description" :rows="4" />
        </a-form-item>

        <a-collapse :bordered="false" style="margin-bottom: 16px">
          <a-collapse-panel key="mi" header="MediaInfo">
            <pre class="mediainfo-text">{{ form.mediainfo || '无 MediaInfo' }}</pre>
          </a-collapse-panel>
        </a-collapse>

        <div style="text-align: right">
          <a-space>
            <a-button @click="handleClose">取消</a-button>
            <a-button @click="handleMarkUnreviewed" v-if="form.reviewed">标记为未核对</a-button>
            <a-button type="primary" :loading="saving" @click="handleSave">保存并标记已核对</a-button>
          </a-space>
        </div>
      </a-form>
    </a-spin>
  </a-modal>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import { message, Empty } from 'ant-design-vue'
import { metadataApi, type MetadataDetail } from '@/api/image-host'

const props = defineProps<{
  open: boolean
  infoHash: string
  torrentName?: string
}>()

const emit = defineEmits<{
  'update:open': [val: boolean]
  saved: [infoHash: string]
}>()

const visible = computed({
  get: () => props.open,
  set: (v) => emit('update:open', v),
})

const loading = ref(false)
const saving = ref(false)
const meta = ref<MetadataDetail | null>(null)
const simpleImage = Empty.PRESENTED_IMAGE_SIMPLE

const form = reactive({
  title: '',
  subtitle: '',
  standard_type: '',
  tags: '',
  description: '',
  imdb_url: '',
  mediainfo: '',
  reviewed: false,
  fetch_source: '',
})

const screenshotList = computed(() => {
  if (!form.tags && !meta.value?.screenshots) return []
  try {
    const raw = meta.value?.screenshots || '[]'
    return JSON.parse(raw) as string[]
  } catch {
    return []
  }
})

async function loadMetadata() {
  if (!props.infoHash) return
  loading.value = true
  try {
    const { data } = await metadataApi.get(props.infoHash)
    const items = data.data?.items || []
    if (items.length > 0) {
      const m = items[0]
      meta.value = m
      form.title = m.title || ''
      form.subtitle = m.subtitle || ''
      form.standard_type = m.standard_type || ''
      form.tags = m.tags || ''
      form.description = m.description || ''
      form.imdb_url = m.imdb_url || ''
      form.mediainfo = m.mediainfo || m.source_mediainfo || ''
      form.reviewed = m.reviewed || false
      form.fetch_source = m.fetch_source || ''
    } else {
      resetForm()
    }
  } catch {
    resetForm()
  } finally {
    loading.value = false
  }
}

function resetForm() {
  meta.value = null
  form.title = props.torrentName || ''
  form.subtitle = ''
  form.standard_type = ''
  form.tags = ''
  form.description = ''
  form.imdb_url = ''
  form.mediainfo = ''
  form.reviewed = false
  form.fetch_source = ''
}

async function handleSave() {
  saving.value = true
  try {
    await metadataApi.update({
      infoHash: props.infoHash,
      siteName: meta.value?.site_name,
      title: form.title,
      subtitle: form.subtitle,
      standardType: form.standard_type,
      tags: form.tags,
      description: form.description,
    })
    message.success('保存成功，已标记为已核对')
    emit('saved', props.infoHash)
    visible.value = false
  } catch {
    message.error('保存失败')
  } finally {
    saving.value = false
  }
}

async function handleMarkUnreviewed() {
  try {
    await metadataApi.setReviewed(props.infoHash, false, meta.value?.site_name)
    message.success('已标记为未核对')
    emit('saved', props.infoHash)
    visible.value = false
  } catch {
    message.error('操作失败')
  }
}

function handleClose() {
  visible.value = false
}

watch(() => props.open, (val) => {
  if (val && props.infoHash) {
    loadMetadata()
  }
})
</script>

<style scoped>
.screenshot-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.screenshot-item {
  border-radius: 4px;
  border: 1px solid #d9d9d9;
}
.mediainfo-text {
  background: #f5f5f5;
  padding: 8px 12px;
  border-radius: 4px;
  font-size: 12px;
  font-family: monospace;
  max-height: 300px;
  overflow-y: auto;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
