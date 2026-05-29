<template>
  <div>
    <a-page-header :title="t('publish.exclusionRules')" :subtitle="t('publish.exclusionSubtitle')" @back="$router.push('/publish')">
      <template #extra>
        <a-button type="primary" @click="openModal()">
          <template #icon><PlusOutlined /></template>
          {{ t('publish.addExclusionRule') }}
        </a-button>
      </template>
    </a-page-header>

    <a-table
      :columns="columns"
      :data-source="exclusions"
      :loading="loading"
      :pagination="false"
      row-key="id"
      size="small"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'target_site'">
          <a-tag color="red">{{ record.target_site }}</a-tag>
        </template>
        <template v-if="column.key === 'source_site'">
          <a-tag color="blue">{{ record.source_site }}</a-tag>
        </template>
        <template v-if="column.key === 'actions'">
          <a-popconfirm :title="t('publish.deleteExclusionConfirm')" @confirm="handleDelete(record)">
            <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
          </a-popconfirm>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="modalVisible"
      :title="t('publish.addExclusionRule')"
      :confirm-loading="submitting"
      @ok="handleSubmit"
    >
      <a-form :model="form" layout="vertical">
        <a-form-item :label="t('publish.targetSite')" name="target_site" :rules="[{ required: true, message: t('publish.pleaseEnterTargetSite') }]">
          <a-input v-model:value="form.target_site" :placeholder="t('publish.targetSitePlaceholder')" />
        </a-form-item>
        <a-form-item :label="t('publish.sourceSite')" name="source_site" :rules="[{ required: true, message: t('publish.pleaseEnterSourceSite') }]">
          <a-input v-model:value="form.source_site" :placeholder="t('publish.sourceSitePlaceholder')" />
        </a-form-item>
      </a-form>
      <a-typography-text type="secondary">
        {{ t('publish.exclusionHint') }}
      </a-typography-text>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { PlusOutlined } from '@ant-design/icons-vue'
import { exclusionsApi } from '@/api/exclusions'
import { formatTime } from '@/utils/format'

const { t } = useI18n()
const loading = ref(false)
const exclusions = ref<Record<string, unknown>[]>([])
const modalVisible = ref(false)
const submitting = ref(false)

const form = reactive({
  target_site: '',
  source_site: '',
})

const columns = [
  { title: t('publish.targetSite'), key: 'target_site', width: 200 },
  { title: t('publish.sourceSite'), key: 'source_site', width: 200 },
  { title: t('common.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 200, customRender: ({ text }: { text: string }) => formatTime(text) },
  { title: t('common.actions'), key: 'actions', width: 100 },
]

async function fetchExclusions() {
  loading.value = true
  try {
    const resp = await exclusionsApi.list()
    exclusions.value = resp.data.data || []
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    loading.value = false
  }
}

function openModal() {
  form.target_site = ''
  form.source_site = ''
  modalVisible.value = true
}

async function handleSubmit() {
  if (!form.target_site || !form.source_site) {
    message.warning(t('publish.fillTargetAndSource'))
    return
  }
  submitting.value = true
  try {
    await exclusionsApi.create({ target_site: form.target_site, source_site: form.source_site })
    message.success(t('common.addSuccess'))
    modalVisible.value = false
    fetchExclusions()
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    submitting.value = false
  }
}

async function handleDelete(record: Record<string, unknown>) {
  try {
    await exclusionsApi.remove({ target_site: record.target_site as string, source_site: record.source_site as string })
    message.success(t('common.deleted'))
    fetchExclusions()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

onMounted(() => fetchExclusions())
</script>
