<template>
  <div>
    <a-alert type="info" show-icon style="margin-bottom: 16px">
      <template #message>{{ t('settings.filterRules.globalExclusionHint') }}</template>
    </a-alert>

    <div style="margin-bottom: 16px; display: flex; justify-content: flex-end">
      <a-button type="primary" @click="openModal()">
        <template #icon><PlusOutlined /></template>
        {{ t('settings.filterRules.addExclusionRule') }}
      </a-button>
    </div>

    <a-table
      :columns="columns"
      :data-source="pagination.data.value"
      :loading="pagination.loading.value"
      :pagination="{
        current: pagination.currentPage.value,
        pageSize: pagination.pageSize.value,
        total: pagination.total.value,
        showSizeChanger: true,
        showTotal: (total: number) => t('common.totalCount', { count: total }),
      }"
      row-key="id"
      @change="(pag: { current: number; pageSize: number }) => pagination.onPageChange(pag.current, pag.pageSize)"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'enabled'">
          <a-switch :checked="record.enabled" @change="toggleEnabled(record)" />
        </template>
        <template v-if="column.key === 'conditions'">
          <span v-for="(c, i) in (record.conditions || [])" :key="i">
            <a-tag v-if="i < 3">{{ keyLabels[c.key] || c.key }} {{ compareLabels[c.compare_type || c.compareType] || c.compare_type || c.compareType }} {{ formatValue(c) }}</a-tag>
          </span>
          <span v-if="(record.conditions || []).length > 3" style="color: #999">{{ t('settings.filterRules.moreConditions', { n: record.conditions.length - 3 }) }}</span>
          <span v-if="!(record.conditions || []).length" style="color: #999">{{ t('settings.filterRules.none') }}</span>
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button type="link" size="small" @click="openModal(record)">{{ t('common.edit') }}</a-button>
            <a-popconfirm :title="t('settings.filterRules.confirmDeleteThisRule')" @confirm="handleDelete(record.id)">
              <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="modalVisible"
      :title="editingRule ? t('settings.filterRules.editExclusionRule') : t('settings.filterRules.addExclusionRule')"
      :confirm-loading="submitting"
      width="720px"
      @ok="handleSubmit"
    >
      <a-form :model="form" layout="vertical">
        <a-row :gutter="16">
          <a-col :span="16">
            <a-form-item :label="t('settings.filterRules.ruleName')" name="name" :rules="[{ required: true, message: t('settings.filterRules.ruleNameRequired') }]">
              <a-input v-model:value="form.name" :placeholder="t('settings.filterRules.ruleNamePlaceholder')" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item :label="t('common.enable')">
              <a-switch v-model:checked="form.enabled" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item :label="t('settings.filterRules.ruleType')">
              <a-select v-model:value="form.ruleType">
                <a-select-option value="accept">{{ t('settings.filterRules.accept') }}</a-select-option>
                <a-select-option value="reject">{{ t('settings.filterRules.reject') }}</a-select-option>
                <a-select-option value="accept_and_reject">{{ t('settings.filterRules.acceptAndReject') }}</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item :label="t('settings.filterRules.priority')">
              <a-input-number v-model:value="form.priority" :min="0" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item :label="t('settings.filterRules.tags')">
              <a-input v-model:value="form.tags" :placeholder="t('settings.filterRules.tagsPlaceholder')" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('settings.filterRules.savePath')">
              <a-input v-model:value="form.savePath" :placeholder="t('settings.filterRules.savePathPlaceholder')" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('settings.filterRules.category')">
              <a-input v-model:value="form.category" :placeholder="t('settings.filterRules.categoryPlaceholder')" />
            </a-form-item>
          </a-col>
        </a-row>

        <a-divider>{{ t('settings.filterRules.matchConditionHint') }}</a-divider>

        <div v-for="(cond, idx) in form.conditionList" :key="idx" style="margin-bottom: 12px">
          <a-row :gutter="8" align="middle">
            <a-col :span="6">
              <a-select v-model:value="cond.key" :placeholder="t('subscription.selectField')" style="width:100%">
                <a-select-option value="title">{{ t('subscription.title') }}</a-select-option>
                <a-select-option value="size">{{ t('subscription.volumeSize') }}</a-select-option>
                <a-select-option value="uploader">{{ t('subscription.uploader') }}</a-select-option>
                <a-select-option value="site">{{ t('common.site') }}</a-select-option>
                <a-select-option value="category">{{ t('subscription.categoryLabel') }}</a-select-option>
                <a-select-option value="free">{{ t('subscription.isFree') }}</a-select-option>
                <a-select-option value="tags">{{ t('subscription.tagsLabel') }}</a-select-option>
                <a-select-option value="discount_level">{{ t('subscription.discountLevel') }}</a-select-option>
              </a-select>
            </a-col>
            <a-col :span="6">
              <a-select v-model:value="cond.compareType" :placeholder="t('subscription.compareMethod')" style="width:100%">
                <a-select-option v-for="opt in compareOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</a-select-option>
              </a-select>
            </a-col>
            <a-col :span="10">
              <a-input-number
                v-if="cond.key === 'size'"
                v-model:value="cond.numValue"
                style="width:100%"
                :min="0"
                :placeholder="t('settings.filterRules.inputNumeric')"
              />
              <a-select
                v-else-if="cond.key === 'free'"
                v-model:value="cond.value"
                style="width:100%"
                :placeholder="t('settings.filterRules.selectOption')"
              >
                <a-select-option value="true">{{ t('subscription.yesFree') }}</a-select-option>
                <a-select-option value="false">{{ t('subscription.noNotFree') }}</a-select-option>
              </a-select>
              <a-input v-else v-model:value="cond.value" :placeholder="t('subscription.inputValue')" style="width:100%" />
            </a-col>
            <a-col :span="2">
              <a-button type="text" danger :disabled="form.conditionList.length <= 1" @click="removeCondition(idx)">
                <template #icon><DeleteOutlined /></template>
              </a-button>
            </a-col>
          </a-row>
          <div v-if="cond.key === 'size'" style="margin-top: 4px; color: #999; font-size: 12px; padding-left: 4px">
            {{ t('settings.filterRules.unitBytesDetail') }}
          </div>
        </div>

        <a-button type="dashed" block style="margin-bottom: 16px" @click="addCondition">
          <template #icon><PlusOutlined /></template>
          {{ t('subscription.addCondition') }}
        </a-button>
      </a-form>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons-vue'
