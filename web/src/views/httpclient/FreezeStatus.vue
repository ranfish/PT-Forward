<template>
  <div>
    <a-page-header :title="t('httpclient.title')" :subtitle="t('httpclient.subtitle')">
      <template #extra>
        <a-button @click="fetchData">
          <template #icon><ReloadOutlined /></template>
          {{ t('common.refresh') }}
        </a-button>
      </template>
    </a-page-header>

    <a-tabs v-model:active-key="activeTab" @change="onTabChange">
      <a-tab-pane key="freeze" :tab="t('httpclient.freezeStatus')">
        <a-table
          :columns="freezeColumns"
          :data-source="freezeStatuses"
          :loading="freezeLoading"
          :pagination="false"
          row-key="domain"
          size="small"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'frozen'">
              <a-badge :status="record.frozen ? 'error' : 'success'" :text="record.frozen ? t('httpclient.frozen') : t('httpclient.normal')" />
            </template>
            <template v-if="column.key === 'reason'">
              {{ record.reason || '-' }}
            </template>
            <template v-if="column.key === 'actions'">
              <a-popconfirm v-if="record.frozen" :title="t('httpclient.unfreezeConfirm')" @confirm="handleUnfreeze(record.domain)">
                <a-button type="link" size="small">{{ t('httpclient.unfreeze') }}</a-button>
              </a-popconfirm>
              <span v-else>-</span>
            </template>
          </template>
        </a-table>
      </a-tab-pane>

      <a-tab-pane key="circuit" :tab="t('httpclient.circuitStatus')">
        <a-table
          :columns="circuitColumns"
          :data-source="circuitStatuses"
          :loading="circuitLoading"
          :pagination="false"
          row-key="domain"
          size="small"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'state'">
              <a-tag :color="record.state === 'closed' ? 'green' : record.state === 'open' ? 'red' : 'orange'">
                {{ record.state === 'closed' ? t('httpclient.closed') : record.state === 'open' ? t('httpclient.open') : t('httpclient.halfOpen') }}
              </a-tag>
            </template>
            <template v-if="column.key === 'actions'">
              <a-popconfirm :title="t('httpclient.resetCircuitConfirm')" @confirm="handleResetCircuit(record.domain)">
                <a-button type="link" size="small">{{ t('httpclient.reset') }}</a-button>
              </a-popconfirm>
            </template>
          </template>
        </a-table>
      </a-tab-pane>
    </a-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { ReloadOutlined } from '@ant-design/icons-vue'
import { httpclientApi } from '@/api/httpclient'
import { formatTime } from '@/utils/format'

const { t } = useI18n()

interface FreezeStatusItem {
  domain: string
  frozen: boolean
  reason: string
  remaining: string
}

interface CircuitStatusItem {
  domain: string
  state: string
  failures: number
  last_failure: string
}

const activeTab = ref('freeze')
const freezeLoading = ref(false)
const circuitLoading = ref(false)
const freezeStatuses = ref<FreezeStatusItem[]>([])
const circuitStatuses = ref<CircuitStatusItem[]>([])

const freezeColumns = [
  { title: t('site.domain'), dataIndex: 'domain', key: 'domain', ellipsis: true },
  { title: t('common.status'), key: 'frozen', width: 100 },
  { title: t('httpclient.reason'), key: 'reason', ellipsis: true },
  { title: t('httpclient.remainingTime'), dataIndex: 'remaining', key: 'remaining', width: 120 },
  { title: t('common.actions'), key: 'actions', width: 80 },
]

const circuitColumns = [
  { title: t('site.domain'), dataIndex: 'domain', key: 'domain', ellipsis: true },
  { title: t('common.status'), key: 'state', width: 100 },
  { title: t('httpclient.failures'), dataIndex: 'failures', key: 'failures', width: 100 },
  { title: t('httpclient.lastFailure'), dataIndex: 'last_failure', key: 'last_failure', width: 180, customRender: ({ text }: { text: string }) => formatTime(text) },
  { title: t('common.actions'), key: 'actions', width: 80 },
]

async function fetchFreezeStatuses() {
  freezeLoading.value = true
  try {
    const resp = await httpclientApi.getFreezeStatuses()
    freezeStatuses.value = (resp.data.data || []) as unknown as FreezeStatusItem[]
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    freezeLoading.value = false
  }
}

async function fetchCircuitStatuses() {
  circuitLoading.value = true
  try {
    const resp = await httpclientApi.getCircuitStatuses()
    circuitStatuses.value = (resp.data.data || []) as CircuitStatusItem[]
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    circuitLoading.value = false
  }
}

function fetchData() {
  if (activeTab.value === 'freeze') fetchFreezeStatuses()
  else fetchCircuitStatuses()
}

function onTabChange(key: string) {
  if (key === 'freeze' && freezeStatuses.value.length === 0) fetchFreezeStatuses()
  if (key === 'circuit' && circuitStatuses.value.length === 0) fetchCircuitStatuses()
}

async function handleUnfreeze(domain: string) {
  try {
    await httpclientApi.unfreezeDomain(domain)
    message.success(t('httpclient.domainUnfrozen', { domain }))
    fetchFreezeStatuses()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function handleResetCircuit(domain: string) {
  try {
    await httpclientApi.resetCircuit(domain)
    message.success(t('httpclient.circuitReset', { domain }))
    fetchCircuitStatuses()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

onMounted(() => {
  fetchFreezeStatuses()
})
</script>
