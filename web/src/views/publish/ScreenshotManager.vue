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
      <!-- §56.27 决策 5: ScreenshotInDesc toggle -->
      <a-tooltip title="开启后截图以 [img] 标签嵌入简介正文，关闭则作为独立附件">
        <a-switch
          v-model:checked="screenshotInDesc"
          size="small"
        />
        <span class="toggle-label">截图嵌入简介</span>
      </a-tooltip>
    </div>

    <!-- 截图列表（拖拽排序） -->
    <div v-if="!screenshots.length" class="empty-hint">
      <InboxOutlined style="font-size: 32px; color: #d9d9d9" />
      <p>暂无截图</p>
    </div>
    <div v-else class="screenshot-grid">
      <div
        v-for="(url, i) in screenshots"
        :key="url + i"
        class="screenshot-item"
        :class="{ dragging: dragIndex === i, 'drag-over': dragOverIndex === i }"
        draggable="true"
        @dragstart="onDragStart(i)"
        @dragover.prevent="onDragOver(i)"
        @dragend="onDragEnd"
        @drop.prevent="onDrop(i)"
      >
        <a-image
          :src="url"
          :width="200"
          :height="113"
          class="screenshot-img"
          :preview="{ visible: previewVisible, onVisibleChange: (v: boolean) => previewVisible = v }"
        />
        <div class="screenshot-actions">
          <span class="screenshot-index">#{{ i + 1 }}</span>
          <span v-if="i > 0" class="drag-handle" title="拖拽排序">≡</span>
          <a-button type="text" danger size="small" @click="remove(i)">
            <DeleteOutlined />
          </a-button>
        </div>
      </div>
    </div>

    <!-- 截图统计 -->
    <div v-if="screenshots.length" class="stats-hint">
      共 {{ screenshots.length }} 张截图 · 拖拽调整顺序 · 点击 ≡ 拖动
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

// v-model 双向（避免 mutating props）
const screenshots = computed({
  get: () => props.screenshots,
  set: (v: string[]) => emit('update:screenshots', v),
})
const screenshotInDesc = computed({
  get: () => props.screenshotInDesc,
  set: (v: boolean) => emit('update:screenshotInDesc', v),
})

const newUrl = ref('')
const previewVisible = ref(false)
const dragIndex = ref(-1)
const dragOverIndex = ref(-1)

function addUrl() {
  const url = newUrl.value.trim()
  if (!url) return
  // 简单 URL 校验
  if (!/^https?:\/\//i.test(url)) {
    return
  }
  const arr = [...screenshots.value, url]
  screenshots.value = arr
  newUrl.value = ''
}

function remove(idx: number) {
  const arr = [...screenshots.value]
  arr.splice(idx, 1)
  screenshots.value = arr
}

// 原生 HTML5 拖拽排序
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
.empty-hint {
  text-align: center;
  padding: 40px 0;
  color: #999;
}
.screenshot-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 12px;
}
.screenshot-item {
  position: relative;
  border: 1px solid #f0f0f0;
  border-radius: 4px;
  overflow: hidden;
  background: #fff;
  cursor: grab;
  transition: all 0.2s;
}
.screenshot-item:hover {
  border-color: #1677ff;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
}
.screenshot-item.dragging {
  opacity: 0.5;
  cursor: grabbing;
}
.screenshot-item.drag-over {
  border-color: #52c41a;
  border-width: 2px;
}
.screenshot-img {
  display: block;
  width: 100%;
}
.screenshot-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 8px;
  background: rgba(0, 0, 0, 0.04);
}
.screenshot-index {
  font-size: 12px;
  color: #666;
  font-weight: 600;
}
.drag-handle {
  cursor: grab;
  color: #999;
  font-size: 16px;
  line-height: 1;
  user-select: none;
}
.drag-handle:hover {
  color: #1677ff;
}
.stats-hint {
  margin-top: 8px;
  font-size: 11px;
  color: #999;
  text-align: right;
}
</style>
