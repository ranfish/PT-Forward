<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; justify-content: flex-end">
      <a-button type="primary" @click="openModal()">
        <template #icon><PlusOutlined /></template>
        {{ t('settings.notifications.addChannel') }}
      </a-button>
    </div>

    <a-table
      :columns="columns"
      :data-source="channels"
      :loading="loading"
      :pagination="false"
      row-key="id"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'type'">
          <a-tag>{{ record.type }}</a-tag>
        </template>
        <template v-if="column.key === 'enabled'">
          <a-switch :checked="record.enabled" @change="toggleChannel(record)" />
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button type="link" size="small" @click="showHistory(record)">{{ t('settings.notifications.history') }}</a-button>
            <a-button type="link" size="small" @click="openModal(record)">{{ t('common.edit') }}</a-button>
            <a-button type="link" size="small" @click="testChannel(record.id)">{{ t('common.test') }}</a-button>
            <a-popconfirm :title="t('settings.notifications.deleteConfirm')" @confirm="handleDelete(record.id)">
              <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="historyVisible"
      :title="t('settings.notifications.historyTitle', { name: historyChannel?.name || '' })"
      :footer="null"
      width="700px"
    >
      <a-table
        :columns="historyColumns"
        :data-source="historyRecords"
        :loading="historyLoading"
        :pagination="{ pageSize: 10 }"
        row-key="id"
        size="small"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'success'">
            <a-tag :color="record.success ? 'green' : 'red'">{{ record.success ? t('common.success') : t('common.failed') }}</a-tag>
          </template>
        </template>
      </a-table>
    </a-modal>

    <a-modal
      v-model:open="modalVisible"
      :title="editingChannel ? t('settings.notifications.editChannel') : t('settings.notifications.addChannel')"
      :confirm-loading="submitting"
      @ok="handleSubmit"
    >
      <a-form :model="form" layout="vertical">
        <a-form-item :label="t('settings.notifications.channelName')" name="name" :rules="[{ required: true, message: t('settings.notifications.channelNameRequired') }]">
          <a-input v-model:value="form.name" :placeholder="t('settings.notifications.channelNamePlaceholder')" />
        </a-form-item>
        <a-form-item :label="t('settings.notifications.channelType')" name="type" :rules="[{ required: true, message: t('settings.notifications.channelTypeRequired') }]">
          <a-select v-model:value="form.type" :placeholder="t('settings.notifications.selectTypePlaceholder')">
            <a-select-option value="telegram">Telegram</a-select-option>
            <a-select-option value="bark">Bark</a-select-option>
            <a-select-option value="webhook">Webhook</a-select-option>
            <a-select-option value="serverchan">{{ t('settings.notifications.serverchan') }}</a-select-option>
            <a-select-option value="dingtalk">{{ t('settings.notifications.dingtalk') }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item :label="t('settings.notifications.channelConfig')" name="config">
          <a-textarea v-model:value="form.config" :rows="4" :placeholder="t('settings.notifications.channelConfigPlaceholder')" />
        </a-form-item>
        <a-form-item :label="t('common.enabled')" name="enabled">
          <a-switch v-model:checked="form.enabled" />
        </a-form-item>
        <a-form-item :label="t('settings.notifications.subscribeEvents')" name="events">
          <a-select v-model:value="form.events" mode="multiple" :placeholder="t('settings.notifications.allEventsPlaceholder')">
            <a-select-option value="all">all</a-select-option>
            <a-select-option value="rss">rss</a-select-option>
            <a-select-option value="rss_new">rss_new</a-select-option>
            <a-select-option value="publish">publish</a-select-option>
            <a-select-option value="info">info</a-select-option>
            <a-select-option value="warning">warning</a-select-option>
            <a-select-option value="error">error</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item :label="t('settings.notifications.maxErrorsPerHour')" name="maxErrorsPerHour">
          <a-input-number v-model:value="form.maxErrorsPerHour" :min="0" :max="10000" style="width: 100%" />
        </a-form-item>
        <a-form-item :label="t('settings.notifications.timeoutMs')" name="timeoutMs">
          <a-input-number v-model:value="form.timeoutMs" :min="1000" :max="60000" :step="1000" style="width: 100%" />
        </a-form-item>
        <a-form-item :label="t('settings.notifications.quietHoursStart')" name="quietHoursStart">
          <a-input v-model:value="form.quietHoursStart" :placeholder="t('settings.notifications.quietHoursStartPlaceholder')" />
        </a-form-item>
        <a-form-item :label="t('settings.notifications.quietHoursEnd')" name="quietHoursEnd">
          <a-input v-model:value="form.quietHoursEnd" :placeholder="t('settings.notifications.quietHoursEndPlaceholder')" />
        </a-form-item>
        <a-form-item :label="t('settings.notifications.messageTemplate')" name="messageTemplate">
          <a-textarea v-model:value="form.messageTemplate" :rows="2" :placeholder="t('settings.notifications.messageTemplatePlaceholder')" />
        </a-form-item>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { PlusOutlined } from '@ant-design/icons-vue'
import { notificationsApi } from '@/api/notifications'

const { t } = useI18n()

const loading = ref(false)
const modalVisible = ref(false)
const submitting = ref(false)
const editingChannel = ref<Record<string, unknown> | null>(null)
const channels = ref<Record<string, unknown>[]>([])
const historyVisible = ref(false)
const historyLoading = ref(false)
const historyChannel = ref<Record<string, unknown> | null>(null)
const historyRecords = ref<Record<string, unknown>[]>([])

const form = reactive({
  name: '',
  type: 'telegram',
  config: '',
  enabled: true,
  events: [] as string[],
  maxErrorsPerHour: 100,
  timeoutMs: 10000,
  quietHoursStart: '',
  quietHoursEnd: '',
  messageTemplate: '',
})

const columns = [
  { title: t('common.name'), dataIndex: 'name', key: 'name' },
  { title: t('common.type'), key: 'type', width: 120 },
  { title: t('common.enabled'), key: 'enabled', width: 80 },
  { title: t('common.createdAt'), dataIndex: 'createdAt', key: 'createdAt', width: 180 },
  { title: t('common.actions'), key: 'actions', width: 260 },
]

const historyColumns = [
  { title: t('settings.notifications.colTime'), dataIndex: 'created_at', key: 'created_at', width: 180 },
  { title: t('settings.notifications.colEvent'), dataIndex: 'event', key: 'event', width: 150 },
  { title: t('settings.notifications.colTitle'), dataIndex: 'title', key: 'title', ellipsis: true },
  { title: t('settings.notifications.colSuccess'), key: 'success', width: 80 },
  { title: t('settings.notifications.colError'), dataIndex: 'error_msg', key: 'error_msg', ellipsis: true },
]

async function fetchChannels() {
  loading.value = true
  try {
    const resp = await notificationsApi.list()
    channels.value = resp.data.data?.items || resp.data.data || []
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    loading.value = false
  }
}

function openModal(record?: Record<string, unknown>) {
  editingChannel.value = record || null
  if (record) {
    Object.assign(form, {
      name: record.name,
      type: record.type,
      config: typeof record.config === 'string' ? record.config : JSON.stringify(record.config || {}, null, 2),
      enabled: record.enabled !== false,
      events: typeof record.events === 'string' ? record.events.split(',').map((s: string) => s.trim()).filter(Boolean) : [],
      maxErrorsPerHour: record.maxErrorsPerHour ?? 100,
      timeoutMs: record.timeoutMs ?? 10000,
      quietHoursStart: record.quietHoursStart || '',
      quietHoursEnd: record.quietHoursEnd || '',
      messageTemplate: record.messageTemplate || '',
    })
  } else {
    Object.assign(form, {
      name: '',
      type: 'telegram',
      config: '',
      enabled: true,
      events: [],
      maxErrorsPerHour: 100,
      timeoutMs: 10000,
      quietHoursStart: '',
      quietHoursEnd: '',
      messageTemplate: '',
    })
  }
  modalVisible.value = true
}

async function handleSubmit() {
  if (form.config && form.config.trim()) {
    try {
      JSON.parse(form.config)
    } catch {
      message.error(t('common.jsonFormatError'))
      return
    }
  }
  submitting.value = true
  try {
    const payload = {
      ...form,
      config: form.config ? form.config : '{}',
      events: form.events.length > 0 ? form.events.join(',') : '',
    }
    if (editingChannel.value) {
      await notificationsApi.update(editingChannel.value.id as number, payload)
    } else {
      await notificationsApi.create(payload)
    }
    message.success(t('common.operationSuccess'))
    modalVisible.value = false
    fetchChannels()
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    submitting.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await notificationsApi.delete(id)
    message.success(t('common.deleteSuccess'))
    fetchChannels()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function testChannel(id: number) {
  try {
    await notificationsApi.test(id)
    message.success(t('settings.notifications.testSent'))
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function toggleChannel(record: Record<string, unknown>) {
  try {
    await notificationsApi.update(record.id as number, { ...record, enabled: !record.enabled })
    message.success(t('settings.notifications.statusToggled'))
    fetchChannels()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function showHistory(record: Record<string, unknown>) {
  historyChannel.value = record
  historyVisible.value = true
  historyLoading.value = true
  try {
    const resp = await notificationsApi.listHistory(record.id as number)
    historyRecords.value = (resp.data.data?.items || resp.data.data || []).map((h: Record<string, unknown>) => ({
      ...h,
      success: h.success,
    }))
  } catch {
    historyRecords.value = []
  } finally {
    historyLoading.value = false
  }
}

onMounted(fetchChannels)
</script>
