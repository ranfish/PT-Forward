<template>
  <a-modal
    :open="open"
    title="选站发布"
    :footer="null"
    width="760px"
    @update:open="(v: boolean) => emit('update:open', v)"
  >
    <div v-else-if="!result" class="step-select">
      <a-form layout="vertical">
        <a-form-item label="目标站（已启用发布配置——多选并行发布）">
          <a-select
            v-model:value="selectedSites"
            mode="multiple"
            style="width: 100%"
            placeholder="选择站点（单选可用预检——适配工具；多选直接提交）"
          >
            <a-select-option v-for="t in targets" :key="t.name" :value="t.name">
              {{ t.name }}<a-tag v-if="t.has_pre_audit" color="blue" style="margin-left: 8px">官方预检</a-tag>
            </a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item v-if="selectedSites.length === 1" label="条件标签人工勾选（auto:false——人工确认后进入）">
          <a-select
            v-model:value="tagOverrides"
            mode="multiple"
            style="width: 100%"
            placeholder="留空=纯自动推断"
            :options="conditionalTagOptions"
          />
        </a-form-item>
      </a-form>
      <div class="actions">
        <a-button
          v-if="selectedSites.length === 1"
          :disabled="loading"
          :loading="loading"
          @click="runDryRun"
        >
          预检（DryRun 适配工具·不落站）
        </a-button>
        <a-button :disabled="!selectedSites.length || loading" type="primary" :loading="loading" @click="runBatch">
          {{ loading ? '发布中…' : `发布（${selectedSites.length || 0} 站）` }}
        </a-button>
      </div>
    </div>

    <div v-if="batchResults" class="step-result">
      <a-alert :type="batchAlertType" show-icon style="margin-bottom: 12px">
        <template #message>
          <span style="font-size: 15px; font-weight: 600">
            发布成功 {{ okCount }} 站<span v-if="existCount"> · 已存在 {{ existCount }} 站</span><span v-if="failCount"> · 失败 {{ failCount }} 站</span>
          </span>
        </template>
        <template #description>
          逐站明细（{{ batchResults.length }} 站）——失败原因见各行/发布日志
        </template>
      </a-alert>
      <div v-for="r in batchResults" :key="r.site" class="batch-row">
        <a-tag :color="batchRowColor(r.status)">{{ r.site }}</a-tag>
        <span>{{ batchRowLabel(r.status) }}</span>
        <span v-if="r.message" class="muted">{{ r.message.slice(0, 80) }}</span>
        <a v-if="r.url" :href="r.url" target="_blank" style="font-size: 12px">详情</a>
      </div>
      <div class="actions">
        <a-button @click="$router.push('/publish/logs')">查看发布日志</a-button>
        <a-button type="primary" @click="emit('update:open', false)">完成</a-button>
      </div>
    </div>

    <div v-else class="step-result">
      <a-alert
        :type="alertType"
        :message="alertTitle"
        :description="result.message || undefined"
        show-icon
        style="margin-bottom: 12px"
      />
      <template v-if="result.pre_audit">
        <h4>官方预检明细</h4>
        <div v-for="(d, i) in result.pre_audit.details || []" :key="i" class="pa-detail" :class="d.level?.toLowerCase()">
          <a-tag :color="d.level === 'ERROR' ? 'red' : d.level === 'WARNING' ? 'orange' : 'green'">{{ d.level }}</a-tag>
          <span class="code">{{ d.errorCode }}</span> {{ d.message }}
        </div>
        <p v-if="!(result.pre_audit.details || []).length" class="muted">零明细（满分通过）</p>
      </template>
      <h4 style="margin-top: 12px">表单组装（{{ Object.keys(result.form || {}).length }} 域）</h4>
      <div class="form-grid">
        <div v-for="(v, k) in result.form" :key="k" class="kv">
          <span>{{ k }}</span><b>{{ String(v).slice(0, 40) }}</b>
        </div>
      </div>
      <p v-if="result.tags?.length" class="muted">tags: {{ result.tags.join(', ') }}</p>
      <div class="actions">
        <a-button @click="reset">返回重选</a-button>
        <a-button
          v-if="result.status === 'dry_run_ok' && result.pre_audit?.passed"
          type="primary"
          :loading="loading"
          @click="runSubmit"
        >
          确认提交（上传+加种）
        </a-button>
        <a-button v-else-if="result.status === 'dry_run_ok'" disabled>预检未通过——修正后重试</a-button>
      </div>
    </div>
  </a-modal>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { message } from 'ant-design-vue'
import client from '@/api/client'
import { executeApi, formConfigApi, type ExecuteResult, type PublishTarget } from '@/api/formConfig'

const props = defineProps<{ open: boolean; infoHash: string; seedName?: string }>()
const emit = defineEmits<{ (e: 'update:open', v: boolean): void; (e: 'done'): void }>()

