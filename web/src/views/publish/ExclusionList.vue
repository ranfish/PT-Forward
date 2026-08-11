<template>
  <div>
    <a-page-header title="发布规则" subtitle="站点互斥、合规规则、声明过滤、小组映射" @back="$router.push('/publish')" />

    <a-tabs v-model:active-key="activeTab" style="padding: 0 24px">
      <!-- Tab 1: 站点互斥 -->
      <a-tab-pane key="exclusions" tab="站点互斥">
        <div style="margin-bottom: 16px">
          <a-button type="primary" @click="openExclusionModal()">
            <template #icon><PlusOutlined /></template>
            {{ t('publish.addExclusionRule') }}
          </a-button>
        </div>

        <a-table
          :columns="exclusionColumns"
          :data-source="exclusions"
          :loading="exclusionLoading"
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
            <template v-if="column.key === 'source_badge'">
              <a-tag :color="record.is_hardcoded ? 'default' : 'green'" style="font-size: 11px">
                {{ record.is_hardcoded ? '内置' : '用户' }}
              </a-tag>
            </template>
            <template v-if="column.key === 'actions'">
              <a-tooltip v-if="record.is_hardcoded" :title="t('publish.builtinCannotDelete')">
                <a-button type="link" danger size="small" disabled>{{ t('common.delete') }}</a-button>
              </a-tooltip>
              <a-popconfirm v-else :title="t('publish.deleteExclusionConfirm')" @confirm="handleExclusionDelete(record)">
                <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
              </a-popconfirm>
            </template>
          </template>
        </a-table>

        <a-modal
          v-model:open="exclusionModalVisible"
          :title="t('publish.addExclusionRule')"
          :confirm-loading="exclusionSubmitting"
          @ok="handleExclusionSubmit"
        >
          <a-form :model="exclusionForm" layout="vertical">
            <a-form-item :label="t('publish.targetSite')" name="target_site" :rules="[{ required: true, message: t('publish.pleaseEnterTargetSite') }]">
              <a-input v-model:value="exclusionForm.target_site" :placeholder="t('publish.targetSitePlaceholder')" />
            </a-form-item>
            <a-form-item :label="t('publish.sourceSite')" name="source_site" :rules="[{ required: true, message: t('publish.pleaseEnterSourceSite') }]">
              <a-input v-model:value="exclusionForm.source_site" :placeholder="t('publish.sourceSitePlaceholder')" />
            </a-form-item>
          </a-form>
          <a-typography-text type="secondary">
            {{ t('publish.exclusionHint') }}
          </a-typography-text>
        </a-modal>
      </a-tab-pane>

      <!-- Tab 2: 合规规则 -->
      <a-tab-pane key="compliance" tab="合规规则">
        <a-alert
          type="info"
          show-icon
          style="margin-bottom: 16px"
          message="合规规则在发布/辅种/手动转发时自动拦截"
          description="内置规则不可删除（仅可查看）。用户自定义规则可随时增删。规则匹配标题与副标题（禁转关键词/成人内容/禁转小组）。"
        />

        <div style="margin-bottom: 16px">
          <a-button type="primary" @click="openComplianceModal()">
            <template #icon><PlusOutlined /></template>
            添加自定义规则
          </a-button>
          <a-tag color="blue" style="margin-left: 12px">
            共 {{ complianceRules.length }} 条（内置 {{ builtinCount }} / 用户 {{ userCount }}）
          </a-tag>
        </div>

        <a-table
          :columns="complianceColumns"
          :data-source="complianceRules"
          :loading="complianceLoading"
          :pagination="false"
          row-key="id"
          size="small"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'rule_type'">
              <a-tag :color="ruleTypeColor(record.rule_type)">{{ ruleTypeLabel(record.rule_type) }}</a-tag>
            </template>
            <template v-if="column.key === 'pattern'">
              <code style="font-size: 13px">{{ record.pattern }}</code>
            </template>
            <template v-if="column.key === 'scope'">
              <span style="font-size: 12px; color: #666">{{ scopeLabel(record.scope) }}</span>
            </template>
            <template v-if="column.key === 'source'">
              <a-tag :color="record.source === 'builtin' ? 'default' : 'green'" style="font-size: 11px">
                {{ record.source === 'builtin' ? '内置' : '用户' }}
              </a-tag>
            </template>
            <template v-if="column.key === 'actions'">
              <a-tooltip v-if="record.source === 'builtin'" title="内置规则不可删除">
                <a-button type="link" danger size="small" disabled>{{ t('common.delete') }}</a-button>
              </a-tooltip>
              <a-popconfirm v-else title="确定删除此规则？" @confirm="handleComplianceDelete(record)">
                <a-button type="link" danger size="small">{{ t('common.delete') }}</a-button>
              </a-popconfirm>
            </template>
          </template>
        </a-table>

        <a-modal
          v-model:open="complianceModalVisible"
          title="添加自定义合规规则"
          :confirm-loading="complianceSubmitting"
          @ok="handleComplianceSubmit"
        >
          <a-form :model="complianceForm" layout="vertical">
            <a-form-item label="规则类型" name="rule_type" :rules="[{ required: true, message: '请选择规则类型' }]">
              <a-select v-model:value="complianceForm.rule_type" placeholder="选择规则类型">
                <a-select-option value="forbidden_keyword">禁转关键词</a-select-option>
                <a-select-option value="forbidden_group">禁转小组</a-select-option>
                <a-select-option value="adult">成人内容</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="匹配内容" name="pattern" :rules="[{ required: true, message: '请输入匹配内容' }]">
              <a-input v-model:value="complianceForm.pattern" placeholder="关键词或小组名（子串匹配）" />
            </a-form-item>
            <a-form-item label="适用场景" name="scope">
              <a-select v-model:value="complianceForm.scope">
                <a-select-option value="share">发布+辅种（默认）</a-select-option>
                <a-select-option value="all">全部场景</a-select-option>
                <a-select-option value="publish">仅发布</a-select-option>
                <a-select-option value="reseed">仅辅种</a-select-option>
              </a-select>
            </a-form-item>
          </a-form>
          <a-typography-text type="secondary">
            规则采用子串匹配（标题或副标题中包含即命中）。
          </a-typography-text>
        </a-modal>
      </a-tab-pane>

      <!-- §59.20 Tab 3: 声明过滤规则 -->
      <a-tab-pane key="decl-filters" tab="声明过滤">
        <div style="margin-bottom: 12px; display: flex; gap: 8px; align-items: center">
          <a-switch v-model:checked="declFiltersEnabled" />
          <span style="font-size: 13px; color: #666">启用声明过滤（发布时自动移除匹配的 [quote] 块）</span>
        </div>
        <div style="margin-bottom: 8px">
          <a-button type="primary" size="small" @click="addDeclPattern">添加模式</a-button>
        </div>
        <a-list :data-source="declPatterns" bordered size="small">
          <template #renderItem="{ index }">
            <a-list-item>
              <div style="display: flex; gap: 8px; align-items: center; width: 100%">
                <a-input v-model:value="declPatterns[index]" size="small" style="flex: 1" />
                <a-button size="small" danger @click="declPatterns.splice(index, 1)">删除</a-button>
              </div>
            </a-list-item>
          </template>
        </a-list>
        <div style="margin-top: 12px">
          <a-button type="primary" :loading="declSaving" @click="saveDeclPatterns">保存</a-button>
          <a-button style="margin-left: 8px" @click="fetchDeclPatterns">重置</a-button>
        </div>
      </a-tab-pane>

      <!-- §59.20 Tab 4: 小组源站映射（只读） -->
      <a-tab-pane key="group-mappings" tab="小组映射">
        <a-alert type="info" show-icon style="margin-bottom: 12px"
          message="制作组到源站的映射关系决定种子是否可转发。映射管理请联系开发维护人员。" />
        <a-table
          :data-source="groupMappings"
          :columns="groupMappingColumns"
          :loading="groupMappingsLoading"
          row-key="id"
          size="small"
          :pagination="{ pageSize: 50, showSizeChanger: false }"
        />
      </a-tab-pane>
    </a-tabs>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { message } from 'ant-design-vue'
