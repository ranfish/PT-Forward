import { message } from 'ant-design-vue'
import { useI18n } from 'vue-i18n'

export function useAutoSave() {
  const { t } = useI18n()

  async function autoSave(fn: () => Promise<unknown>) {
    try {
      await fn()
      message.success(t('common.configSaved'))
    } catch (e: unknown) {
      message.error(e instanceof Error ? e.message : String(e))
    }
  }

  return { autoSave }
}
