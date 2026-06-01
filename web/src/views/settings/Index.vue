<template>
  <div>
    <a-card :title="t('settings.title')" style="margin-bottom: 24px">
      <a-form :model="form" layout="vertical" style="max-width: 600px">
        <a-form-item :label="t('settings.httpProxy')">
          <a-input v-model:value="form.httpProxy" :placeholder="t('settings.httpProxyPlaceholder')" />
        </a-form-item>
        <a-form-item :label="t('settings.socksProxy')">
          <a-input v-model:value="form.socksProxy" :placeholder="t('settings.socksProxyPlaceholder')" />
        </a-form-item>

        <a-divider>{{ t('settings.cookiecloudConfig') }}</a-divider>
        <a-form-item :label="t('settings.cookiecloudUrl')">
          <a-input v-model:value="form.cookieCloudUrl" :placeholder="t('settings.cookiecloudUrlPlaceholder')" />
        </a-form-item>
        <a-form-item :label="t('settings.cookiecloudUuid')">
          <a-input v-model:value="form.cookieCloudUuid" :placeholder="t('settings.cookiecloudUuidPlaceholder')" />
        </a-form-item>
        <a-form-item :label="t('settings.cookiecloudPassword')">
          <a-input-password v-model:value="form.cookieCloudPassword" :placeholder="t('settings.cookiecloudPasswordPlaceholder')" />
        </a-form-item>

        <a-divider>{{ t('settings.otherSettings') }}</a-divider>
        <a-form-item :label="t('settings.websocketEnabled')">
          <a-switch v-model:checked="form.websocketEnabled" @change="(v: boolean) => saveBoolSetting('websocketEnabled', v)" />
        </a-form-item>

        <a-divider>{{ t('settings.loginSecurity') }}</a-divider>
        <a-form-item :label="t('settings.loginLockoutEnabled')">
          <a-switch v-model:checked="form.loginLockoutEnabled" @change="(v: boolean) => saveBoolSetting('login_lockout_enabled', v)" />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('settings.loginMaxRetries')">
              <a-input-number v-model:value="form.loginMaxRetries" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('settings.loginLockoutMin')">
              <a-input-number v-model:value="form.loginLockoutMin" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>

        <a-divider>{{ t('settings.rateLimit') }}</a-divider>
        <a-form-item :label="t('settings.rateLimitEnabled')">
          <a-switch v-model:checked="form.rateLimitEnabled" @change="(v: boolean) => saveBoolSetting('rate_limit_enabled', v)" />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item :label="t('settings.rateLimitGlobal')">
              <a-input-number v-model:value="form.rateLimitGlobal" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item :label="t('settings.rateLimitWrite')">
              <a-input-number v-model:value="form.rateLimitWrite" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item :label="t('settings.rateLimitDownload')">
              <a-input-number v-model:value="form.rateLimitDownload" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>

        <a-divider>{{ t('settings.screenshotConfig') }}</a-divider>
        <a-form-item :label="t('settings.screenshotEnabled')">
          <a-switch v-model:checked="form.screenshotEnabled" @change="(v: boolean) => saveBoolSetting('screenshot_enabled', v)" />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('settings.screenshotMpvPath')">
              <a-input v-model:value="form.screenshotMpvPath" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('settings.screenshotCount')">
              <a-input-number v-model:value="form.screenshotCount" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item :label="t('settings.screenshotMinInterval')">
              <a-input-number v-model:value="form.screenshotMinInterval" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item :label="t('settings.screenshotJpegQuality')">
              <a-input-number v-model:value="form.screenshotJpegQuality" :min="1" :max="100" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>

        <a-divider>{{ t('settings.dataCleanup') }}</a-divider>
        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item :label="t('settings.cleanupTorrentEventDays')">
              <a-input-number v-model:value="form.cleanupTorrentEventDays" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item :label="t('settings.cleanupPublishResultDays')">
              <a-input-number v-model:value="form.cleanupPublishResultDays" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item :label="t('settings.cleanupSeenRecordDays')">
              <a-input-number v-model:value="form.cleanupSeenRecordDays" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item :label="t('settings.cleanupNotificationDays')">
              <a-input-number v-model:value="form.cleanupNotificationDays" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item :label="t('settings.cleanupPublishTaskDays')">
              <a-input-number v-model:value="form.cleanupPublishTaskDays" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item :label="t('settings.cleanupReseedMatchDays')">
              <a-input-number v-model:value="form.cleanupReseedMatchDays" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item :label="t('settings.cleanupPtgenCacheDays')">
              <a-input-number v-model:value="form.cleanupPtgenCacheDays" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item :label="t('settings.cleanupSeedingArchiveDays')">
              <a-input-number v-model:value="form.cleanupSeedingArchiveDays" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item :label="t('settings.cleanupAuditLogDays')">
              <a-input-number v-model:value="form.cleanupAuditLogDays" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item :label="t('settings.cleanupTorrentTrafficDays')">
              <a-input-number v-model:value="form.cleanupTorrentTrafficDays" :min="7" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>

        <a-form-item>
          <a-space>
            <a-button type="primary" :loading="saving" @click="saveSettings">{{ t('common.saveConfig') }}</a-button>
            <a-button @click="fetchSettings">{{ t('common.reset') }}</a-button>
          </a-space>
        </a-form-item>
      </a-form>
    </a-card>

    <a-card :title="t('settings.dataManagement')">
      <a-space>
        <a-button :loading="backingUp" @click="backupSettings">{{ t('settings.exportBackup') }}</a-button>
        <a-upload :before-upload="restoreSettings" :show-upload-list="false" accept=".json">
          <a-button :loading="restoring">{{ t('settings.importBackup') }}</a-button>
        </a-upload>
      </a-space>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'
