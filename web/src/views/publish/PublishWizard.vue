<template>
  <div>
    <a-page-header :title="t('publish.manualForward')" @back="$router.push('/publish')">
      <template #extra>
        <a-button @click="$router.push('/publish')">{{ t('common.cancel') }}</a-button>
      </template>
    </a-page-header>

    <a-card>
      <a-steps :current="currentStep" style="margin-bottom: 24px">
        <a-step :title="t('publish.wizard.selectTorrent')" />
        <a-step :title="t('publish.wizard.analyze')" />
        <a-step :title="t('publish.wizard.review')" />
        <a-step :title="t('publish.wizard.selectTargets')" />
        <a-step :title="t('publish.wizard.preview')" />
        <a-step :title="t('publish.wizard.submit')" />
      </a-steps>

      <div v-if="currentStep === 0">
        <a-form layout="vertical">
          <a-form-item :label="t('seeding.client')">
            <a-select v-model:value="selectedClientId" style="width: 240px" :loading="clientsLoading" @change="fetchSeededTorrents">
              <a-select-option v-for="c in clients" :key="c.id" :value="c.id">{{ c.name }} ({{ c.type }})</a-select-option>
            </a-select>
          </a-form-item>
        </a-form>
        <a-table
          :columns="torrentColumns"
          :data-source="seededTorrents"
          :loading="torrentsLoading"
          :pagination="{ pageSize: 20 }"
          row-key="info_hash"
          size="small"
          :row-selection="{ type: 'radio', selectedRowKeys: selectedTorrent ? [selectedTorrent.info_hash] : [], onSelect: (r: unknown) => { selectedTorrent = r as SeededTorrent } }"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'size'">
              {{ formatBytes(record.size) }}
            </template>
          </template>
        </a-table>
        <div style="margin-top: 16px; text-align: right">
          <a-button type="primary" :disabled="!selectedTorrent" @click="startAnalyze">{{ t('common.create') }}</a-button>
        </div>
      </div>

      <div v-if="currentStep === 1">
        <a-spin :spinning="analyzing" :tip="t('publish.wizard.analyzing')">
          <a-result v-if="!analyzing && analyzeResult" status="success" :title="t('publish.wizard.analyzeComplete')">
            <template #subTitle>
              {{ analyzeResult.name }}
            </template>
          </a-result>
          <a-result v-if="!analyzing && analyzeError" status="error" :title="analyzeError" />
        </a-spin>
        <div v-if="!analyzing && analyzeResult" style="margin-top: 16px; text-align: right">
          <a-button style="margin-right: 8px" @click="currentStep = 0">{{ t('common.cancel') }}</a-button>
          <a-button type="primary" :disabled="analyzeResult?.forbidden" @click="currentStep = 2">{{ t('common.save') }}</a-button>
        </div>
      </div>

      <div v-if="currentStep === 2">
        <a-alert v-if="analyzeResult?.forbidden" type="error" show-icon style="margin-bottom: 16px">
          <template #message>{{ analyzeResult?.forbid_reason }}</template>
        </a-alert>
        <a-form layout="vertical">
          <a-form-item :label="t('common.title')">
            <a-input v-model:value="form.title" />
          </a-form-item>
          <a-form-item :label="t('publish.mediaInfo')">
            <a-textarea v-model:value="form.mediaInfo" :rows="4" />
          </a-form-item>
          <a-form-item :label="t('common.name')">
            <a-textarea v-model:value="form.description" :rows="4" />
          </a-form-item>
        </a-form>
        <div style="margin-top: 16px; text-align: right">
          <a-button style="margin-right: 8px" @click="currentStep = 1">{{ t('common.cancel') }}</a-button>
          <a-button type="primary" @click="fetchTargets">{{ t('common.save') }}</a-button>
        </div>
      </div>

      <div v-if="currentStep === 3">
        <a-table
          :columns="targetColumns"
          :data-source="eligibleTargets"
          :pagination="false"
          row-key="name"
          size="small"
          :row-selection="{ selectedRowKeys: selectedTargets, onSelectChange: (keys: string[]) => { selectedTargets = keys } }"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'blocked'">
              <a-tag :color="record.blocked ? 'red' : 'green'">{{ record.blocked ? t('publish.blocked') : t('publish.ok') }}</a-tag>
            </template>
          </template>
        </a-table>
        <div style="margin-top: 16px; text-align: right">
          <a-button style="margin-right: 8px" @click="currentStep = 2">{{ t('common.cancel') }}</a-button>
          <a-button type="primary" :disabled="selectedTargets.length === 0" @click="currentStep = 4">{{ t('common.save') }}</a-button>
        </div>
      </div>

      <div v-if="currentStep === 4">
        <a-descriptions bordered :column="1" size="small">
          <a-descriptions-item :label="t('common.title')">{{ form.title }}</a-descriptions-item>
          <a-descriptions-item :label="t('publish.sourceSite')">{{ analyzeResult?.source_site || '-' }}</a-descriptions-item>
          <a-descriptions-item :label="t('publish.targetSites')">{{ selectedTargets.join(', ') }}</a-descriptions-item>
          <a-descriptions-item :label="t('publish.mediaInfo')">
            <pre style="max-height: 200px; overflow: auto; margin: 0; font-size: 12px">{{ form.mediaInfo || '-' }}</pre>
          </a-descriptions-item>
        </a-descriptions>
        <div style="margin-top: 16px; text-align: right">
          <a-button style="margin-right: 8px" @click="currentStep = 3">{{ t('common.cancel') }}</a-button>
          <a-button type="primary" :loading="submitting" @click="doSubmit">{{ t('publish.publishAction') }}</a-button>
        </div>
      </div>

      <div v-if="currentStep === 5">
        <a-result status="success" :title="t('common.operationSuccess')">
          <template #sub-title>
            {{ t('publish.wizard.candidateCreated', { id: submittedCandidateId }) }}
          </template>
          <template #extra>
            <a-button type="primary" @click="reset">{{ t('publish.wizard.publishAnother') }}</a-button>
            <a-button @click="$router.push('/publish')">{{ t('common.detail') }}</a-button>
          </template>
        </a-result>
      </div>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { manualForwardApi } from '@/api/publish'