import { filterRulesApi } from '@/api/filter-rules'
import { usePagination } from '@/composables/usePagination'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

interface ConditionData {
  key: string
  compare_type?: string
  compareType?: string
  value: string
}

interface FilterRuleItem {
  [key: string]: unknown
  id: number
  name: string
  enabled: boolean
  conditions: ConditionData[]
}

const modalVisible = ref(false)
const submitting = ref(false)
const editingRule = ref<FilterRuleItem | null>(null)

interface ConditionForm {
  key: string
  compareType: string
  value: string
  numValue: number
}

const form = reactive({
  name: '',
  ruleType: 'reject',
  priority: 0,
  savePath: '',
  category: '',
  tags: '',
  conditionList: [] as ConditionForm[],
  enabled: true,
})

const keyLabels: Record<string, string> = {
  title: t('subscription.title'), size: t('subscription.volumeSize'), uploader: t('subscription.uploader'), site: t('common.site'),
  category: t('subscription.categoryLabel'), free: t('subscription.isFree'), tags: t('subscription.tagsLabel'),
}

const compareLabels: Record<string, string> = {
  equals: t('subscription.equals'), bigger: t('subscription.greaterThan'), smaller: t('subscription.lessThan'),
  contain: t('subscription.contains'), not_contain: t('subscription.notContains'),
  include_in: t('settings.filterRules.includeIn'), not_include_in: t('settings.filterRules.notIncludeIn'),
  regexp: t('subscription.regexpMatch'), not_regexp: t('settings.filterRules.notRegexp'),
}

