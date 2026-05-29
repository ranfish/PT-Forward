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
            <a-tag v-if="i < 3">{{ keyLabels[c.key] || c.key }} {{ compareLabels[c.compare_type] || c.compare_type }} {{ formatValue(c) }}</a-tag>
          </span>
          <span v-if="(record.conditions || []).length > 3" style="color: #999">{{ t('settings.filterRules.moreConditions', { n: record.conditions.length - 3 }) }}</span>
          <span v-if="!(record.conditions || []).length" style="color: #999">{{ t('settings.filterRules.none') }}</span>
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button type="link" size="small" @click="openTestModal(record)">{{ t('common.test') }}</a-button>
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
              <div v-if="cond.key === 'size'" style="display: flex; gap: 8px">
                <a-input-number
                  v-model:value="cond.numValue"
                  style="flex: 1"
                  :min="0"
                  :placeholder="t('settings.filterRules.inputNumeric')"
                />
                <a-select v-model:value="cond.sizeUnit" style="width: 80px">
                  <a-select-option value="MB">MB</a-select-option>
                  <a-select-option value="GB">GB</a-select-option>
                </a-select>
              </div>
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
        </div>

        <a-button type="dashed" block style="margin-bottom: 16px" @click="addCondition">
          <template #icon><PlusOutlined /></template>
          {{ t('subscription.addCondition') }}
        </a-button>
      </a-form>
    </a-modal>

    <a-modal
      v-model:open="testModalVisible"
      :title="t('settings.filterRules.testRule')"
      width="560px"
      :footer="null"
    >
      <a-form layout="vertical">
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('subscription.title')">
              <a-input v-model:value="testForm.title" :placeholder="t('settings.filterRules.sampleTitle')" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('subscription.volumeSize')">
              <a-input-number v-model:value="testForm.size" style="width:100%" :min="0" :placeholder="t('settings.filterRules.bytesHint')" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('subscription.uploader')">
              <a-input v-model:value="testForm.uploader" :placeholder="t('settings.filterRules.uploaderName')" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('common.site')">
              <a-input v-model:value="testForm.siteName" :placeholder="t('settings.filterRules.siteNameHint')" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item :label="t('subscription.categoryLabel')">
              <a-input v-model:value="testForm.category" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item :label="t('subscription.isFree')">
              <a-select v-model:value="testForm.free">
                <a-select-option value="">{{ t('system.allLevels') }}</a-select-option>
                <a-select-option value="true">{{ t('common.yes') }}</a-select-option>
                <a-select-option value="false">{{ t('common.no') }}</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item :label="t('subscription.discountLevel')">
              <a-input v-model:value="testForm.discountLevel" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item :label="t('subscription.tagsLabel')">
          <a-input v-model:value="testForm.tags" :placeholder="t('settings.filterRules.tagsExample')" />
        </a-form-item>
      </a-form>
      <div style="text-align: right; margin-top: 12px">
        <a-button type="primary" :loading="testSubmitting" @click="runTest">{{ t('common.test') }}</a-button>
      </div>
      <a-alert v-if="testResult !== null" :type="testResult ? 'warning' : 'info'" show-icon style="margin-top: 12px">
        <template #message>
          {{ testResult ? t('settings.filterRules.testMatched') : t('settings.filterRules.testNotMatched') }}
        </template>
      </a-alert>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { PlusOutlined, DeleteOutlined } from '@ant-design/icons-vue'
import { filterRulesApi } from '@/api/filter-rules'
import type { FilterRule, RuleCondition } from '@/api/types'
import { usePagination } from '@/composables/usePagination'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const modalVisible = ref(false)
const submitting = ref(false)
const editingRule = ref<FilterRule | null>(null)

interface ConditionForm {
  key: string
  compareType: string
  value: string
  numValue: number
  sizeUnit: string
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
  { title: t('common.actions'), key: 'actions', width: 200 },
]

const pagination = usePagination<FilterRule>((page, size) => filterRulesApi.list(page, size))

function formatValue(c: RuleCondition) {
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
  form.conditionList.push({ key: 'title', compareType: 'contain', value: '', numValue: 0, sizeUnit: 'GB' })
}

function removeCondition(idx: number) {
  form.conditionList.splice(idx, 1)
}

function conditionsToForm(conditions: RuleCondition[]) {
  return (conditions || []).map((c) => ({
    key: c.key || 'title',
    compareType: c.compare_type || 'contain',
    value: c.value || '',
    numValue: c.key === 'size' ? (Number(c.value || 0) >= 1073741824 ? Number(c.value || 0) / 1073741824 : Number(c.value || 0) / 1048576) : 0,
    sizeUnit: c.key === 'size' ? (Number(c.value || 0) >= 1073741824 || Number(c.value || 0) === 0 ? 'GB' : 'MB') : 'GB',
  }))
}

function formToConditions() {
  return form.conditionList.map(c => {
    let sizeVal = c.numValue || 0
    if (c.key === 'size') {
      sizeVal = c.sizeUnit === 'GB' ? sizeVal * 1073741824 : sizeVal * 1048576
    }
    return {
      key: c.key,
      compare_type: c.compareType,
      value: c.key === 'size' ? String(Math.round(sizeVal)) : c.value,
    }
  })
}

function openModal(record?: FilterRule) {
  editingRule.value = record || null
  if (record) {
    form.conditionList = conditionsToForm(record.conditions)
    if (!form.conditionList.length) form.conditionList.push({ key: 'title', compareType: 'contain', value: '', numValue: 0, sizeUnit: 'GB' })
    Object.assign(form, {
      name: record.name || '',
      ruleType: record.ruleType || 'reject',
      priority: record.priority || 0,
      savePath: record.savePath || '',
      category: record.category || '',
      tags: record.tags || '',
      enabled: record.enabled ?? true,
    })
  } else {
    Object.assign(form, {
      name: '', ruleType: 'reject', priority: 0, savePath: '', category: '', tags: '',
      conditionList: [{ key: 'title', compareType: 'contain', value: '', numValue: 0, sizeUnit: 'GB' }], enabled: true,
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
      savePath: form.savePath,
      category: form.category,
      tags: form.tags,
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

async function toggleEnabled(record: FilterRule) {
  try {
    await filterRulesApi.update(record.id, { enabled: !record.enabled, name: record.name, conditions: record.conditions })
    pagination.fetch()
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}

const testModalVisible = ref(false)
const testSubmitting = ref(false)
const testResult = ref<boolean | null>(null)
const testingRuleId = ref(0)
const testForm = reactive({
  title: '',
  size: 0,
  uploader: '',
  siteName: '',
  category: '',
  free: '',
  tags: '',
  discountLevel: '',
})

function openTestModal(record: FilterRule) {
  testingRuleId.value = record.id
  testResult.value = null
  Object.assign(testForm, { title: '', size: 0, uploader: '', siteName: '', category: '', free: '', tags: '', discountLevel: '' })
  testModalVisible.value = true
}

async function runTest() {
  testSubmitting.value = true
  testResult.value = null
  try {
    const resp = await filterRulesApi.test(testingRuleId.value, {
      title: testForm.title || undefined,
      size: testForm.size || undefined,
      uploader: testForm.uploader || undefined,
      siteName: testForm.siteName || undefined,
      category: testForm.category || undefined,
      free: testForm.free || undefined,
      tags: testForm.tags || undefined,
      discountLevel: testForm.discountLevel || undefined,
    })
    const data = resp.data?.data
    testResult.value = !!data?.passed
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    testSubmitting.value = false
  }
}

onMounted(() => pagination.fetch())
</script>