import { downloadersApi } from '@/api/downloaders'

const { t } = useI18n()

const currentStep = ref(0)

interface SeededTorrent {
  info_hash: string
  name: string
  size: number
  save_path: string
  upload_speed: number
  seeders: number
  state: string
  client_id: number
}

const clients = ref<{ id: number; name: string; type: string }[]>([])
const clientsLoading = ref(false)
const selectedClientId = ref<number | undefined>(undefined)
const seededTorrents = ref<SeededTorrent[]>([])
const torrentsLoading = ref(false)
const selectedTorrent = ref<SeededTorrent | null>(null)

const analyzing = ref(false)
const analyzeResult = ref<Record<string, unknown> | null>(null)
const analyzeError = ref('')

const eligibleTargets = ref<{ id: number; name: string; domain: string; base_url: string; blocked: boolean }[]>([])
const selectedTargets = ref<string[]>([])

const form = ref({ title: '', mediaInfo: '', description: '', screenshots: [] as string[] })
const submitting = ref(false)
const submittedCandidateId = ref(0)

const torrentColumns = [
  { title: t('common.name'), dataIndex: 'name', key: 'name', ellipsis: true },
  { title: t('common.size'), key: 'size', width: 100 },
  { title: t('seeding.client'), dataIndex: 'client_id', key: 'client_id', width: 80 },
  { title: t('publish.state'), dataIndex: 'state', key: 'state', width: 100 },
]

const targetColumns = [
  { title: t('common.name'), dataIndex: 'name', key: 'name' },
  { title: t('site.domain'), dataIndex: 'domain', key: 'domain' },
  { title: t('common.status'), key: 'blocked', width: 80 },
]

import { formatBytes } from '@/utils/format'