const compareOptions = [
  { value: 'equals', label: t('subscription.equals') },
  { value: 'bigger', label: t('subscription.greaterThan') },
  { value: 'smaller', label: t('subscription.lessThan') },
  { value: 'contain', label: t('subscription.contains') },
  { value: 'not_contain', label: t('subscription.notContains') },
  { value: 'include_in', label: t('settings.filterRules.includeIn') },
  { value: 'not_include_in', label: t('settings.filterRules.notIncludeIn') },
  { value: 'regexp', label: t('subscription.regexpMatch') },
  { value: 'not_regexp', label: t('settings.filterRules.notRegexp') },
]

const columns = [
  { title: t('common.name'), dataIndex: 'name', key: 'name' },
  { title: t('settings.filterRules.exclusionCondition'), key: 'conditions' },
  { title: t('common.enable'), key: 'enabled', width: 80, align: 'center' as const },
  { title: t('common.actions'), key: 'actions', width: 140 },
]

const pagination = usePagination((page, size) => filterRulesApi.list(page, size))

function formatValue(c: ConditionData) {
  const v = c.value || ''
  if (c.key === 'size' && v) {
    const n = Number(v)
    if (n >= 1073741824) return (n / 1073741824).toFixed(1) + ' GB'
    if (n >= 1048576) return (n / 1048576).toFixed(0) + ' MB'
    return v + ' B'
  }
  return v
}

function addCondition() {
  form.conditionList.push({ key: 'title', compareType: 'contain', value: '', numValue: 0 })
}

function removeCondition(idx: number) {
  form.conditionList.splice(idx, 1)
}

function conditionsToForm(conditions: ConditionData[]) {
  return (conditions || []).map((c: ConditionData) => ({
    key: c.key || 'title',
    compareType: c.compare_type || c.compareType || 'contain',
    value: c.value || '',
    numValue: c.key === 'size' ? Number(c.value || 0) : 0,
  }))
}

function formToConditions() {
  return form.conditionList.map(c => ({
    key: c.key,
    compare_type: c.compareType,
    value: c.key === 'size' ? String(c.numValue || 0) : c.value,
  }))
}

function openModal(record?: FilterRuleItem) {
  editingRule.value = record || null
  if (record) {
    form.conditionList = conditionsToForm(record.conditions)
    if (!form.conditionList.length) form.conditionList.push({ key: 'title', compareType: 'contain', value: '', numValue: 0 })
    Object.assign(form, {
      name: record.name || '',
      ruleType: record.ruleType || record.rule_type || 'reject',
      priority: record.priority || 0,
      savePath: record.savePath || record.save_path || '',
      category: record.category || '',
      tags: record.tags || '',
      enabled: record.enabled ?? true,
    })
  } else {
    Object.assign(form, {
      name: '', ruleType: 'reject', priority: 0, savePath: '', category: '', tags: '',
      conditionList: [{ key: 'title', compareType: 'contain', value: '', numValue: 0 }], enabled: true,
    })
  }
  modalVisible.value = true
}

async function handleSubmit() {
  if (!form.name.trim()) {
    message.error(t('settings.filterRules.ruleNameRequired'))
    return
  }
  if (!form.conditionList.length) {
    message.error(t('settings.filterRules.atLeastOneCondition'))
    return
  }
  submitting.value = true
  try {
    const payload = {
      name: form.name,
      ruleType: form.ruleType,
      priority: form.priority,
      savePath: form.savePath || undefined,
      category: form.category || undefined,
      tags: form.tags || undefined,
      conditions: formToConditions(),
      enabled: form.enabled,
    }
    if (editingRule.value) {
      await filterRulesApi.update(editingRule.value.id, payload)
    } else {
      await filterRulesApi.create(payload)
    }
    message.success(t('common.operationSuccess'))
    modalVisible.value = false
    pagination.fetch()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  } finally {
    submitting.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await filterRulesApi.delete(id)
    message.success(t('common.deleteSuccess'))
    pagination.fetch()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

async function toggleEnabled(record: FilterRuleItem) {
  try {
    await filterRulesApi.update(record.id, { ...record, enabled: !record.enabled })
    pagination.fetch()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

onMounted(() => pagination.fetch())
</script>
