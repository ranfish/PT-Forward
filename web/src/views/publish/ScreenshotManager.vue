<template>
  <div class="screenshot-manager">
    <!-- 操作工具栏 -->
    <div class="toolbar">
      <a-input-group compact style="flex: 1">
        <a-input
          v-model:value="newUrl"
          placeholder="粘贴截图 URL 后回车添加"
          style="width: calc(100% - 60px)"
          @press-enter="addUrl"
        />
        <a-button type="primary" :disabled="!newUrl.trim()" @click="addUrl">
          添加
        </a-button>
      </a-input-group>
      <!-- §59.54: 批量粘贴——多行输入，支持裸 URL / [img] / <img> 三形态解析 -->
      <a-button @click="bulkPasteVisible = true">批量粘贴</a-button>
      <!-- §59.54: 恢复引用——转存前快照存在时显示，一键还原 -->
      <a-button v-if="rehostSnapshot" type="link" @click="restoreFromSnapshot">恢复引用</a-button>

      <a-tooltip title="开启后截图以 [img] 标签嵌入简介正文，关闭则作为独立附件">
        <a-switch v-model:checked="screenshotInDesc" size="small" />
        <span class="toggle-label">截图嵌入简介</span>
      </a-tooltip>
    </div>

    <!-- §59.54: 批量粘贴弹窗 -->
    <a-modal
      v-model:open="bulkPasteVisible"
      title="批量粘贴截图"
      ok-text="解析并添加"
      cancel-text="取消"
      @ok="applyBulkPaste"
    >
      <a-textarea
        v-model:value="bulkPasteText"
        :rows="8"
        placeholder="粘贴含截图的内容，支持裸 URL / [img] 标签 / img 标签三种形态，自动提取去重" 
      />
      <div v-if="bulkParsedPreview.length" style="margin-top: 8px; color: #666; font-size: 12px">
        已解析 {{ bulkParsedPreview.length }} 个 URL（点击确定添加）
      </div>
    </a-modal>
    <!-- 左右分栏 -->
    <div v-if="screenshots.length" class="split-layout">
      <!-- 左侧：URL 列表 -->
      <div class="url-list">
        <div
          v-for="(url, i) in screenshots"
          :key="url + i"
          class="url-item"
          :class="{
            dragging: dragIndex === i,
            'drag-over': dragOverIndex === i,
            selected: selectedIndex === i,
          }"
          draggable="true"
          @dragstart="onDragStart(i)"
          @dragover.prevent="onDragOver(i)"
          @dragend="onDragEnd"
          @drop.prevent="onDrop(i)"
          @click="selectedIndex = i"
        >
          <span class="url-index">{{ i + 1 }}</span>
          <span v-if="i > 0" class="drag-handle" title="拖拽排序">≡</span>
          <input
            class="url-text"
            :value="url"
            @input="updateUrl(i, ($event.target as HTMLInputElement).value)"
            @click.stop
          />
          <a-button type="text" danger size="small" @click.stop="remove(i)">
            <DeleteOutlined />
          </a-button>
        </div>
      </div>

      <!-- 右侧：按顺序平铺全部截图（§59.52: 纵向列表一次看全，点任一放大） -->
      <div class="preview-panel">
        <div class="preview-stack">
          <div v-for="(u, i) in screenshots" :key="i" class="preview-item">
            <span class="preview-idx">{{ i + 1 }}</span>
            <a-image :src="u" style="width: 100%; border-radius: 4px; cursor: zoom-in" />
          </div>
        </div>
      </div>
    </div>

    <!-- 空状态 -->
    <div v-else class="empty-hint">
      <InboxOutlined style="font-size: 32px; color: #d9d9d9" />
      <p>暂无截图</p>
    </div>

    <!-- 截图统计 -->
    <div v-if="screenshots.length" class="stats-hint">
      共 {{ screenshots.length }} 张截图 · 拖拽调整顺序 · 点击任一图片查看大图
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { InboxOutlined, DeleteOutlined } from '@ant-design/icons-vue'

const props = defineProps<{
  screenshots: string[]
  screenshotInDesc: boolean
}>()

const emit = defineEmits<{
  'update:screenshots': [value: string[]]
  'update:screenshotInDesc': [value: boolean]
}>()

const screenshots = computed({
  get: () => props.screenshots,
  set: (v: string[]) => emit('update:screenshots', v),
})
const screenshotInDesc = computed({
  get: () => props.screenshotInDesc,
  set: (v: boolean) => emit('update:screenshotInDesc', v),
})

const newUrl = ref('')
// §59.54: 批量粘贴
const bulkPasteVisible = ref(false)
const bulkPasteText = ref('')
// §59.54: 恢复引用——转存前快照（null = 无可恢复）
const rehostSnapshot = ref<string[] | null>(null)

