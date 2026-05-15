<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; justify-content: flex-end">
      <a-button type="primary" @click="openModal()">
        <template #icon><PlusOutlined /></template>
        {{ t('seeding.addRule') }}
      </a-button>
    </div>

    <a-table
      :columns="columns"
      :data-source="rules"
      :loading="loading"
      :pagination="false"
      row-key="id"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'enabled'">
          <a-switch :checked="record.enabled" @change="toggleRule(record)" />
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button type="link" size="small" @click="openModal(record)">{{ t('common.edit') }}</a-button>
            <a-button type="link" size="small" @click="testRule(record.id)">{{ t('common.test') }}</a-button>
            <a-popconfirm :title="t('seeding.deleteRuleConfirm')" @confirm="handleDelete(record.id)">
              <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="modalVisible"
      :title="editingRule ? t('seeding.editRule') : t('seeding.addRule')"
      :confirm-loading="submitting"
      width="640px"
      @ok="handleSubmit"
    >
      <a-form :model="form" layout="vertical">
        <a-form-item :label="t('seeding.ruleAlias')" name="alias" :rules="[{ required: true, message: t('seeding.pleaseInputRuleAlias') }]">
          <a-input v-model:value="form.alias" :placeholder="t('seeding.ruleAlias')" />
        </a-form-item>
        <a-form-item :label="t('common.type')" name="type">
          <a-select v-model:value="form.type" :placeholder="t('common.selectType')">
            <a-select-option value="normal">{{ t('seeding.normalRule') }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item :label="t('seeding.conditionsJson')" name="conditions">
          <a-textarea v-model:value="form.conditions" :rows="3" placeholder="[{&quot;field&quot;:&quot;seed_time&quot;,&quot;op&quot;:&quot;gt&quot;,&quot;value&quot;:720}]" />
        </a-form-item>
        <a-form-item :label="t('seeding.expression')" name="expr">
          <a-input v-model:value="form.expr" :placeholder="t('seeding.celExpressionOptional')" />
        </a-form-item>
        <a-form-item :label="t('seeding.action')" name="action">
          <a-select v-model:value="form.action" :placeholder="t('seeding.selectAction')">
            <a-select-option value="delete">{{ t('seeding.deleteTorrent') }}</a-select-option>
            <a-select-option value="pause">{{ t('seeding.pauseTorrent') }}</a-select-option>
            <a-select-option value="limit_speed">{{ t('seeding.limitSpeed') }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item :label="t('seeding.priority')" name="priority">
          <a-input-number v-model:value="form.priority" :min="0" style="width: 100%" />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('common.enable')">
              <a-switch v-model:checked="form.enabled" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('seeding.deleteNum')">
              <a-input-number v-model:value="form.delete_num" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item :label="t('seeding.removeData')">
          <a-switch v-model:checked="form.remove_data" />
        </a-form-item>
        <a-collapse :bordered="false" style="margin-top: 8px; background: transparent">
          <a-collapse-panel key="advanced" header="高级选项">
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="适应时间(秒)">
                  <a-input-number v-model:value="form.fit_time" :min="0" style="width: 100%" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item label="仅删种子(保留数据)">
                  <a-switch v-model:checked="form.only_delete_torrent" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="限速字节数">
                  <a-input-number v-model:value="form.limit_speed_bytes" :min="0" style="width: 100%" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item label="先 Reannounce">
                  <a-switch v-model:checked="form.reannounce_before" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="Reannounce 等待(ms)">
                  <a-input-number v-model:value="form.reannounce_wait_ms" :min="0" style="width: 100%" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item label="重试次数">
                  <a-input-number v-model:value="form.reannounce_retries" :min="0" style="width: 100%" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item label="重试间隔(ms)">
                  <a-input-number v-model:value="form.reannounce_interval_ms" :min="0" style="width: 100%" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item label="级联删除">
                  <a-switch v-model:checked="form.cascade_delete" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row v-if="form.cascade_delete" :gutter="16">
              <a-col :span="12">
                <a-form-item label="级联最大深度">
                  <a-input-number v-model:value="form.cascade_max_depth" :min="1" style="width: 100%" />
                </a-form-item>
              </a-col>
            </a-row>
          </a-collapse-panel>
        </a-collapse>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { PlusOutlined } from '@ant-design/icons-vue'
import { deleteRulesApi } from '@/api/seeding'
import type { DeleteRule } from '@/api/types'

const { t } = useI18n()
const loading = ref(false)
const modalVisible = ref(false)
const submitting = ref(false)
const editingRule = ref<DeleteRule | null>(null)
const rules = ref<DeleteRule[]>([])

const form = reactive({
  alias: '',
  type: 'normal',
  conditions: '',
  expr: '',
  action: 'delete',
  priority: 0,
  enabled: true,
  delete_num: 1,
  remove_data: true,
  fit_time: 0,
  only_delete_torrent: false,
  limit_speed_bytes: 0,
  reannounce_before: true,
  reannounce_wait_ms: 2000,
  reannounce_retries: 2,
  reannounce_interval_ms: 3000,
  cascade_delete: false,
  cascade_max_depth: 1,
})

const columns = [
  { title: t('seeding.alias'), dataIndex: 'alias', key: 'alias' },
  { title: t('common.type'), dataIndex: 'type', key: 'type', width: 80 },
  { title: t('seeding.condition'), dataIndex: 'conditions', key: 'conditions', ellipsis: true },
  { title: t('seeding.action'), dataIndex: 'action', key: 'action', width: 100 },
  { title: t('seeding.priority'), dataIndex: 'priority', key: 'priority', width: 80 },
  { title: t('common.enabledStatus'), key: 'enabled', width: 80 },
  { title: t('common.actions'), key: 'actions', width: 200 },
]

async function fetchRules() {
  loading.value = true
  try {
    const resp = await deleteRulesApi.list()
    rules.value = resp.data.data?.items || resp.data.data || []
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

function openModal(record?: DeleteRule) {
  editingRule.value = record || null
  if (record) {
    Object.assign(form, {
      alias: record.alias || '',
      type: record.type || 'normal',
      conditions: record.conditions || '',
      expr: record.expr || '',
      action: record.action || 'delete',
      priority: record.priority || 0,
      enabled: record.enabled ?? true,
      delete_num: record.delete_num || 1,
      remove_data: record.remove_data ?? true,
      fit_time: record.fit_time ?? 0,
      only_delete_torrent: record.only_delete_torrent || false,
      limit_speed_bytes: record.limit_speed_bytes ?? 0,
      reannounce_before: record.reannounce_before ?? true,
      reannounce_wait_ms: record.reannounce_wait_ms ?? 2000,
      reannounce_retries: record.reannounce_retries ?? 2,
      reannounce_interval_ms: record.reannounce_interval_ms ?? 3000,
      cascade_delete: record.cascade_delete || false,
      cascade_max_depth: record.cascade_max_depth ?? 1,
    })
  } else {
    Object.assign(form, { alias: '', type: 'normal', conditions: '', expr: '', action: 'delete', priority: 0, enabled: true, delete_num: 1, remove_data: true, fit_time: 0, only_delete_torrent: false, limit_speed_bytes: 0, reannounce_before: true, reannounce_wait_ms: 2000, reannounce_retries: 2, reannounce_interval_ms: 3000, cascade_delete: false, cascade_max_depth: 1 })
  }
  modalVisible.value = true
}

async function handleSubmit() {
  submitting.value = true
  try {
    if (editingRule.value) {
      await deleteRulesApi.update(editingRule.value.id, form)
    } else {
      await deleteRulesApi.create(form)
    }
    message.success(t('common.operationSuccess'))
    modalVisible.value = false
    fetchRules()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    submitting.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await deleteRulesApi.delete(id)
    message.success(t('common.deleted'))
    fetchRules()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function testRule(id: number) {
  try {
    const resp = await deleteRulesApi.test(id)
    const result = resp.data.data
    message.success(t('seeding.matchedResult', { matched: result?.matched?.length || 0, total: result?.total || 0 }))
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function toggleRule(record: DeleteRule) {
  try {
    await deleteRulesApi.update(record.id, { ...record, enabled: !record.enabled })
    message.success(t('common.statusToggled'))
    fetchRules()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

onMounted(fetchRules)
</script>