async function fetchClients() {
  clientsLoading.value = true
  try {
    const resp = await downloadersApi.list(1, 100)
    const data = resp.data?.data
    clients.value = (data?.items || data || []) as { id: number; name: string; type: string }[]
    if (clients.value.length > 0 && !selectedClientId.value) {
      selectedClientId.value = clients.value[0].id
      fetchSeededTorrents()
    }
  } catch { /* ignore */ } finally {
    clientsLoading.value = false
  }
}

async function fetchSeededTorrents() {
  if (!selectedClientId.value) return
  torrentsLoading.value = true
  try {
    const resp = await manualForwardApi.seededTorrents(selectedClientId.value)
    seededTorrents.value = (resp.data?.data || []) as SeededTorrent[]
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    torrentsLoading.value = false
  }
}

let pollTimer: ReturnType<typeof setTimeout> | null = null

onBeforeUnmount(() => {
  if (pollTimer !== null) {
    clearTimeout(pollTimer)
    pollTimer = null
  }
})

async function startAnalyze() {
  if (!selectedTorrent.value) return
  currentStep.value = 1
  analyzing.value = true
  analyzeError.value = ''
  analyzeResult.value = null

  try {
    const startResp = await manualForwardApi.startAnalyze({
      client_id: selectedTorrent.value.client_id,
      info_hash: selectedTorrent.value.info_hash,
      name: selectedTorrent.value.name,
      save_path: selectedTorrent.value.save_path,
    })
    const taskId = startResp.data?.data?.taskId
    if (!taskId) { analyzeError.value = 'No task_id returned'; analyzing.value = false; return }

    const poll = async () => {
      try {
        const pollResp = await manualForwardApi.pollAnalyze(taskId)
        const task = pollResp.data?.data as { status: string; result?: Record<string, unknown>; error?: string } | undefined
        if (task?.status === 'completed') {
          analyzeResult.value = task.result ?? null
          form.value.title = (task.result?.title as string) || selectedTorrent.value?.name || ''
          form.value.mediaInfo = (task.result?.media_info as string) || ''
          form.value.description = (task.result?.description as string) || ''
          form.value.screenshots = (task.result?.screenshots as string[]) || []
          analyzing.value = false
        } else if (task?.status === 'failed') {
          analyzeError.value = task.error || 'Analysis failed'
          analyzing.value = false
        } else {
          pollTimer = setTimeout(poll, 2000)
        }
      } catch (e: unknown) {
        analyzeError.value = (e as Error).message
        analyzing.value = false
      }
    }
    pollTimer = setTimeout(poll, 1500)
  } catch (e: unknown) {
    analyzeError.value = (e as Error).message
    analyzing.value = false
  }
}

async function fetchTargets() {
  currentStep.value = 3
  try {
    const resp = await manualForwardApi.eligibleTargets({
      source_site: (analyzeResult.value?.source_site as string) || '',
      blocked_targets: (analyzeResult.value?.blocked_targets as string[]) || [],
    })
    eligibleTargets.value = (resp.data?.data || []) as unknown as typeof eligibleTargets.value
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function doSubmit() {
  submitting.value = true
  try {
    const resp = await manualForwardApi.submit({
      client_id: selectedTorrent.value?.client_id,
      info_hash: selectedTorrent.value?.info_hash,
      source_site: analyzeResult.value?.source_site as string | undefined,
      source_site_id: analyzeResult.value?.source_site_id as number | undefined,
      title: form.value.title,
      description: form.value.description,
      media_info: form.value.mediaInfo,
      screenshots: form.value.screenshots,
      target_sites: selectedTargets.value,
    })
    submittedCandidateId.value = (resp.data?.data as unknown as { candidate_id?: number })?.candidate_id || 0
    currentStep.value = 5
    message.success(t('common.operationSuccess'))
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    submitting.value = false
  }
}

function reset() {
  currentStep.value = 0
  selectedTorrent.value = null
  analyzeResult.value = null
  analyzeError.value = ''
  eligibleTargets.value = []
  selectedTargets.value = []
  form.value = { title: '', mediaInfo: '', description: '', screenshots: [] }
  submittedCandidateId.value = 0
}

onMounted(fetchClients)
</script>