const targets = ref<PublishTarget[]>([])
const conditionalTagOptions = ref<{ label: string; value: string }[]>([])
const selectedSites = ref<string[]>([])
const batchResults = ref<Array<{ site: string; status: string; message?: string; url?: string }> | null>(null)
const tagOverrides = ref<string[]>([])
const loading = ref(false)
const result = ref<ExecuteResult | null>(null)

watch(
  () => props.open,
  async (v) => {
    if (v) {
      result.value = null
      batchResults.value = null
      selectedSites.value = []
      tagOverrides.value = []
      try {
        const res = await executeApi.targets()
        targets.value = res.data?.data ?? []
      } catch {
        targets.value = []
      }
      conditionalTagOptions.value = []
    }
  },
)

const targetSite = computed(() => selectedSites.value[0] ?? '')

watch(targetSite, async (site) => {
  tagOverrides.value = []
  if (!site) return
  try {
    const res = await formConfigApi.get(site)
    const cfg = res.data?.data?.config
    const tags = cfg?.value_mappings?.tags ?? []
    conditionalTagOptions.value = tags
      .filter((t) => t.auto === false && t.standard_keys?.length)
      .map((t) => ({ label: `${t.label}（${t.standard_keys![0]}）`, value: t.standard_keys![0] }))
  } catch {
    conditionalTagOptions.value = []
  }
})

async function runDryRun() {
  loading.value = true
  try {
    const res = await executeApi.execute(props.infoHash, targetSite.value, {
      dryRun: true,
      tagOverrides: tagOverrides.value,
    })
    result.value = res.data?.data?.result ?? null
  } catch (e) {
    message.error(String(e))
  } finally {
    loading.value = false
  }
}

async function runBatch() {
  loading.value = true
  try {
    const res = await client.post('/publish/seeds/execute-batch', {
      info_hash: props.infoHash,
      target_sites: selectedSites.value,
      tag_overrides: tagOverrides.value,
    })
    batchResults.value = res.data?.data?.results ?? []
    emit('done')
  } catch (e) {
    message.error(String(e))
  } finally {
    loading.value = false
  }
}

const okCount = computed(() => (batchResults.value ?? []).filter((r) => r.status === 'pushed' || r.status === 'pushed_existing').length)
const existCount = computed(() => (batchResults.value ?? []).filter((r) => r.status === 'existing' || r.status === 'duplicate').length)
const failCount = computed(() => {
  const rs = batchResults.value ?? []
  return rs.length - okCount.value - existCount.value
})
const batchAlertType = computed(() => {
  if (failCount.value > 0) return 'error'
  if (okCount.value > 0) return 'success'
  return 'warning'
})

function batchRowLabel(s: string): string {
  const map: Record<string, string> = {
    pushed: '发布成功·已推种',
    pushed_existing: '站上已有·已推种',
    existing: '站上已有（未定位 ID）',
    duplicate: '站上已有同内容种',
    uploaded: '已上传（推种未确认）',
    failed: '失败',
  }
  return map[s] ?? s
}

function batchRowColor(s: string): string {
  if (s === 'pushed' || s === 'pushed_existing') return 'green'
  if (s === 'existing') return 'blue'
  return 'red'
}

async function runSubmit() {
  loading.value = true
  try {
    const res = await executeApi.execute(props.infoHash, targetSite.value, {
      dryRun: false,
      tagOverrides: tagOverrides.value,
    })
    result.value = res.data?.data?.result ?? null
    if (result.value && ['uploaded', 'pushed', 'uploaded_existing', 'pushed_existing', 'existing'].includes(result.value.status)) {
      message.success(`发布成功：${result.value.target_torrent_url || result.value.status}`)
      emit('done')
    }
  } catch (e) {
    message.error(String(e))
  } finally {
    loading.value = false
  }
}

function reset() {
  result.value = null
}

const alertType = computed(() => {
  if (!result.value) return 'info'
  const s = result.value.status
  if (s === 'pushed' || s === 'uploaded') return 'success'
  if (s === 'pushed_existing' || s === 'uploaded_existing') return 'info'
  if (s === 'dry_run_ok') return result.value.pre_audit?.passed ? 'success' : 'warning'
  return 'error'
})
const alertTitle = computed(() => {
  if (!result.value) return ''
  const s = result.value.status
  const pa = result.value.pre_audit
  if (s === 'pushed') return '已发布+已加种'
  if (s === 'uploaded') return '已上传（加种未确认）'
  if (s === 'dry_run_ok') return `DryRun 预检${pa?.passed ? '通过' : '未通过'}（${pa?.totalScore ?? 0} 分）`
  return `失败：${s}`
})
</script>

<style scoped>
.actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 12px; }
.form-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 4px 16px; font-size: 12px; }
.kv span { color: #888; margin-right: 8px; }
.kv b { font-weight: 500; word-break: break-all; }
.pa-detail { font-size: 12px; margin: 2px 0; }
.pa-detail .code { color: #c62828; font-family: monospace; margin-right: 4px; }
.muted { color: #888; font-size: 12px; }
h4 { margin: 8px 0 4px; }
</style>
