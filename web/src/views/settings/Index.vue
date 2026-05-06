<template>
  <div>
    <a-card :title="t('settings.title')" style="margin-bottom: 24px">
      <a-form :model="form" layout="vertical" style="max-width: 600px">
        <a-form-item :label="t('settings.httpProxy')">
          <a-input v-model:value="form.httpProxy" placeholder="例如: http://127.0.0.1:7890" />
        </a-form-item>
        <a-form-item :label="t('settings.socksProxy')">
          <a-input v-model:value="form.socksProxy" placeholder="例如: socks5://127.0.0.1:7891" />
        </a-form-item>

        <a-divider>{{ t('settings.cookiecloudConfig') }}</a-divider>
        <a-form-item label="CookieCloud 地址">
          <a-input v-model:value="form.cookieCloudUrl" placeholder="CookieCloud 服务器地址" />
        </a-form-item>
        <a-form-item label="CookieCloud UUID">
          <a-input v-model:value="form.cookieCloudUuid" placeholder="用户 UUID" />
        </a-form-item>
        <a-form-item label="CookieCloud 密码">
          <a-input-password v-model:value="form.cookieCloudPassword" placeholder="加密密码" />
        </a-form-item>

        <a-divider>{{ t('settings.otherSettings') }}</a-divider>
        <a-form-item :label="t('settings.dataRetentionDays')">
          <a-input-number v-model:value="form.dataRetentionDays" :min="1" style="width: 100%" />
        </a-form-item>
        <a-form-item :label="t('settings.websocketEnabled')">
          <a-switch v-model:checked="form.websocketEnabled" />
        </a-form-item>

        <a-form-item>
          <a-space>
            <a-button type="primary" @click="saveSettings" :loading="saving">{{ t('common.saveConfig') }}</a-button>
            <a-button @click="fetchSettings">{{ t('common.reset') }}</a-button>
          </a-space>
        </a-form-item>
      </a-form>
    </a-card>

    <a-card :title="t('settings.dataManagement')">
      <a-space>
        <a-button @click="backupSettings" :loading="backingUp">{{ t('settings.exportBackup') }}</a-button>
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

const { t } = useI18n()

const saving = ref(false)
const backingUp = ref(false)
const restoring = ref(false)

const form = reactive({
  httpProxy: '',
  socksProxy: '',
  cookieCloudUrl: '',
  cookieCloudUuid: '',
  cookieCloudPassword: '',
  dataRetentionDays: 30,
  websocketEnabled: true,
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
      dataRetentionDays: parseInt(data.dataRetentionDays) || 30,
      websocketEnabled: data.websocketEnabled !== 'false',
    })
  } catch (e: any) {
    message.error(e.message)
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
      ['dataRetentionDays', String(form.dataRetentionDays)],
      ['websocketEnabled', String(form.websocketEnabled)],
    ]
    for (const [key, value] of entries) {
      await settingsApi.update(key, { value })
    }
    message.success(t('settings.settingsSaved'))
  } catch (e: any) {
    message.error(e.message)
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
  } catch (e: any) {
    message.error(e.message)
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
  } catch (e: any) {
    message.error(e.message)
  } finally {
    restoring.value = false
  }
  return false
}

onMounted(fetchSettings)
</script>
