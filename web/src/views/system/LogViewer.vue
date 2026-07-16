<template>
  <div>
    <a-card title="实时日志">
      <template #extra>
        <a-space>
          <a-select v-model:value="levelFilter" style="width: 120px" @change="reconnect">
            <a-select-option value="">全部级别</a-select-option>
            <a-select-option value="debug">Debug</a-select-option>
            <a-select-option value="info">Info</a-select-option>
            <a-select-option value="warn">Warn</a-select-option>
            <a-select-option value="error">Error</a-select-option>
          </a-select>
          <a-switch v-model:checked="autoScroll" checked-children="自动滚动" un-checked-children="手动" />
          <a-switch v-model:checked="connected" checked-children="已连接" un-checked-children="已断开" @change="toggleConnection" />
          <a-button size="small" @click="clearLogs">清空</a-button>
        </a-space>
      </template>

      <div ref="logContainer" class="log-container">
        <div v-for="(line, i) in displayLogs" :key="i" class="log-line" :class="lineClass(line)">
          {{ line }}
        </div>
        <div v-if="logs.length === 0" class="log-empty">
          等待日志...
        </div>
      </div>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'

const logs = ref<string[]>([])
const maxLogs = 500
const autoScroll = ref(true)
const connected = ref(false)
const levelFilter = ref('')
const logContainer = ref<HTMLElement>()
let eventSource: EventSource | null = null

const displayLogs = computed(() => {
  if (logs.value.length > maxLogs) {
    return logs.value.slice(-maxLogs)
  }
  return logs.value
})

function lineClass(line: string): string {
  if (line.includes('"level":"error"')) return 'log-error'
  if (line.includes('"level":"warn"')) return 'log-warn'
  if (line.includes('"level":"debug"')) return 'log-debug'
  return ''
}

function scrollToEnd() {
  if (autoScroll.value && logContainer.value) {
    logContainer.value.scrollTop = logContainer.value.scrollHeight
  }
}

function toggleConnection(val: boolean | string | number) {
  if (val) {
    connect()
  } else {
    disconnect()
  }
}

function connect() {
  disconnect()
  const token = localStorage.getItem('pt-forward-access-token') || ''
  const params = levelFilter.value ? `?level=${levelFilter.value}&token=${token}` : `?token=${token}`
  const proto = window.location.protocol === 'https:' ? 'https:' : 'http:'
  const wsUrl = `${proto}//${window.location.host}/api/v1/system/logs/stream${params}`
  eventSource = new EventSource(wsUrl)
  eventSource.onopen = () => {
    connected.value = true
  }
  eventSource.onmessage = (ev) => {
    if (ev.data.startsWith(':')) return
    logs.value.push(ev.data)
    if (logs.value.length > maxLogs * 2) {
      logs.value = logs.value.slice(-maxLogs)
    }
    nextTick(scrollToEnd)
  }
  eventSource.onerror = () => {
    connected.value = false
  }
}

function disconnect() {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
  connected.value = false
}

function reconnect() {
  if (connected.value) {
    connect()
  }
}

function clearLogs() {
  logs.value = []
}

watch(autoScroll, () => {
  if (autoScroll.value) nextTick(scrollToEnd)
})

onMounted(() => {
  connected.value = true
  connect()
})
onUnmounted(disconnect)
</script>

<style scoped>
.log-container {
  height: 600px;
  overflow-y: auto;
  background: #1e1e1e;
  border-radius: 4px;
  padding: 8px 12px;
  font-family: 'Fira Code', 'Consolas', monospace;
  font-size: 12px;
  line-height: 1.6;
}
.log-line {
  color: #d4d4d4;
  white-space: pre-wrap;
  word-break: break-all;
}
.log-error {
  color: #f48771;
}
.log-warn {
  color: #cca700;
}
.log-debug {
  color: #569cd6;
}
.log-empty {
  color: #666;
  text-align: center;
  padding-top: 40px;
}
</style>