import { PlusOutlined } from '@ant-design/icons-vue'
import { exclusionsApi, complianceApi } from '@/api/exclusions'
import { publishTorrentsApi } from '@/api/publish'
import type { ExclusionRule, ComplianceRule } from '@/api/exclusions'
import { formatTime } from '@/utils/format'

const { t } = useI18n()
const activeTab = ref('exclusions')

// ============ Tab 1: 站点互斥 ============
const exclusionLoading = ref(false)
const exclusions = ref<ExclusionRule[]>([])
const exclusionModalVisible = ref(false)
const exclusionSubmitting = ref(false)

const exclusionForm = reactive({
  target_site: '',
  source_site: '',
})

const exclusionColumns = computed(() => [
  { title: t('publish.targetSite'), key: 'target_site', width: 180 },
  { title: t('publish.sourceSite'), key: 'source_site', width: 180 },
  { title: '类型', key: 'source_badge', width: 80 },
  { title: t('common.createdAt'), dataIndex: 'created_at', key: 'created_at', width: 160, customRender: ({ text }: { text: string }) => text ? formatTime(text) : '-' },
  { title: t('common.actions'), key: 'actions', width: 100 },
])

async function fetchExclusions() {
  exclusionLoading.value = true
  try {
    const resp = await exclusionsApi.list()
    exclusions.value = resp.data.data || []
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    exclusionLoading.value = false
  }
}

function openExclusionModal() {
  exclusionForm.target_site = ''
  exclusionForm.source_site = ''
  exclusionModalVisible.value = true
}

async function handleExclusionSubmit() {
  if (!exclusionForm.target_site || !exclusionForm.source_site) {
    message.warning(t('publish.fillTargetAndSource'))
    return
  }
  exclusionSubmitting.value = true
  try {
    await exclusionsApi.create({ target_site: exclusionForm.target_site, source_site: exclusionForm.source_site })
    message.success(t('common.addSuccess'))
    exclusionModalVisible.value = false
    fetchExclusions()
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    exclusionSubmitting.value = false
  }
}

