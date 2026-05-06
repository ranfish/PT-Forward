<template>
  <div>
    <div style="margin-bottom: 16px; display: flex; justify-content: flex-end">
      <a-button type="primary" @click="openModal()">
        <template #icon><PlusOutlined /></template>
        {{ t('settings.filterRules.addRule') }}
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
        showTotal: (total: number) => `共 ${total} 条`,
      }"
      row-key="id"
      @change="(pag: any) => pagination.onPageChange(pag.current, pag.pageSize)"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'ruleType'">
          <a-tag :color="record.ruleType === 'accept' ? 'green' : record.ruleType === 'reject' ? 'red' : 'blue'">
            {{ record.ruleType }}
          </a-tag>
        </template>
        <template v-if="column.key === 'actions'">
          <a-space>
            <a-button type="link" size="small" @click="openModal(record)">{{ t('common.edit') }}</a-button>
            <a-popconfirm :title="t('settings.filterRules.deleteConfirm')" @confirm="handleDelete(record.id)">
              <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
            </a-popconfirm>
          </a-space>
        </template>
      </template>
    </a-table>

    <a-modal
      v-model:open="modalVisible"
      :title="editingRule ? t('settings.filterRules.editRule') : t('settings.filterRules.addRule')"
      @ok="handleSubmit"
      :confirm-loading="submitting"
      width="640px"
    >
      <a-form :model="form" layout="vertical">
        <a-form-item :label="t('settings.filterRules.ruleName')" name="name" :rules="[{ required: true, message: t('settings.filterRules.ruleNameRequired') }]">
          <a-input v-model:value="form.name" placeholder="规则名称" />
        </a-form-item>
        <a-form-item :label="t('settings.filterRules.ruleType')" name="ruleType">
          <a-select v-model:value="form.ruleType" :placeholder="t('settings.filterRules.selectRuleType')">
            <a-select-option value="accept">{{ t('settings.filterRules.accept') }}</a-select-option>
            <a-select-option value="reject">{{ t('settings.filterRules.reject') }}</a-select-option>
            <a-select-option value="accept_and_reject">{{ t('settings.filterRules.acceptAndReject') }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item :label="t('settings.filterRules.conditions')" name="conditions">
          <a-textarea v-model:value="form.conditions" :rows="3" placeholder='如 [{"field":"title","op":"contains","value":"关键词"}]' />
        </a-form-item>
        <a-form-item :label="t('settings.filterRules.savePath')" name="savePath">
          <a-input v-model:value="form.savePath" placeholder="留空使用默认" />
        </a-form-item>
        <a-form-item :label="t('settings.filterRules.category')" name="category">
          <a-input v-model:value="form.category" placeholder="留空使用默认" />
        </a-form-item>
        <a-form-item :label="t('settings.filterRules.tags')" name="tags">
          <a-input v-model:value="form.tags" placeholder="逗号分隔" />
        </a-form-item>
        <a-form-item :label="t('settings.filterRules.priority')" name="priority">
          <a-input-number v-model:value="form.priority" :min="0" style="width: 100%" />
        </a-form-item>
        <a-form-item :label="t('common.enable')" name="enabled">
          <a-switch v-model:checked="form.enabled" />
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
import { filterRulesApi } from '@/api/filter-rules'
import { usePagination } from '@/composables/usePagination'

const { t } = useI18n()

const modalVisible = ref(false)
const submitting = ref(false)
const editingRule = ref<any>(null)

const form = reactive({
  name: '',
  ruleType: 'reject',
  conditions: '',
  savePath: '',
  category: '',
  tags: '',
  priority: 0,
  enabled: true,
})

const columns = [
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: '类型', key: 'ruleType', width: 120 },
  { title: '条件', dataIndex: 'conditions', key: 'conditions', ellipsis: true },
  { title: '保存路径', dataIndex: 'savePath', key: 'savePath', ellipsis: true },
  { title: '分类', dataIndex: 'category', key: 'category', width: 100 },
  { title: '优先级', dataIndex: 'priority', key: 'priority', width: 80 },
  { title: '操作', key: 'actions', width: 120 },
]

const pagination = usePagination((page, size) => filterRulesApi.list(page, size))

function openModal(record?: any) {
  editingRule.value = record || null
  if (record) {
    Object.assign(form, {
      name: record.name || '',
      ruleType: record.ruleType || 'reject',
      conditions: typeof record.conditions === 'string' ? record.conditions : JSON.stringify(record.conditions || []),
      savePath: record.savePath || '',
      category: record.category || '',
      tags: record.tags || '',
      priority: record.priority || 0,
      enabled: record.enabled ?? true,
    })
  } else {
    Object.assign(form, { name: '', ruleType: 'reject', conditions: '', savePath: '', category: '', tags: '', priority: 0, enabled: true })
  }
  modalVisible.value = true
}

async function handleSubmit() {
  submitting.value = true
  try {
    if (editingRule.value) {
      await filterRulesApi.update(editingRule.value.id, form)
    } else {
      await filterRulesApi.create(form)
    }
    message.success(t('common.operationSuccess'))
    modalVisible.value = false
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  } finally {
    submitting.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await filterRulesApi.delete(id)
    message.success(t('common.deleteSuccess'))
    pagination.fetch()
  } catch (e: any) {
    message.error(e.message)
  }
}

onMounted(() => pagination.fetch())
</script>