import { settingsApi } from '@/api/settings'
import { switchAutoSave } from '@/utils/switch-auto-save'

const { t } = useI18n()

function saveBoolSetting(key: string, value: boolean) {
  return switchAutoSave(() => settingsApi.update(key, { value: String(value) }), t('common.configSaved'))
}

const saving = ref(false)
const backingUp = ref(false)
const restoring = ref(false)

const form = reactive({
  httpProxy: '',
  socksProxy: '',
  cookieCloudUrl: '',
  cookieCloudUuid: '',
  cookieCloudPassword: '',
  websocketEnabled: true,
  loginLockoutEnabled: false,
  loginMaxRetries: 5,
  loginLockoutMin: 5,
  rateLimitEnabled: false,
  rateLimitGlobal: 600,
  rateLimitWrite: 200,
  rateLimitDownload: 50,
  screenshotEnabled: false,
  screenshotMpvPath: 'mpv',
  screenshotCount: 6,
  screenshotMinInterval: 60,
  screenshotJpegQuality: 85,
  cleanupTorrentEventDays: 30,
  cleanupPublishResultDays: 30,
  cleanupSeenRecordDays: 30,
  cleanupNotificationDays: 30,
  cleanupPublishTaskDays: 30,
  cleanupReseedMatchDays: 30,
  cleanupPtgenCacheDays: 90,
  cleanupSeedingArchiveDays: 90,
  cleanupAuditLogDays: 90,
  cleanupTorrentTrafficDays: 30,
})