async function handleExclusionDelete(record: ExclusionRule) {
  try {
    await exclusionsApi.remove({ target_site: record.target_site, source_site: record.source_site })
    message.success(t('common.deleted'))
    fetchExclusions()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

// ============ Tab 2: 合规规则 ============
const complianceLoading = ref(false)
const complianceRules = ref<ComplianceRule[]>([])
const complianceModalVisible = ref(false)
const complianceSubmitting = ref(false)

const complianceForm = reactive({
  rule_type: 'forbidden_keyword' as 'adult' | 'forbidden_keyword' | 'forbidden_group',
  pattern: '',
  scope: 'share',
})

const builtinCount = computed(() => complianceRules.value.filter(r => r.source === 'builtin').length)
const userCount = computed(() => complianceRules.value.filter(r => r.source === 'user').length)

const complianceColumns = [
  { title: '规则类型', key: 'rule_type', width: 130 },
  { title: '匹配内容', key: 'pattern' },
  { title: '适用场景', key: 'scope', width: 120 },
  { title: '来源', key: 'source', width: 80 },
  { title: '操作', key: 'actions', width: 100 },
]

function ruleTypeLabel(type: string): string {
  const m: Record<string, string> = {
    adult: '成人内容',
    forbidden_keyword: '禁转关键词',
    forbidden_group: '禁转小组',
    site_blacklist_category: '站点分类黑名单',
  }
  return m[type] || type
}

function ruleTypeColor(type: string): string {
  const m: Record<string, string> = {
    adult: 'red',
    forbidden_keyword: 'orange',
    forbidden_group: 'purple',
    site_blacklist_category: 'default',
  }
  return m[type] || 'default'
}

function scopeLabel(scope: string): string {
  const m: Record<string, string> = {
    all: '全部场景',
    publish: '仅发布',
    reseed: '仅辅种',
    share: '发布+辅种',
    download: '仅下载',
  }
  return m[scope] || scope
}

async function fetchCompliance() {
  complianceLoading.value = true
  try {
    const resp = await complianceApi.list()
    complianceRules.value = resp.data?.data?.items || []
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    complianceLoading.value = false
  }
}

function openComplianceModal() {
  complianceForm.rule_type = 'forbidden_keyword'
  complianceForm.pattern = ''
  complianceForm.scope = 'share'
  complianceModalVisible.value = true
}

async function handleComplianceSubmit() {
  if (!complianceForm.rule_type || !complianceForm.pattern.trim()) {
    message.warning('请填写规则类型和匹配内容')
    return
  }
  complianceSubmitting.value = true
  try {
    await complianceApi.create({
      rule_type: complianceForm.rule_type,
      pattern: complianceForm.pattern.trim(),
      scope: complianceForm.scope,
    })
    message.success('规则已添加')
    complianceModalVisible.value = false
    fetchCompliance()
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    complianceSubmitting.value = false
  }
}

async function handleComplianceDelete(record: ComplianceRule) {
  try {
    await complianceApi.remove(record.id)
    message.success(t('common.deleted'))
    fetchCompliance()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

onMounted(() => {
  fetchExclusions()
  fetchCompliance()
  fetchDeclPatterns()
  fetchGroupMappings()
})

// ============ §59.20 Tab 3: 声明过滤 ============
const declPatterns = ref<string[]>([])
const declFiltersEnabled = ref(true)
const declSaving = ref(false)

async function fetchDeclPatterns() {
  try {
    const resp = await publishTorrentsApi.getDeclarationFilters()
    declPatterns.value = resp.data?.data?.patterns || []
  } catch { /* silent */ }
}

function addDeclPattern() {
  declPatterns.value.push('')
}

async function saveDeclPatterns() {
  declSaving.value = true
  try {
    const filtered = declPatterns.value.filter(p => p.trim() !== '')
    await publishTorrentsApi.setDeclarationFilters(filtered)
    declPatterns.value = filtered
    message.success('声明过滤规则已保存')
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    declSaving.value = false
  }
}

// ============ §59.20 Tab 4: 小组映射（只读） ============
const groupMappings = ref<Array<{ id: number; group_name: string; domain: string; site_name: string; is_official: boolean; is_builtin: boolean }>>([])
const groupMappingsLoading = ref(false)
const groupMappingColumns = [
  { title: '制作组', dataIndex: 'group_name', key: 'group_name', width: 120 },
  { title: '站点', dataIndex: 'site_name', key: 'site_name', width: 100 },
  { title: '域名', dataIndex: 'domain', key: 'domain', ellipsis: true },
  { title: '官组', dataIndex: 'is_official', key: 'is_official', width: 60 },
  { title: '内置', dataIndex: 'is_builtin', key: 'is_builtin', width: 60 },
]

async function fetchGroupMappings() {
  groupMappingsLoading.value = true
  try {
    const resp = await publishTorrentsApi.listGroupMappings()
    groupMappings.value = (resp.data?.data?.items || []) as unknown as typeof groupMappings.value
  } catch { /* silent */ } finally {
    groupMappingsLoading.value = false
  }
}
</script>
