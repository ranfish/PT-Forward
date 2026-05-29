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
        <template v-else-if="column.key === 'action'">
          {{ translateAction(record.action) }}
        </template>
        <template v-else-if="column.key === 'type'">
          {{ translateType(record.type) }}
        </template>
        <template v-else-if="column.key === 'conditions'">
          <template v-if="record.type === 'expr'">
            <a-tag color="purple">{{ t('seeding.exprType') }}</a-tag>
            <span style="color: #666; font-size: 12px">{{ truncate(record.expr, 40) }}</span>
          </template>
          <template v-else>
            <template v-for="(c, i) in parseConditions(record.conditions)" :key="i">
              <a-tag style="margin-bottom: 2px">{{ fieldLabel(c.field) }} {{ opSymbol(c.operator) }} {{ c.value }}{{ fieldMeta[c.field]?.unit === 'hours' ? 'h' : '' }}</a-tag>
              <span v-if="i < parseConditions(record.conditions).length - 1" style="color: #999; font-size: 11px; margin: 0 2px">{{ record.logic === 'or' ? t('seeding.logicOrShort') : t('seeding.logicAndShort') }}</span>
            </template>
            <span v-if="!record.conditions" style="color: #999">-</span>
          </template>
        </template>
        <template v-if="column.key === 'delete_num'">
          {{ record.delete_num > 0 ? record.delete_num : t('seeding.unlimited') }}
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
            <a-select-option value="expr">{{ t('seeding.expression') }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item v-if="form.type !== 'expr'" :label="t('seeding.conditionLogic')" name="logic">
          <a-radio-group v-model:value="form.logic" size="small">
            <a-radio-button value="and">{{ t('seeding.logicAnd') }}</a-radio-button>
            <a-radio-button value="or">{{ t('seeding.logicOr') }}</a-radio-button>
          </a-radio-group>
        </a-form-item>
        <a-form-item v-if="form.type !== 'expr'" :label="t('seeding.conditionsJson')" name="conditions">
          <div style="margin-bottom: 8px; display: flex; gap: 8px">
            <a-button size="small" @click="addCondition">{{ t('seeding.addCondition') }}</a-button>
            <a-button size="small" type="dashed" @click="showRawJson = !showRawJson">{{ showRawJson ? t('seeding.hideRawJson') : t('seeding.showRawJson') }}</a-button>
          </div>
          <div v-for="(cond, idx) in conditions" :key="idx" style="display: flex; gap: 8px; margin-bottom: 8px; align-items: center;">
            <a-select v-model:value="cond.field" show-search option-filter-prop="label" style="width: 180px" :placeholder="t('seeding.selectField')">
              <a-select-option v-for="f in availableFields" :key="f.value" :value="f.value" :label="f.label">
                {{ f.label }} <span style="color: #999; font-size: 11px">({{ f.value }})</span>
              </a-select-option>
            </a-select>
            <a-select v-model:value="cond.operator" style="width: 100px" :placeholder="t('seeding.selectOperator')">
              <a-select-option v-for="op in applicableOperators(cond.field)" :key="op.value" :value="op.value">{{ op.label }}</a-select-option>
            </a-select>
            <template v-if="fieldType(cond.field) === 'bool'">
              <a-switch style="flex: 1" :checked="cond.value === 'true'" @change="(v: boolean) => cond.value = String(v)" />
            </template>
            <template v-else-if="fieldType(cond.field) === 'enum:status'">
              <a-select v-model:value="cond.value" style="flex: 1" :placeholder="t('seeding.conditionValue')">
                <a-select-option v-for="v in statusOptions" :key="v.value" :value="v.value">{{ v.label }}</a-select-option>
              </a-select>
            </template>
            <template v-else-if="fieldType(cond.field) === 'enum:discount'">
              <a-select v-model:value="cond.value" style="flex: 1" :placeholder="t('seeding.conditionValue')">
                <a-select-option v-for="v in discountOptions" :key="v.value" :value="v.value">{{ v.label }}</a-select-option>
              </a-select>
            </template>
            <template v-else>
              <a-input v-model:value="cond.value" style="flex: 1" :placeholder="fieldHint(cond.field)" />
            </template>
            <a-button type="link" danger size="small" @click="removeCondition(idx)">{{ t('common.delete') }}</a-button>
          </div>
          <a-textarea v-if="showRawJson || conditions.length === 0" v-model:value="form.conditions" :rows="2" :placeholder="'[{&quot;field&quot;:&quot;seed_time&quot;,&quot;operator&quot;:&quot;>&quot;,&quot;value&quot;:&quot;720&quot;}]'" style="margin-top: 4px" />
        </a-form-item>
        <a-form-item v-if="form.type === 'expr'" :label="t('seeding.expression')" name="expr">
          <a-textarea v-model:value="form.expr" :rows="3" :placeholder="t('seeding.celExpressionOptional')" />
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
              <a-input-number v-model:value="form.delete_num" :min="0" style="width: 100%" />
              <div style="font-size: 11px; color: #999; margin-top: 2px">{{ t('seeding.deleteNumHint') }}</div>
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item :label="t('seeding.removeData')">
          <a-switch v-model:checked="form.remove_data" />
        </a-form-item>
        <a-collapse :bordered="false" style="margin-top: 8px; background: transparent">
          <a-collapse-panel key="advanced" :header="t('seeding.advancedOptions')">
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item :label="t('seeding.fitTime')">
                  <a-input-number v-model:value="form.fit_time" :min="0" style="width: 100%" />
                  <div style="font-size: 11px; color: #999; margin-top: 2px">{{ t('seeding.fitTimeHint') }}</div>
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item :label="t('seeding.onlyDeleteTorrent')">
                  <a-switch v-model:checked="form.only_delete_torrent" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item :label="t('seeding.limitSpeedBytes')">
                  <a-input-number v-model:value="form.limit_speed_mb" :min="0" style="width: calc(100% - 50px)" />
                  <span style="margin-left: 8px; color: #999">MB/s</span>
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item :label="t('seeding.reannounceBefore')">
                  <a-switch v-model:checked="form.reannounce_before" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item :label="t('seeding.reannounceWaitMs')">
                  <a-input-number v-model:value="form.reannounce_wait_ms" :min="0" style="width: 100%" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item :label="t('seeding.reannounceRetries')">
                  <a-input-number v-model:value="form.reannounce_retries" :min="0" style="width: 100%" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row :gutter="16">
              <a-col :span="12">
                <a-form-item :label="t('seeding.reannounceIntervalMs')">
                  <a-input-number v-model:value="form.reannounce_interval_ms" :min="0" style="width: 100%" />
                </a-form-item>
              </a-col>
              <a-col :span="12">
                <a-form-item :label="t('seeding.cascadeDelete')">
                  <a-switch v-model:checked="form.cascade_delete" />
                </a-form-item>
              </a-col>
            </a-row>
            <a-row v-if="form.cascade_delete" :gutter="16">
              <a-col :span="12">
                <a-form-item :label="t('seeding.cascadeMaxDepth')">
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
const showRawJson = ref(false)

const actionLabels: Record<string, string> = {
  delete: t('seeding.deleteTorrent'),
  pause: t('seeding.pauseTorrent'),
  limit_speed: t('seeding.limitSpeed'),
}

function translateAction(action: string): string {
  return actionLabels[action] || action
}

const typeLabels: Record<string, string> = {
  normal: t('seeding.conditionType'),
  expr: t('seeding.exprType'),
}

function translateType(type: string): string {
  return typeLabels[type] || type
}

const modalVisible = ref(false)
const submitting = ref(false)
const editingRule = ref<DeleteRule | null>(null)
const rules = ref<DeleteRule[]>([])

const form = reactive({
  alias: '',
  type: 'normal',
  logic: 'and',
  conditions: '',
  expr: '',
  action: 'delete',
  priority: 0,
  enabled: true,
  delete_num: 1,
  remove_data: true,
  fit_time: 0,
  only_delete_torrent: false,
  limit_speed_mb: 0,
  reannounce_before: true,
  reannounce_wait_ms: 2000,
  reannounce_retries: 2,
  reannounce_interval_ms: 3000,
  cascade_delete: false,
  cascade_max_depth: 1,
})

interface ConditionItem { field: string; operator: string; value: string }
const conditions = ref<ConditionItem[]>([])

const statusOptions = [
  { value: 'seeding', label: t('seeding.seedingStatus') },
  { value: 'paused_free_end', label: t('seeding.pausedFreeEnd') },
  { value: 'paused_rule', label: t('seeding.pausedRule') },
  { value: 'stopped', label: t('seeding.stoppedStatus') },
]

const discountOptions = [
  { value: 'free', label: t('seeding.free') },
  { value: '2x_free', label: '2x Free' },
  { value: '2x_up', label: '2x Up' },
  { value: '2x_50', label: '50% + 2x Up' },
  { value: '50%', label: '50%' },
  { value: '30%', label: '30%' },
  { value: 'none', label: t('seeding.noDiscount') },
]

const fieldMeta: Record<string, { type: string; hint: string; unit?: string }> = {
  site_name: { type: 'text', hint: '' },
  status: { type: 'enum:status', hint: '' },
  is_free: { type: 'bool', hint: '' },
  has_hr: { type: 'bool', hint: '' },
  hr_seed_time_h: { type: 'number', hint: t('seeding.unitHours') },
  discount: { type: 'enum:discount', hint: '' },
  free_level: { type: 'text', hint: '' },
  client_id: { type: 'text', hint: '' },
  torrent_id: { type: 'text', hint: '' },
  source: { type: 'text', hint: '' },
  last_action_by: { type: 'text', hint: '' },
  name: { type: 'text', hint: '' },
  seed_time: { type: 'number', hint: t('seeding.unitHours'), unit: 'hours' },
  size: { type: 'number', hint: t('seeding.unitBytes') },
  totalSize: { type: 'number', hint: t('seeding.unitBytes') },
  progress: { type: 'number', hint: '0~1' },
  state: { type: 'text', hint: '' },
  uploaded: { type: 'number', hint: t('seeding.unitBytes') },
  uploadSpeed: { type: 'number', hint: 'B/s' },
  downloadSpeed: { type: 'number', hint: 'B/s' },
  ratio: { type: 'number', hint: '' },
  downloadUploadRatio: { type: 'number', hint: '' },
  category: { type: 'text', hint: '' },
  tags: { type: 'text', hint: '' },
  savePath: { type: 'text', hint: '' },
  seeder: { type: 'number', hint: '' },
  leecher: { type: 'number', hint: '' },
  is_finished: { type: 'bool', hint: '' },
  addedTime: { type: 'number', hint: t('seeding.unitHours'), unit: 'hours' },
  freeRemainSec: { type: 'number', hint: t('seeding.unitHours'), unit: 'hours' },
  hrRemainSec: { type: 'number', hint: t('seeding.unitHours'), unit: 'hours' },
  freeSpace: { type: 'number', hint: t('seeding.unitBytes') },
  scoringScore: { type: 'number', hint: '' },
  scoringRank: { type: 'number', hint: '' },
  lowScoreCount: { type: 'number', hint: '' },
  hour: { type: 'number', hint: '0~23' },
  activeUploads: { type: 'number', hint: '' },
  activeDownloads: { type: 'number', hint: '' },
  globalUploadSpeed: { type: 'number', hint: 'B/s' },
  globalDownloadSpeed: { type: 'number', hint: 'B/s' },
}

function fieldType(field: string): string {
  return fieldMeta[field]?.type || 'text'
}

function fieldHint(field: string): string {
  return fieldMeta[field]?.hint || t('seeding.conditionValue')
}

function applicableOperators(field: string): { value: string; label: string }[] {
  const ft = fieldType(field)
  if (ft === 'bool') {
    return [{ value: '==', label: '=' }]
  }
  if (ft === 'number') {
    return [
      { value: '>', label: '>' },
      { value: '>=', label: '≥' },
      { value: '<', label: '<' },
      { value: '<=', label: '≤' },
      { value: '==', label: '=' },
      { value: '!=', label: '≠' },
    ]
  }
  if (ft.startsWith('enum:')) {
    return [
      { value: '==', label: '=' },
      { value: '!=', label: '≠' },
    ]
  }
  return [
    { value: '==', label: '=' },
    { value: '!=', label: '≠' },
    { value: 'contains', label: t('seeding.opContains') },
    { value: 'not_contains', label: t('seeding.opNotContains') },
    { value: 'includeIn', label: t('seeding.opIncludeIn') },
    { value: 'notIncludeIn', label: t('seeding.opNotIncludeIn') },
    { value: 'regExp', label: t('seeding.opRegExp') },
    { value: 'notRegExp', label: t('seeding.opNotRegExp') },
  ]
}

function fieldLabel(field: string): string {
  const f = availableFields.find(x => x.value === field)
  return f ? f.label : field
}

function opSymbol(op: string): string {
  const map: Record<string, string> = { '==': '=', '!=': '≠', '>=': '≥', '<=': '≤', '>': '>', '<': '<', 'contains': '∋', 'not_contains': '∌', 'includeIn': 'in', 'notIncludeIn': '∉', 'regExp': '~', 'notRegExp': '!~' }
  return map[op] || op
}

const availableFields = [
  { value: 'site_name', label: t('seeding.fieldSiteName') },
  { value: 'status', label: t('seeding.fieldStatus') },
  { value: 'is_free', label: t('seeding.fieldIsFree') },
  { value: 'free_level', label: t('seeding.fieldFreeLevel') },
  { value: 'has_hr', label: t('seeding.fieldHasHR') },
  { value: 'hr_seed_time_h', label: t('seeding.fieldHRSeedTime') },
  { value: 'discount', label: t('seeding.fieldDiscount') },
  { value: 'client_id', label: t('seeding.fieldClientID') },
  { value: 'torrent_id', label: t('seeding.fieldTorrentID') },
  { value: 'source', label: t('seeding.fieldSource') },
  { value: 'last_action_by', label: t('seeding.fieldLastActionBy') },
  { value: 'name', label: t('seeding.fieldTorrentName') },
  { value: 'seed_time', label: t('seeding.fieldSeedTime') },
  { value: 'size', label: t('seeding.fieldSize') },
  { value: 'totalSize', label: t('seeding.fieldTotalSize') },
  { value: 'progress', label: t('seeding.fieldProgress') },
  { value: 'state', label: t('seeding.fieldState') },
  { value: 'uploaded', label: t('seeding.fieldUploaded') },
  { value: 'uploadSpeed', label: t('seeding.fieldUploadSpeed') },
  { value: 'downloadSpeed', label: t('seeding.fieldDownloadSpeed') },
  { value: 'ratio', label: t('seeding.fieldRatio') },
  { value: 'downloadUploadRatio', label: t('seeding.fieldDownloadUploadRatio') },
  { value: 'category', label: t('seeding.fieldCategory') },
  { value: 'tags', label: t('seeding.fieldTags') },
  { value: 'savePath', label: t('seeding.fieldSavePath') },
  { value: 'seeder', label: t('seeding.fieldSeeder') },
  { value: 'leecher', label: t('seeding.fieldLeecher') },
  { value: 'is_finished', label: t('seeding.fieldIsFinished') },
  { value: 'addedTime', label: t('seeding.fieldAddedTime') },
  { value: 'freeRemainSec', label: t('seeding.fieldFreeRemain') },
  { value: 'hrRemainSec', label: t('seeding.fieldHRRemain') },
  { value: 'freeSpace', label: t('seeding.fieldFreeSpace') },
  { value: 'scoringScore', label: t('seeding.fieldScoringScore') },
  { value: 'scoringRank', label: t('seeding.fieldScoringRank') },
  { value: 'lowScoreCount', label: t('seeding.fieldLowScoreCount') },
  { value: 'hour', label: t('seeding.fieldHour') },
  { value: 'activeUploads', label: t('seeding.fieldActiveUploads') },
  { value: 'activeDownloads', label: t('seeding.fieldActiveDownloads') },
  { value: 'globalUploadSpeed', label: t('seeding.fieldGlobalUploadSpeed') },
  { value: 'globalDownloadSpeed', label: t('seeding.fieldGlobalDownloadSpeed') },
]

function addCondition() {
  conditions.value.push({ field: '', operator: '==', value: '' })
}

function removeCondition(idx: number) {
  conditions.value.splice(idx, 1)
}

function conditionsToJSON(): string {
  if (conditions.value.length === 0) return ''
  return JSON.stringify(conditions.value.map(c => {
    if (fieldMeta[c.field]?.unit === 'hours' && c.value) {
      return { ...c, value: String(Math.round(Number(c.value) * 3600)) }
    }
    return c
  }))
}

function parseConditions(json: string): ConditionItem[] {
  if (!json) return []
  try {
    const parsed = JSON.parse(json)
    return parsed.map((c: ConditionItem) => {
      if (fieldMeta[c.field]?.unit === 'hours' && c.value) {
        const hours = Number(c.value) / 3600
        return { ...c, value: String(Math.round(hours * 100) / 100) }
      }
      return c
    })
  } catch {
    return []
  }
}

function truncate(s: string, n: number): string {
  if (!s) return ''
  return s.length > n ? s.substring(0, n) + '...' : s
}

const columns = [
  { title: t('seeding.alias'), dataIndex: 'alias', key: 'alias', width: 120 },
  { title: t('seeding.condition'), key: 'conditions' },
  { title: t('seeding.action'), dataIndex: 'action', key: 'action', width: 90 },
  { title: t('seeding.deleteNum'), key: 'delete_num', width: 80 },
  { title: t('seeding.priority'), dataIndex: 'priority', key: 'priority', width: 70 },
  { title: t('common.enabledStatus'), key: 'enabled', width: 70 },
  { title: t('common.actions'), key: 'actions', width: 180 },
]

async function fetchRules() {
  loading.value = true
  try {
    const resp = await deleteRulesApi.list()
    const body = resp.data.data
    rules.value = body?.items ?? []
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    loading.value = false
  }
}

function openModal(record?: DeleteRule) {
  editingRule.value = record || null
  showRawJson.value = false
  if (record) {
    Object.assign(form, {
      alias: record.alias || '',
      type: record.type || 'normal',
      logic: record.logic === 'or' ? 'or' : 'and',
      conditions: record.conditions || '',
      expr: record.expr || '',
      action: record.action || 'delete',
      priority: record.priority || 0,
      enabled: record.enabled ?? true,
      delete_num: record.delete_num ?? 1,
      remove_data: record.remove_data ?? true,
      fit_time: record.fit_time ?? 0,
      only_delete_torrent: record.only_delete_torrent || false,
      limit_speed_mb: Math.round((record.limit_speed_bytes ?? 0) / 1048576 * 100) / 100,
      reannounce_before: record.reannounce_before ?? true,
      reannounce_wait_ms: record.reannounce_wait_ms ?? 2000,
      reannounce_retries: record.reannounce_retries ?? 2,
      reannounce_interval_ms: record.reannounce_interval_ms ?? 3000,
      cascade_delete: record.cascade_delete || false,
      cascade_max_depth: record.cascade_max_depth ?? 1,
    })
    conditions.value = parseConditions(record.conditions || '')
  } else {
    Object.assign(form, { alias: '', type: 'normal', logic: 'and', conditions: '', expr: '', action: 'delete', priority: 0, enabled: true, delete_num: 1, remove_data: true, fit_time: 0, only_delete_torrent: false, limit_speed_mb: 0, reannounce_before: true, reannounce_wait_ms: 2000, reannounce_retries: 2, reannounce_interval_ms: 3000, cascade_delete: false, cascade_max_depth: 1 })
    conditions.value = []
  }
  modalVisible.value = true
}

async function handleSubmit() {
  submitting.value = true
  try {
    if (form.type !== 'expr' && conditions.value.length > 0) {
      form.conditions = conditionsToJSON()
    }
    const payload = { ...form, limit_speed_bytes: Math.round((form.limit_speed_mb || 0) * 1048576) }
    delete (payload as Record<string, unknown>).limit_speed_mb
    if (editingRule.value) {
      await deleteRulesApi.update(editingRule.value.id, payload)
    } else {
      await deleteRulesApi.create(payload)
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
    message.success(t('seeding.matchedResult', { matched: result?.matched ? 1 : 0, total: result?.torrentsAffected || 0 }))
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
