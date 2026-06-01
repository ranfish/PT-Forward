import { message } from 'ant-design-vue'

export async function switchAutoSave(fn: () => Promise<unknown>, successMsg = '配置已保存') {
  try {
    await fn()
    message.success(successMsg)
  } catch (e: unknown) {
    message.error(e instanceof Error ? e.message : String(e))
  }
}