// §59.54: 三形态解析（裸 URL / [img]url[/img] / <img src="url">）——与后端 ExtractImages 同口径
function parseBulkPaste(text: string): string[] {
  const urls: string[] = []
  const seen = new Set<string>()
  const push = (u: string) => {
    if (u && !seen.has(u)) {
      seen.add(u)
      urls.push(u)
    }
  }
  // [img]url[/img]
  for (const m of text.matchAll(/\[img\]\s*([^\s[]+?)\s*\[\/img\]/gi)) push(m[1])
  // <img src="url"> / <img src='url'>
  for (const m of text.matchAll(/<img[^>]+src=["']([^"']+)["']/gi)) push(m[1])
  // 裸 URL（图片扩展名放宽：jpg/jpeg/png/gif/webp + 任意带图片路径特征的 URL）
  for (const m of text.matchAll(/https?:\/\/[^\s["'<>]+/gi)) {
    const u = m[0].replace(/[),.;，。]+$/, '')
    push(u)
  }
  return urls
}

const bulkParsedPreview = computed(() => (bulkPasteText.value ? parseBulkPaste(bulkPasteText.value) : []))

function applyBulkPaste() {
  const urls = bulkParsedPreview.value.filter((u) => !screenshots.value.includes(u))
  if (urls.length) {
    screenshots.value = [...screenshots.value, ...urls]
  }
  bulkPasteText.value = ''
  bulkPasteVisible.value = false
}

function snapshotBeforeRehost() {
  rehostSnapshot.value = [...screenshots.value]
}

function restoreFromSnapshot() {
  if (!rehostSnapshot.value) return
  screenshots.value = [...rehostSnapshot.value]
  rehostSnapshot.value = null
}
const dragIndex = ref(-1)
const dragOverIndex = ref(-1)
const selectedIndex = ref(0)

function addUrl() {
  const url = newUrl.value.trim()
  if (!url) return
  if (!/^https?:\/\//i.test(url)) return
  const arr = [...screenshots.value, url]
  screenshots.value = arr
  selectedIndex.value = arr.length - 1
  newUrl.value = ''
}

function remove(idx: number) {
  const arr = [...screenshots.value]
  arr.splice(idx, 1)
  screenshots.value = arr
  if (selectedIndex.value >= arr.length) {
    selectedIndex.value = Math.max(0, arr.length - 1)
  }
}

function updateUrl(idx: number, value: string) {
  const arr = [...screenshots.value]
  arr[idx] = value
  screenshots.value = arr
}

function onDragStart(idx: number) {
  dragIndex.value = idx
}
function onDragOver(idx: number) {
  dragOverIndex.value = idx
}
function onDrop(targetIdx: number) {
  const srcIdx = dragIndex.value
  if (srcIdx < 0 || srcIdx === targetIdx) {
    onDragEnd()
    return
  }
  const arr = [...screenshots.value]
  const [moved] = arr.splice(srcIdx, 1)
  arr.splice(targetIdx, 0, moved)
  screenshots.value = arr
  // 保持选中项跟随移动
  if (selectedIndex.value === srcIdx) {
    selectedIndex.value = targetIdx
  }
  onDragEnd()
}
function onDragEnd() {
  dragIndex.value = -1
  dragOverIndex.value = -1
}
</script>

<style scoped>
.screenshot-manager {
  width: 100%;
}
.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}
.toggle-label {
  margin-left: 6px;
  font-size: 12px;
  color: #666;
}

/* 左右分栏 */
.split-layout {
  display: flex;
  gap: 16px;
  min-height: 400px;
}

/* 左侧 URL 列表 */
.url-list {
  width: 420px;
  min-width: 320px;
  max-height: 500px;
  overflow-y: auto;
  border: 1px solid #f0f0f0;
  border-radius: 4px;
}
.url-item {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 8px;
  border-bottom: 1px solid #f5f5f5;
  cursor: pointer;
  transition: background 0.15s;
}
.url-item:hover {
  background: #f0f5ff;
}
.url-item.selected {
  background: #e6f4ff;
  border-left: 3px solid #1677ff;
  padding-left: 5px;
}
.url-item.dragging {
  opacity: 0.4;
}
.url-item.drag-over {
  border-top: 2px solid #52c41a;
}
.url-index {
  font-size: 12px;
  color: #999;
  font-weight: 600;
  min-width: 24px;
  text-align: center;
}
.drag-handle {
  cursor: grab;
  color: #ccc;
  font-size: 14px;
  user-select: none;
}
.drag-handle:hover {
  color: #1677ff;
}
.url-text {
  flex: 1;
  border: 1px solid transparent;
  border-radius: 3px;
  padding: 2px 6px;
  font-size: 12px;
  color: #333;
  background: transparent;
  outline: none;
  min-width: 0;
}
.url-text:focus {
  border-color: #1677ff;
  background: #fff;
}

/* 右侧预览：纵向平铺（§59.52） */
.preview-panel {
  flex: 1;
  background: #fafafa;
  border-radius: 4px;
  padding: 12px;
  min-height: 400px;
  max-height: 500px;
  overflow-y: auto;
}
.preview-stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.preview-item {
  position: relative;
}
.preview-idx {
  position: absolute;
  top: 6px;
  left: 6px;
  z-index: 1;
  background: rgba(0, 0, 0, 0.5);
  color: #fff;
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 3px;
}
.preview-empty {
  display: none;
  height: 100%;
  color: #999;
}

/* 空状态 */
.empty-hint {
  text-align: center;
  padding: 40px 0;
  color: #999;
}
.stats-hint {
  margin-top: 8px;
  font-size: 11px;
  color: #999;
  text-align: right;
}
</style>

// §59.54: 暴露给父组件（CrossSeedPanel 转存前调快照）
defineExpose({ snapshotBeforeRehost })
