<template>
  <div>
    <a-page-header :title="site.name || site.domain" @back="$router.push('/sites')">
      <template #tags>
        <a-tag v-if="needsCookie && site.hasCookie" color="green">{{ t('site.cookieValid') }}</a-tag>
        <a-tag v-else-if="needsCookie" color="red">{{ t('site.cookieNotConfigured') }}</a-tag>
        <a-tag v-if="needsApiKey && site.hasApiKey" color="green">{{ t('site.apiKeyValid') }}</a-tag>
        <a-tag v-else-if="needsApiKey" color="red">{{ t('site.apiKeyNotConfigured') }}</a-tag>
        <a-tag v-if="needsPasskey && site.hasPasskey" color="green">{{ t('site.passkeyValid') }}</a-tag>
        <a-tag v-else-if="needsPasskey" color="red">{{ t('site.passkeyNotConfigured') }}</a-tag>
        <a-tag v-if="site.hasAuthKey" color="green">{{ t('site.authKeyValid') }}</a-tag>
        <a-tag v-if="site.hasRssKey" color="green">{{ t('site.rssKeyValid') }}</a-tag>
      </template>
      <template #extra>
        <a-button :loading="detecting" @click="runDetect">{{ t('site.detect') }}</a-button>
      </template>
    </a-page-header>

    <a-modal v-model:open="showDetectResult" :title="t('site.detectResult')" :footer="null" width="520px">
      <a-descriptions bordered :column="1" size="small">
        <a-descriptions-item :label="t('site.detectFramework')">
          <a-tag color="blue">{{ detectResult.framework }}</a-tag>
        </a-descriptions-item>
        <a-descriptions-item :label="t('site.detectConfidence')">{{ ((detectResult.confidence ?? 0) * 100).toFixed(0) }}%</a-descriptions-item>
        <a-descriptions-item v-if="detectResult.detectionDetail" :label="t('site.detectDetail')">{{ detectResult.detectionDetail }}</a-descriptions-item>
      </a-descriptions>
    </a-modal>

    <a-spin :spinning="loading">
      <div style="margin-bottom: 16px; display: flex; gap: 8px; align-items: center;">
        <a-button v-if="siteFrozen" type="primary" danger size="small" :loading="freezeLoading" @click="handleUnfreeze">{{ t('site.unfreeze') }}</a-button>
        <a-button v-else size="small" :loading="freezeLoading" @click="showFreezeModal = true">{{ t('site.freeze') }}</a-button>
        <a-tag v-if="siteFrozen" color="red">{{ t('site.frozen') }}</a-tag>
      </div>

      <a-modal v-model:open="showFreezeModal" :title="t('site.freezeTitle')" @ok="handleFreeze">
        <a-form layout="vertical">
          <a-form-item :label="t('site.freezeDuration')">
            <a-input v-model:value="freezeForm.duration" placeholder="1h / 30m / 2d" />
          </a-form-item>
          <a-form-item :label="t('site.freezeReason')">
            <a-textarea v-model:value="freezeForm.reason" :rows="2" />
          </a-form-item>
        </a-form>
      </a-modal>

      <a-descriptions bordered :column="2" style="margin-bottom: 24px">
        <a-descriptions-item :label="t('common.name')">{{ site.name }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.framework')">
          <a-tag :color="frameworkColors[site.framework as string] || 'default'">
            {{ frameworkLabels[site.framework as string] || site.framework }}
          </a-tag>
        </a-descriptions-item>
        <a-descriptions-item :label="t('site.authType')">{{ authLabel }}</a-descriptions-item>
        <a-descriptions-item v-if="needsCookie" :label="t('sites.cookie')">{{ site.hasCookie ? t('common.configured') : t('common.notConfigured') }}</a-descriptions-item>
        <a-descriptions-item v-if="needsApiKey" :label="t('sites.apiKey')">{{ site.hasApiKey ? t('common.configured') : t('common.notConfigured') }}</a-descriptions-item>
        <a-descriptions-item v-if="needsPasskey" :label="t('sites.passkey')">{{ site.hasPasskey ? t('common.configured') : t('common.notConfigured') }}</a-descriptions-item>
        <a-descriptions-item v-if="site.hasAuthKey" :label="t('sites.authKey')">{{ t('common.configured') }}</a-descriptions-item>
        <a-descriptions-item v-if="site.hasRssKey" :label="t('sites.rssKey')">{{ t('common.configured') }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.enabledLabel')"><a-badge :status="site.enabled ? 'success' : 'default'" :text="site.enabled ? t('common.yes') : t('common.no')" /></a-descriptions-item>
        <a-descriptions-item :label="t('site.role')">{{ [site.isSource ? t('site.sourceSiteRole') : '', site.isTarget ? t('site.targetSiteRole') : ''].filter(Boolean).join(', ') || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.participateAutoPublishLabel')">{{ site.participateAutoPublish ? t('common.yes') : t('common.no') }}</a-descriptions-item>
        <a-descriptions-item v-if="needsCookie" :label="t('site.cookieCloudSyncLabel')">{{ site.cookieCloudSync ? t('common.yes') : t('common.no') }}</a-descriptions-item>
        <a-descriptions-item v-if="needsCookie && site.cookieCloudSync" :label="t('site.cookieCloudDomainLabel')">{{ site.cookieCloudDomain || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.hashStrategy')">{{ site.hashStrategy || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.sizeStrategy')">{{ site.sizeStrategy || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.idStrategy')">{{ site.idStrategy || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.overrideRssUrl')">{{ site.overrideRssUrl || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.overrideSavePath')">{{ site.overrideSavePath || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.proxyAddress')">{{ site.proxyUrl || '-' }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.skipSslVerify')">{{ site.skipSslVerify ? t('common.yes') : t('common.no') }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.maxConcurrent')">{{ site.maxConcurrent || 2 }}</a-descriptions-item>
        <a-descriptions-item :label="t('site.lastSync')">{{ formatTime(site.lastSyncAt) }}</a-descriptions-item>
        <a-descriptions-item :label="t('common.createdAt')">{{ formatTime(site.createdAt) }}</a-descriptions-item>
      </a-descriptions>

      <a-card :title="t('site.siteStats')" style="margin-bottom: 24px">
        <template #extra>
          <a-button size="small" :loading="statsSyncing" @click="syncStats">{{ t('site.syncStats') }}</a-button>
        </template>
        <a-spin :spinning="statsLoading">
          <a-row :gutter="16">
            <a-col :span="4"><a-statistic :title="t('site.uploadBytes')" :value="formatBytes(stats.uploadBytes)" /></a-col>
            <a-col :span="4"><a-statistic :title="t('site.downloadBytes')" :value="formatBytes(stats.downloadBytes)" /></a-col>
            <a-col :span="4"><a-statistic :title="t('site.ratio')" :value="Number(stats.uploadBytes ?? 0) > 0 && Number(stats.downloadBytes ?? 0) === 0 ? '∞' : (Number.isFinite(stats.ratio) ? stats.ratio!.toFixed(2) : '-')" /></a-col>
            <a-col :span="4"><a-statistic :title="t('site.seedingCount')" :value="stats.seedingCount ?? '-'" /></a-col>
            <a-col :span="4"><a-statistic :title="t('site.seedingPoints')" :value="stats.seedingPoints ?? '-'" /></a-col>
            <a-col :span="4"><a-statistic :title="t('site.bonusPoints')" :value="stats.bonusPoints ?? '-'" /></a-col>
          </a-row>
          <a-row :gutter="16" style="margin-top: 12px">
            <a-col :span="4"><a-statistic :title="t('site.seedingSize')" :value="formatBytes(stats.seedingSize)" /></a-col>
            <a-col :span="4"><a-statistic :title="t('site.userClass')" :value="stats.userClass || '-'" /></a-col>
            <a-col :span="8"><a-statistic :title="t('site.statsSyncedAt')" :value="formatTime(stats.statsSyncedAt)" /></a-col>
          </a-row>
        </a-spin>
      </a-card>

      <a-card :title="t('site.siteSettings')" style="margin-bottom: 24px">
        <a-form :model="settingsForm" layout="vertical" style="max-width: 600px">
          <a-divider>{{ t('site.basicInfo') }}</a-divider>
          <a-form-item :label="t('common.name')">
            <a-input v-model:value="settingsForm.name" />
          </a-form-item>
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('site.baseUrl')">
                <a-input v-model:value="settingsForm.baseUrl" :placeholder="t('site.baseUrlPlaceholder')" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('site.alternativeDomains')">
                <a-input v-model:value="settingsForm.alternativeDomains" :placeholder="t('site.alternativeDomainsPlaceholder')" />
              </a-form-item>
            </a-col>
          </a-row>

          <a-divider>{{ t('site.roleAndPublish') }}</a-divider>
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('site.enabledLabel')">
                <a-switch v-model:checked="settingsForm.enabled" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('site.participateAutoPublishLabel')">
                <a-switch v-model:checked="settingsForm.participateAutoPublish" />
              </a-form-item>
            </a-col>
          </a-row>
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('site.asSource')">
                <a-switch v-model:checked="settingsForm.isSource" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('site.asTarget')">
                <a-switch v-model:checked="settingsForm.isTarget" />
              </a-form-item>
            </a-col>
          </a-row>
          <a-row v-if="needsCookie" :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('site.cookieCloudSyncLabel')">
                <a-switch v-model:checked="settingsForm.cookieCloudSync" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('site.cookieCloudDomainLabel')">
                <a-input v-model:value="settingsForm.cookieCloudDomain" :placeholder="t('site.cookieCloudDomainShortPlaceholder')" />
              </a-form-item>
            </a-col>
          </a-row>

          <a-divider>{{ t('site.rssSavePathOverride') }}</a-divider>
          <a-form-item :label="t('site.overrideRssUrl')">
            <a-input v-model:value="settingsForm.overrideRssUrl" :placeholder="t('site.customRssUrl')" />
          </a-form-item>
          <a-form-item :label="t('site.overrideSavePath')">
            <a-input v-model:value="settingsForm.overrideSavePath" :placeholder="t('site.customSavePath')" />
          </a-form-item>

          <a-divider>{{ t('site.network') }}</a-divider>
          <a-form-item :label="t('site.proxyAddress')">
            <a-input v-model:value="settingsForm.proxyUrl" :placeholder="t('site.proxyPlaceholder')" />
          </a-form-item>
          <a-form-item :label="t('site.skipSslVerify')">
            <a-switch v-model:checked="settingsForm.skipSslVerify" />
          </a-form-item>
          <a-form-item :label="t('site.maxConcurrent')">
            <a-input-number v-model:value="settingsForm.maxConcurrent" :min="1" :max="100" style="width: 100%" />
            <div style="font-size: 11px; color: #999; margin-top: 2px">{{ t('site.maxConcurrentHint') }}</div>
          </a-form-item>

          <a-divider>{{ t('site.parseStrategy') }}</a-divider>
          <a-row :gutter="16">
            <a-col :span="8">
              <a-form-item :label="t('site.hashStrategy')">
                <a-select v-model:value="settingsForm.hashStrategy" allow-clear>
                  <a-select-option value="guid">GUID</a-select-option>
                  <a-select-option value="xml_tag">{{ t('site.xmlTag') }}</a-select-option>
                  <a-select-option value="fake_from_id">{{ t('site.fakeFromId') }}</a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
            <a-col :span="8">
              <a-form-item :label="t('site.sizeStrategy')">
                <a-select v-model:value="settingsForm.sizeStrategy" allow-clear>
                  <a-select-option value="enclosure">{{ t('site.sizeStrategyEnclosure') }}</a-select-option>
                  <a-select-option value="xml_tag">{{ t('site.xmlTag') }}</a-select-option>
                  <a-select-option value="desc_regex">{{ t('site.descRegex') }}</a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
            <a-col :span="8">
              <a-form-item :label="t('site.idStrategy')">
                <a-select v-model:value="settingsForm.idStrategy" allow-clear>
                  <a-select-option value="query_param">{{ t('site.queryParam') }}</a-select-option>
                  <a-select-option value="link_regex">{{ t('site.linkRegex') }}</a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
          </a-row>
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('site.idPattern')">
                <a-input v-model:value="settingsForm.idPattern" :placeholder="t('site.idPatternPlaceholder')" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('site.sizeBaseUnit')">
                <a-input-number v-model:value="settingsForm.sizeBaseUnit" :placeholder="t('site.sizeBaseUnitPlaceholder')" style="width: 100%" />
              </a-form-item>
            </a-col>
          </a-row>
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('site.hashXmlTagName')">
                <a-input v-model:value="settingsForm.hashXmlTagName" :placeholder="t('site.hashXmlTagNamePlaceholder')" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('site.sizeXmlTagName')">
                <a-input v-model:value="settingsForm.sizeXmlTagName" :placeholder="t('site.sizeXmlTagNamePlaceholder')" />
              </a-form-item>
            </a-col>
          </a-row>
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('site.hashUrlParamName')">
                <a-input v-model:value="settingsForm.hashUrlParamName" :placeholder="t('site.hashUrlParamNamePlaceholder')" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('site.sizeDescRegex')">
                <a-input v-model:value="settingsForm.sizeDescRegex" :placeholder="t('site.sizeDescRegexPlaceholder')" />
              </a-form-item>
            </a-col>
          </a-row>
          <a-form-item :label="t('site.sizeTitleRegex')">
            <a-input v-model:value="settingsForm.sizeTitleRegex" :placeholder="t('site.sizeTitleRegexPlaceholder')" />
          </a-form-item>

          <a-divider>{{ t('site.downloadSettings') }}</a-divider>
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('site.downloadMode')">
                <a-select v-model:value="settingsForm.downloadMode" allow-clear :placeholder="t('site.downloadModeDefault')">
                  <a-select-option value="direct">{{ t('site.downloadModeDirect') }}</a-select-option>
                  <a-select-option value="page">{{ t('site.downloadModePage') }}</a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('site.requiresSideLoading')">
                <a-switch v-model:checked="settingsForm.requiresSideLoading" />
              </a-form-item>
            </a-col>
          </a-row>
          <a-form-item :label="t('site.downloadUrlTemplate')">
            <a-input v-model:value="settingsForm.downloadUrlTemplate" :placeholder="t('site.downloadUrlTemplatePlaceholder')" />
          </a-form-item>
          <a-form-item :label="t('site.downloadPagePattern')">
            <a-input v-model:value="settingsForm.downloadPagePattern" :placeholder="t('site.downloadPagePatternPlaceholder')" />
          </a-form-item>

          <a-divider>{{ t('site.hrStrategy') }}</a-divider>
          <a-form-item :label="t('site.hrStrategy')">
            <a-select v-model:value="settingsForm.hrStrategy" allow-clear :placeholder="t('site.hrStrategyPlaceholder')">
              <a-select-option value="protect">{{ t('site.hrProtect') }}</a-select-option>
              <a-select-option value="ignore">{{ t('site.hrIgnore') }}</a-select-option>
              <a-select-option value="skip">{{ t('site.hrStrict') }}</a-select-option>
            </a-select>
          </a-form-item>

          <a-form-item>
            <a-button type="primary" @click="updateSettings">{{ t('site.saveSettings') }}</a-button>
          </a-form-item>
        </a-form>
      </a-card>

      <a-card :title="t('site.credentialManagement')" style="margin-bottom: 24px">
        <a-form :model="credForm" layout="vertical" style="max-width: 500px">
          <a-form-item v-if="needsCookie" :label="t('sites.cookie')">
            <a-textarea v-model:value="credForm.cookie" :rows="4" :placeholder="t('site.inputCookie')" />
          </a-form-item>
          <a-form-item v-if="needsApiKey" :label="t('sites.apiKey')">
            <a-input-password v-model:value="credForm.apiKey" :placeholder="t('site.inputApiKey')" />
          </a-form-item>
          <a-form-item v-if="showPasskeyField" :label="site.passkeyAlias || t('sites.passkey')">
            <template v-if="site.passkeyHint" #extra>
              <span style="color: rgba(0,0,0,0.45); font-size: 12px;">{{ site.passkeyHint }}</span>
            </template>
            <a-input-group compact>
              <a-input
                v-model:value="credForm.passkey"
                :placeholder="site.passkeyMasked || (site.passkeyAlias ? `${t('site.inputPasskey')}${site.passkeyAlias}` : t('site.inputPasskey'))"
                style="width: calc(100% - 40px)"
              />
              <a-button :disabled="!site.passkeyMasked" @click="fillPasskeyMasked">
                <template #icon><SyncOutlined /></template>
              </a-button>
            </a-input-group>
          </a-form-item>

          <a-divider>{{ t('site.advancedCredentials') }}</a-divider>
          <a-form-item :label="t('site.bearerToken')">
            <a-input-password v-model:value="credForm.bearerToken" :placeholder="t('site.bearerTokenPlaceholder')" />
          </a-form-item>
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('site.authKey')">
                <a-input-password v-model:value="credForm.authKey" :placeholder="site.authKeyMasked || t('site.authKeyPlaceholder')" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('site.authHash')">
                <a-input-password v-model:value="credForm.authHash" :placeholder="t('site.authHashPlaceholder')" />
              </a-form-item>
            </a-col>
          </a-row>
          <a-row :gutter="16">
            <a-col :span="12">
              <a-form-item :label="t('site.userId')">
                <a-input-number v-model:value="credForm.userId" :placeholder="t('site.userIdPlaceholder')" style="width: 100%" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item :label="t('site.rssKey')">
                <a-input-password v-model:value="credForm.rssKey" :placeholder="site.rssKeyMasked || t('site.rssKeyPlaceholder')" />
              </a-form-item>
            </a-col>
          </a-row>
          <a-form-item>
            <a-button type="primary" @click="updateCredentials">{{ t('site.saveCredentials') }}</a-button>
          </a-form-item>
        </a-form>
      </a-card>

      <a-card :title="t('site.siteConfigOverride')" style="margin-bottom: 24px">
        <template #extra>
          <a-space>
            <a-button size="small" @click="showOverrideEditor = true">{{ t('site.editOverrideConfig') }}</a-button>
            <a-popconfirm v-if="hasOverrides" :title="t('site.deleteAllOverrideConfirm')" @confirm="deleteOverrides">
              <a-button size="small" danger>{{ t('site.deleteOverride') }}</a-button>
            </a-popconfirm>
          </a-space>
        </template>
        <div v-if="overrideLoading">
          <a-spin />
        </div>
        <div v-else-if="hasOverrides">
          <a-descriptions bordered size="small" :column="2">
            <a-descriptions-item v-for="(val, key) in overrideData" :key="key" :label="String(key)">
              {{ typeof val === 'object' ? JSON.stringify(val) : String(val) }}
            </a-descriptions-item>
          </a-descriptions>
        </div>
        <a-empty v-else :description="t('site.noOverrideConfig')" />
      </a-card>

      <a-modal
        v-model:open="showOverrideEditor"
        :title="t('site.editOverrideConfig')"
        :confirm-loading="overrideSaving"
        width="640px"
        @ok="saveOverrides"
      >
        <a-alert :message="t('site.overrideConfigHint')" type="info" show-icon style="margin-bottom: 16px" />
        <a-textarea v-model:value="overrideJSON" :rows="12" placeholder="{&quot;upload_rule&quot;: &quot;...&quot;, &quot;download_prefix&quot;: &quot;...&quot;}" />
      </a-modal>

      <a-card :title="t('site.searchTorrents')" style="margin-bottom: 24px">
        <a-form layout="inline" style="margin-bottom: 16px">
          <a-form-item>
            <a-input v-model:value="searchQuery" :placeholder="t('site.searchPlaceholder')" style="width: 300px" @press-enter="doSearch" />
          </a-form-item>
          <a-form-item>
            <a-checkbox v-model:checked="searchFreeOnly">{{ t('site.freeOnly') }}</a-checkbox>
          </a-form-item>
          <a-form-item>
            <a-button type="primary" :loading="searchLoading" @click="doSearch">{{ t('site.search') }}</a-button>
          </a-form-item>
        </a-form>
        <a-table
          v-if="searchResults.length > 0"
          :data-source="searchResults"
          :loading="searchLoading"
          :pagination="{ pageSize: 10 }"
          row-key="torrent_id"
          size="small"
        >
          <a-table-column :title="t('site.torrentTitle')" data-index="title" ellipsis />
          <a-table-column :title="t('site.torrentSize')" data-index="size" :width="100">
            <template #default="{ text }">{{ formatFileSize(text) }}</template>
          </a-table-column>
          <a-table-column :title="t('site.seeders')" data-index="seeders" :width="80" />
          <a-table-column :title="t('site.leechers')" data-index="leechers" :width="80" />
          <a-table-column :title="t('site.discount')" data-index="discount" :width="100">
            <template #default="{ text }">
              <a-tag v-if="text && text !== 'NONE'" :color="discountColor(text)">{{ text }}</a-tag>
              <span v-else>-</span>
            </template>
          </a-table-column>
          <a-table-column :title="t('common.actions')" :width="120">
            <template #default="{ record }">
              <a-button type="link" size="small" :loading="discountLoadingMap[record.torrent_id]" @click="checkDiscount(record.torrent_id)">{{ t('site.checkDiscount') }}</a-button>
            </template>
          </a-table-column>
        </a-table>
        <a-empty v-else-if="!searchLoading" :description="t('site.noSearchResults')" />
        <a-modal v-model:open="showDiscountResult" :title="t('site.discountResult')" :footer="null" width="400px">
          <a-descriptions bordered :column="1" size="small">
            <a-descriptions-item :label="t('site.discountLevel')">
              <a-tag :color="discountColor(discountInfo.level)">{{ discountInfo.level || '-' }}</a-tag>
            </a-descriptions-item>
            <a-descriptions-item :label="t('site.multiplier')">{{ discountInfo.multiplier ?? '-' }}</a-descriptions-item>
            <a-descriptions-item :label="t('site.freeEndAt')">{{ formatTime(discountInfo.free_end_at) }}</a-descriptions-item>
          </a-descriptions>
        </a-modal>
      </a-card>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { message } from 'ant-design-vue'
import { SyncOutlined } from '@ant-design/icons-vue'
import { useI18n } from 'vue-i18n'
import { sitesApi } from '@/api/sites'
import type { Site, SiteCredentials, SearchTorrentResult, SiteConfigOverride } from '@/api/types'

interface DetectResultData {
  framework?: string
  confidence?: number
  detectionDetail?: string
}

interface SiteStatsData {
  uploadBytes?: number
  downloadBytes?: number
  ratio?: number
  seedingCount?: number
  seedingPoints?: number
  bonusPoints?: number
  seedingSize?: number
  userClass?: string
  statsSyncedAt?: string
}

const { t } = useI18n()

const route = useRoute()
const siteId = Number(route.params.id)

const loading = ref(false)
const site = ref<Partial<Site>>({})
const credForm = reactive({ cookie: '', passkey: '', apiKey: '', bearerToken: '', authKey: '', authHash: '', userId: undefined as number | undefined, rssKey: '' })

const settingsForm = reactive({
  name: '',
  baseUrl: '',
  alternativeDomains: '',
  enabled: true,
  isSource: false,
  isTarget: false,
  participateAutoPublish: true,
  cookieCloudSync: false,
  cookieCloudDomain: '',
  overrideRssUrl: '',
  overrideSavePath: '',
  proxyUrl: '',
  skipSslVerify: false,
  maxConcurrent: 2,
  hashStrategy: '',
  sizeStrategy: '',
  idStrategy: '',
  idPattern: '',
  hashXmlTagName: '',
  sizeXmlTagName: '',
  hashUrlParamName: '',
  sizeDescRegex: '',
  sizeTitleRegex: '',
  sizeBaseUnit: 0,
  downloadMode: '',
  downloadUrlTemplate: '',
  downloadPagePattern: '',
  requiresSideLoading: false,
  hrStrategy: '',
})

const authTypeLabels: Record<string, string> = {
  cookie: 'Cookie',
  apikey: 'API Key',
  passkey: 'Passkey',
}

const needsCookie = computed(() => site.value.authType === 'cookie' || !site.value.authType)
const needsApiKey = computed(() => site.value.authType === 'apikey')
const needsPasskey = computed(() => site.value.authType === 'passkey')
const showPasskeyField = computed(() => {
  const fw = site.value.framework as string
  return needsPasskey.value || (fw === 'nexusphp' || fw === 'generic')
})
const authLabel = computed(() => authTypeLabels[site.value.authType as string] || 'Cookie')

const overrideLoading = ref(false)
const overrideSaving = ref(false)
const overrideData = ref<Partial<SiteConfigOverride>>({})
const overrideJSON = ref('{}')
const showOverrideEditor = ref(false)

const hasOverrides = computed(() => Object.keys(overrideData.value).length > 0)

const detecting = ref(false)
const showDetectResult = ref(false)
const detectResult = ref<DetectResultData>({})
const statsLoading = ref(false)
const statsSyncing = ref(false)
const stats = ref<SiteStatsData>({})

import { formatBytes, formatTime } from '@/utils/format'
const frameworkColors: Record<string, string> = {
  nexusphp: 'blue', unit3d: 'green', gazelle: 'purple',
  mteam: 'orange', rousi: 'pink', tnode: 'cyan', luminance: 'magenta', generic: 'default',
}

const frameworkLabels: Record<string, string> = {
  nexusphp: 'NexusPHP', unit3d: 'UNIT3D', gazelle: 'Gazelle',
  mteam: 'M-Team', rousi: 'Rousi', tnode: 'TNode', luminance: 'Luminance', generic: 'Generic',
}

const siteFrozen = ref(false)
const freezeLoading = ref(false)
const showFreezeModal = ref(false)
const freezeForm = reactive({ duration: '1h', reason: '' })

async function checkFreezeStatus() {
  try {
    const resp = await sitesApi.getFreezeStatus(siteId)
    siteFrozen.value = resp.data.data?.frozen ?? false
  } catch {
    siteFrozen.value = false
  }
}

async function handleFreeze() {
  freezeLoading.value = true
  try {
    await sitesApi.freezeSite(siteId, freezeForm)
    message.success(t('site.freezeSuccess'))
    siteFrozen.value = true
    showFreezeModal.value = false
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    freezeLoading.value = false
  }
}

async function handleUnfreeze() {
  freezeLoading.value = true
  try {
    await sitesApi.unfreezeSite(siteId)
    message.success(t('site.unfreezeSuccess'))
    siteFrozen.value = false
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    freezeLoading.value = false
  }
}

async function fetchSite() {
  loading.value = true
  try {
    const resp = await sitesApi.get(siteId)
    site.value = resp.data.data || {}
    Object.assign(settingsForm, {
      name: site.value.name || '',
      baseUrl: site.value.baseUrl || '',
      alternativeDomains: altDomainsToText(site.value.alternativeDomains),
      enabled: site.value.enabled !== undefined ? site.value.enabled : true,
      isSource: site.value.isSource || false,
      isTarget: site.value.isTarget || false,
      participateAutoPublish: site.value.participateAutoPublish !== undefined ? site.value.participateAutoPublish : true,
      cookieCloudSync: site.value.cookieCloudSync || false,
      cookieCloudDomain: site.value.cookieCloudDomain || '',
      overrideRssUrl: site.value.overrideRssUrl || '',
      overrideSavePath: site.value.overrideSavePath || '',
      proxyUrl: site.value.proxyUrl || '',
      skipSslVerify: site.value.skipSslVerify || false,
      maxConcurrent: site.value.maxConcurrent || 2,
      hashStrategy: site.value.hashStrategy || '',
      sizeStrategy: site.value.sizeStrategy || '',
      idStrategy: site.value.idStrategy || '',
      idPattern: site.value.idPattern || '',
      hashXmlTagName: site.value.hashXmlTagName || '',
      sizeXmlTagName: site.value.sizeXmlTagName || '',
      hashUrlParamName: site.value.hashUrlParamName || '',
      sizeDescRegex: site.value.sizeDescRegex || '',
      sizeTitleRegex: site.value.sizeTitleRegex || '',
      sizeBaseUnit: site.value.sizeBaseUnit || 0,
      downloadMode: site.value.downloadMode || '',
      downloadUrlTemplate: site.value.downloadUrlTemplate || '',
      downloadPagePattern: site.value.downloadPagePattern || '',
      requiresSideLoading: site.value.requiresSideLoading || false,
      hrStrategy: site.value.hrStrategy || '',
    })
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    loading.value = false
  }
}

function fillPasskeyMasked() {
  if (site.value.passkeyMasked) {
    credForm.passkey = site.value.passkeyMasked
  }
}

async function updateCredentials() {
  try {
    const payload: SiteCredentials = {}
    if (credForm.passkey) payload.passkey = credForm.passkey
    if (credForm.apiKey) payload.apiKey = credForm.apiKey
    if (credForm.bearerToken) payload.bearerToken = credForm.bearerToken
    if (credForm.authKey) payload.authKey = credForm.authKey
    if (credForm.authHash) payload.authHash = credForm.authHash
    if (credForm.userId) payload.userId = credForm.userId
    if (credForm.rssKey) payload.rssKey = credForm.rssKey
    if (credForm.cookie) payload.cookie = credForm.cookie
    await sitesApi.updateCredentials(siteId, payload)
    message.success(t('site.credentialsUpdated'))
    credForm.cookie = ''
    credForm.passkey = ''
    credForm.apiKey = ''
    credForm.bearerToken = ''
    credForm.authKey = ''
    credForm.authHash = ''
    credForm.userId = undefined
    credForm.rssKey = ''
    fetchSite()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function updateSettings() {
  try {
    await sitesApi.update(siteId, {
      enabled: settingsForm.enabled,
      isSource: settingsForm.isSource,
      isTarget: settingsForm.isTarget,
      participateAutoPublish: settingsForm.participateAutoPublish,
      cookieCloudSync: settingsForm.cookieCloudSync,
      cookieCloudDomain: settingsForm.cookieCloudDomain,
      overrideRssUrl: settingsForm.overrideRssUrl,
      overrideSavePath: settingsForm.overrideSavePath,
      proxyUrl: settingsForm.proxyUrl,
      skipSslVerify: settingsForm.skipSslVerify,
      maxConcurrent: settingsForm.maxConcurrent,
      hashStrategy: settingsForm.hashStrategy,
      sizeStrategy: settingsForm.sizeStrategy,
      idStrategy: settingsForm.idStrategy,
      hrStrategy: settingsForm.hrStrategy,
      name: settingsForm.name,
      baseUrl: settingsForm.baseUrl,
      alternativeDomains: altDomainsToJson(settingsForm.alternativeDomains),
      idPattern: settingsForm.idPattern,
      hashXmlTagName: settingsForm.hashXmlTagName,
      sizeXmlTagName: settingsForm.sizeXmlTagName,
      hashUrlParamName: settingsForm.hashUrlParamName,
      sizeDescRegex: settingsForm.sizeDescRegex,
      sizeTitleRegex: settingsForm.sizeTitleRegex,
      sizeBaseUnit: settingsForm.sizeBaseUnit,
      downloadMode: settingsForm.downloadMode,
      downloadUrlTemplate: settingsForm.downloadUrlTemplate,
      downloadPagePattern: settingsForm.downloadPagePattern,
      requiresSideLoading: settingsForm.requiresSideLoading,
    })
    message.success(t('common.configSaved'))
    fetchSite()
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function fetchOverrides() {
  overrideLoading.value = true
  try {
    const resp = await sitesApi.getOverrides(siteId)
    const items = resp.data?.data?.items
    if (Array.isArray(items)) {
      const filtered: Record<string, unknown> = {}
      for (const item of items) {
        const entry = item as unknown as Record<string, unknown>
        if (entry.fieldPath && entry.fieldPath !== 'id' && entry.fieldPath !== 'site_id' && entry.fieldPath !== 'created_at' && entry.fieldPath !== 'updated_at') {
          filtered[entry.fieldPath as string] = entry.fieldValue
        }
      }
      overrideData.value = filtered
      overrideJSON.value = Object.keys(filtered).length > 0 ? JSON.stringify(filtered, null, 2) : '{}'
    } else {
      overrideData.value = {}
      overrideJSON.value = '{}'
    }
  } catch {
    overrideData.value = {}
    overrideJSON.value = '{}'
  } finally {
    overrideLoading.value = false
  }
}

async function saveOverrides() {
  overrideSaving.value = true
  try {
    const parsed = JSON.parse(overrideJSON.value)
    await sitesApi.updateOverrides(siteId, parsed)
    message.success(t('site.overrideConfigSaved'))
    showOverrideEditor.value = false
    fetchOverrides()
  } catch (e: unknown) {
    if (e instanceof SyntaxError) {
      message.error(t('common.jsonFormatError'))
    } else {
      message.error((e as { response?: { data?: { message?: string } } }).response?.data?.message || t('common.saveFailed'))
    }
  } finally {
    overrideSaving.value = false
  }
}

async function deleteOverrides() {
  try {
    await sitesApi.deleteOverrides(siteId)
    message.success(t('site.overrideConfigDeleted'))
    overrideData.value = {}
    overrideJSON.value = '{}'
  } catch (e: unknown) {
    message.error((e as { response?: { data?: { message?: string } } }).response?.data?.message || t('common.deleteFailed'))
  }
}

async function runDetect() {
  detecting.value = true
  try {
    const resp = await sitesApi.detect(siteId)
    detectResult.value = resp.data.data || {}
    showDetectResult.value = true
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    detecting.value = false
  }
}

async function fetchStats() {
  statsLoading.value = true
  try {
    const resp = await sitesApi.getStats(siteId)
    stats.value = resp.data.data || {}
  } catch {
    stats.value = {}
  } finally {
    statsLoading.value = false
  }
}

async function syncStats() {
  statsSyncing.value = true
  try {
    await sitesApi.syncSiteStats(siteId)
    message.success(t('site.statsSynced'))
    fetchStats()
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    statsSyncing.value = false
  }
}

const searchQuery = ref('')
const searchFreeOnly = ref(false)
const searchLoading = ref(false)
const searchResults = ref<SearchTorrentResult[]>([])
const discountLoadingMap = reactive<Record<string, boolean>>({})
const showDiscountResult = ref(false)
const discountInfo = reactive({ level: '', free_end_at: null as string | null, multiplier: 0 })

function altDomainsToText(val: string | undefined): string {
  if (!val) return ''
  try {
    const arr: string[] = JSON.parse(val)
    if (Array.isArray(arr)) return arr.join(', ')
  } catch { /* not JSON, return as-is */ }
  return val
}

function altDomainsToJson(val: string): string {
  const trimmed = val.trim()
  if (!trimmed) return ''
  const items = trimmed.split(/[,，\s]+/).map(s => s.trim()).filter(Boolean)
  if (items.length === 0) return ''
  return JSON.stringify(items)
}

function formatFileSize(bytes: unknown): string {
  const n = Number(bytes) || 0
  return formatBytes(n)
}

function discountColor(level: string): string {
  const colors: Record<string, string> = {
    FREE: 'green', '2XFREE': 'lime', '2XUP': 'blue',
    PERCENT_50: 'orange', PERCENT_70: 'gold', PERCENT_25: 'volcano',
  }
  return colors[level] || 'default'
}

async function doSearch() {
  if (!searchQuery.value.trim()) {
    message.warning(t('site.searchKeywordRequired'))
    return
  }
  searchLoading.value = true
  try {
    const resp = await sitesApi.searchTorrents(siteId, {
      query: searchQuery.value.trim(),
      freeOnly: searchFreeOnly.value,
    })
    searchResults.value = resp.data.data || []
  } catch (e: unknown) {
    message.error((e as Error).message)
    searchResults.value = []
  } finally {
    searchLoading.value = false
  }
}

async function checkDiscount(torrentId: string) {
  discountLoadingMap[torrentId] = true
  try {
    const resp = await sitesApi.detectDiscount(siteId, { torrentId })
    const data = resp.data.data || {}
    Object.assign(discountInfo, {
      level: data.level || '-',
      free_end_at: data.free_end_at || null,
      multiplier: data.multiplier ?? 0,
    })
    showDiscountResult.value = true
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    discountLoadingMap[torrentId] = false
  }
}

onMounted(() => {
  fetchSite()
  fetchOverrides()
  fetchStats()
  checkFreezeStatus()
})
</script>