async function fetchSettings() {
  try {
    const resp = await settingsApi.get()
    const raw = resp.data.data
    const data: Record<string, string> = {}
    if (Array.isArray(raw)) {
      for (const item of raw) {
        data[item.key] = item.value
      }
    } else if (raw && typeof raw === 'object') {
      Object.assign(data, raw)
    }
    Object.assign(form, {
      httpProxy: data.httpProxy || '',
      socksProxy: data.socksProxy || '',
      cookieCloudUrl: data.cookieCloudUrl || '',
      cookieCloudUuid: data.cookieCloudUuid || '',
      cookieCloudPassword: data.cookieCloudPassword || '',
      websocketEnabled: data.websocketEnabled !== 'false',
      loginLockoutEnabled: data.login_lockout_enabled === 'true',
      loginMaxRetries: parseInt(data.login_max_retries) || 5,
      loginLockoutMin: parseInt(data.login_lockout_min) || 5,
      rateLimitEnabled: data.rate_limit_enabled === 'true',
      rateLimitGlobal: parseInt(data.rate_limit_global) || 600,
      rateLimitWrite: parseInt(data.rate_limit_write) || 200,
      rateLimitDownload: parseInt(data.rate_limit_download) || 50,
      screenshotEnabled: data.screenshot_enabled === 'true',
      screenshotMpvPath: data.screenshot_mpv_path || 'mpv',
      screenshotCount: parseInt(data.screenshot_count) || 6,
      screenshotMinInterval: parseInt(data.screenshot_min_interval) || 60,
      screenshotJpegQuality: parseInt(data.screenshot_jpeg_quality) || 85,
      cleanupTorrentEventDays: parseInt(data.data_cleanup_torrent_event_days) || 30,
      cleanupPublishResultDays: parseInt(data.data_cleanup_publish_result_days) || 30,
      cleanupSeenRecordDays: parseInt(data.data_cleanup_seen_record_days) || 30,
      cleanupNotificationDays: parseInt(data.data_cleanup_notification_days) || 30,
      cleanupPublishTaskDays: parseInt(data.data_cleanup_publish_task_days) || 30,
      cleanupReseedMatchDays: parseInt(data.data_cleanup_reseed_match_days) || 30,
      cleanupPtgenCacheDays: parseInt(data.data_cleanup_ptgen_cache_days) || 90,
      cleanupSeedingArchiveDays: parseInt(data.data_cleanup_seeding_archive_days) || 90,
      cleanupAuditLogDays: parseInt(data.data_cleanup_audit_log_days) || 90,
      cleanupTorrentTrafficDays: parseInt(data.torrent_traffic_retention_days) || 30,
    })
  } catch (e: unknown) {
    message.error((e as Error).message)
  }
}

async function saveSettings() {
  saving.value = true
  try {
    const entries: [string, string][] = [
      ['httpProxy', form.httpProxy],
      ['socksProxy', form.socksProxy],
      ['cookieCloudUrl', form.cookieCloudUrl],
      ['cookieCloudUuid', form.cookieCloudUuid],
      ['cookieCloudPassword', form.cookieCloudPassword],
      ['websocketEnabled', String(form.websocketEnabled)],
      ['login_lockout_enabled', String(form.loginLockoutEnabled)],
      ['login_max_retries', String(form.loginMaxRetries)],
      ['login_lockout_min', String(form.loginLockoutMin)],
      ['rate_limit_enabled', String(form.rateLimitEnabled)],
      ['rate_limit_global', String(form.rateLimitGlobal)],
      ['rate_limit_write', String(form.rateLimitWrite)],
      ['rate_limit_download', String(form.rateLimitDownload)],
      ['screenshot_enabled', String(form.screenshotEnabled)],
      ['screenshot_mpv_path', form.screenshotMpvPath],
      ['screenshot_count', String(form.screenshotCount)],
      ['screenshot_min_interval', String(form.screenshotMinInterval)],
      ['screenshot_jpeg_quality', String(form.screenshotJpegQuality)],
      ['data_cleanup_torrent_event_days', String(form.cleanupTorrentEventDays)],
      ['data_cleanup_publish_result_days', String(form.cleanupPublishResultDays)],
      ['data_cleanup_seen_record_days', String(form.cleanupSeenRecordDays)],
      ['data_cleanup_notification_days', String(form.cleanupNotificationDays)],
      ['data_cleanup_publish_task_days', String(form.cleanupPublishTaskDays)],
      ['data_cleanup_reseed_match_days', String(form.cleanupReseedMatchDays)],
      ['data_cleanup_ptgen_cache_days', String(form.cleanupPtgenCacheDays)],
      ['data_cleanup_seeding_archive_days', String(form.cleanupSeedingArchiveDays)],
      ['data_cleanup_audit_log_days', String(form.cleanupAuditLogDays)],
      ['torrent_traffic_retention_days', String(form.cleanupTorrentTrafficDays)],
    ]
    for (const [key, value] of entries) {
      await settingsApi.update(key, { value })
    }
    message.success(t('settings.settingsSaved'))
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    saving.value = false
  }
}

async function backupSettings() {
  backingUp.value = true
  try {
    const resp = await settingsApi.backup()
    const blob = new Blob([JSON.stringify(resp.data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `pt-forward-backup-${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(url)
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    backingUp.value = false
  }
}

async function restoreSettings(file: File) {
  restoring.value = true
  try {
    const text = await file.text()
    const data = JSON.parse(text)
    await settingsApi.restore(data)
    message.success(t('settings.backupRestored'))
    fetchSettings()
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    restoring.value = false
  }
  return false
}

onMounted(fetchSettings)
</script>
