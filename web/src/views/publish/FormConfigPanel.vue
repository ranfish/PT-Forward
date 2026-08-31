<template>
  <div class="form-config-panel">
    <h3>发布表单配置</h3>
    <p class="hint">HTML 上传半自动：粘贴发布页源码 → 解析草稿 → diff 确认 → 落库（唯一写入路径）</p>

    <div class="grid">
      <!-- 左：HTML 上传 -->
      <div class="card">
        <h3>① 上传发布页 HTML</h3>
        <p class="hint">浏览器打开目标站 upload.php → 另存/查看源码 → 粘贴。原文即弃不落库。</p>
        <input type="file" accept=".html,.htm" @change="onFile">
        <textarea v-model="html" rows="8" placeholder="或粘贴 HTML 源码…" />
        <button class="btn primary" :disabled="!html || parsing" @click="doParse">
          {{ parsing ? '解析中…' : '② 解析 + Diff' }}
        </button>
      </div>

      <!-- 右：当前配置只读 -->
      <div class="card">
        <h3>当前配置</h3>
        <template v-if="current">
          <div class="kv"><span>enabled</span><b>{{ current.enabled }}</b></div>
          <div class="kv"><span>framework</span><b>{{ current.framework || '—' }}</b></div>
          <div class="kv"><span>pre_audit_url</span><b>{{ current.pre_audit_url || '—' }}</b></div>
          <div v-for="(field, domain) in current.form_fields" :key="domain" class="kv">
            <span>{{ domain }}</span><b>{{ field }}</b>
            <small v-if="current.value_mappings?.[domain]?.length">{{ current.value_mappings[domain].length }} 项</small>
          </div>
        </template>
        <p v-else class="hint">未配置（新站首次接入——解析后全量为新增项）</p>
      </div>
    </div>

    <!-- diff 三分类 -->
    <div v-if="diffs" class="card diff-card full">
      <h3>③ Diff 确认<span v-if="diffs.length" class="badge">{{ diffs.length }}</span></h3>
      <p class="hint">matched 已自动校准不显示；added=待标注（standard_keys 空）；changed=改版/语义错位信号</p>
      <table v-if="diffs.length" class="diff-table">
        <thead>
          <tr><th>域</th><th>类型</th><th>Label</th><th>现值</th><th>新值</th><th>语义</th></tr>
        </thead>
        <tbody>
          <tr v-for="(d, i) in diffs" :key="i" :class="d.kind">
            <td>{{ d.domain }}</td>
            <td><span class="kind" :class="d.kind">{{ kindLabel(d.kind) }}</span></td>
            <td>{{ d.label }}<small v-if="d.field_rename">{{ d.field_rename }}</small></td>
            <td>{{ d.current_value || '—' }}</td>
            <td>{{ d.draft_value || '—' }}</td>
            <td>
              <span v-if="d.current_keys?.length">{{ d.current_keys.join(', ') }}</span>
              <span v-else-if="d.kind === 'added'" class="warn">待标注</span>
              <span v-else>—</span>
              <span v-if="d.auto_false" class="badge auto">auto:false</span>
            </td>
          </tr>
        </tbody>
      </table>
      <p v-else class="ok">✓ 全部匹配（基线校准通过——无变化无新增）</p>
      <button v-if="diffs.length >= 0 && merged" class="btn primary" @click="doApply">
        ④ 确认落库（{{ mergedSummary }}）
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { formConfigApi, type FormConfigDiffItem, type PublishFormConfig } from '@/api/formConfig'

// §59.157: 站点详情页嵌入面板（仅 is_source 站渲染——官组映射完备=可发布目标站）
const props = defineProps<{ siteName: string }>()

const current = ref<PublishFormConfig | null>(null)
const html = ref('')
const parsing = ref(false)
const diffs = ref<FormConfigDiffItem[] | null>(null)
const merged = ref<PublishFormConfig | null>(null)

onMounted(() => loadCurrent())

async function loadCurrent() {
  diffs.value = null
  merged.value = null
  const res = await formConfigApi.get(props.siteName)
  current.value = res.data?.data?.config ?? null
}

function onFile(e: Event) {
  const f = (e.target as HTMLInputElement).files?.[0]
  if (!f) return
  const reader = new FileReader()
  reader.onload = () => (html.value = String(reader.result ?? ''))
  reader.readAsText(f)
}

async function doParse() {
  parsing.value = true
  try {
    const res = await formConfigApi.parse(props.siteName, html.value)
    diffs.value = res.data?.data?.diffs ?? []
    merged.value = res.data?.data?.merged ?? null
    html.value = '' // 即弃
  } finally {
    parsing.value = false
  }
}

async function doApply() {
  if (!merged.value) return
  await formConfigApi.apply(props.siteName, merged.value, 'HTML 上传 diff 确认')
  await loadCurrent()
  diffs.value = null
  merged.value = null
}

const mergedSummary = computed(() => {
  if (!merged.value?.form_fields) return ''
  const domains = Object.keys(merged.value.form_fields)
  let values = 0
  for (const v of Object.values(merged.value.value_mappings ?? {})) values += v.length
  return `${domains.length} 域 ${values} 值`
})

function kindLabel(k: string) {
  return { changed: '改版/错位', added: '新增', removed: '删除' }[k] ?? k
}
</script>

<style scoped>
.form-config-panel { margin-top: 24px; }

.hint { color: #888; font-size: 12px; margin: 4px 0 12px; }
.toolbar { display: flex; gap: 8px; margin-bottom: 16px; }
.site-select { min-width: 200px; }
.grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
.card { border: 1px solid #ddd; border-radius: 8px; padding: 12px; margin-bottom: 16px; }
.card h3 { margin: 0 0 8px; }
.card textarea { width: 100%; margin: 8px 0; font-family: monospace; font-size: 11px; }
.kv { display: flex; gap: 8px; align-items: baseline; padding: 2px 0; font-size: 13px; }
.kv span { color: #888; min-width: 110px; }
.kv small { color: #aaa; }
.diff-card.full { grid-column: 1 / -1; }
.diff-card table { width: 100%; border-collapse: collapse; font-size: 12px; }
.diff-card th, .diff-card td { border: 1px solid #eee; padding: 4px 8px; text-align: left; }
.diff-card td small { color: #888; display: block; }
tr.changed td { background: #fff8e1; }
tr.added td { background: #e8f5e9; }
tr.removed td { background: #fbe9e7; }
.kind { font-weight: 600; }
.kind.changed { color: #f57f17; }
.kind.added { color: #2e7d32; }
.kind.removed { color: #c62828; }
.warn { color: #f57f17; }
.badge { background: #eee; border-radius: 8px; padding: 1px 8px; font-size: 11px; margin-left: 8px; }
.badge.auto { background: #ffe0b2; }
.btn { padding: 6px 16px; border-radius: 6px; border: 1px solid #ccc; background: #fff; cursor: pointer; }
.btn.primary { background: #1976d2; color: #fff; border-color: #1976d2; }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.ok { color: #2e7d32; }
</style>
