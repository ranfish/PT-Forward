<template>
  <div class="site-tiles">
    <div
      v-for="t in sites"
      :key="t.name"
      class="site-tile"
      :class="{ active: isActive(t.name) }"
      @click="pick(t.name)"
    >
      <span class="tile-name">{{ t.name }}</span>
      <a-tag v-if="t.hasPreAudit" color="blue" class="tile-tag">官方预检</a-tag>
    </div>
  </div>
  <div v-if="!sites.length" class="tile-empty">暂无已启用发布配置的目标站</div>
</template>

<script setup lang="ts">
// §59.166 SiteTiles 站点平铺点选公共组件（一种多站 Modal 多选 / 一站多种弹窗单选）。
// 单选：modelValue=string（再点同一个=取消选择）；多选：modelValue=string[]。
export interface SiteTileItem {
  name: string
  hasPreAudit?: boolean
}

const props = defineProps<{
  sites: SiteTileItem[]
  multiple?: boolean
  modelValue: string | string[]
}>()

const emit = defineEmits<{ (e: 'update:modelValue', v: string | string[]): void }>()

function isActive(name: string): boolean {
  return props.multiple
    ? Array.isArray(props.modelValue) && props.modelValue.includes(name)
    : props.modelValue === name
}

function pick(name: string) {
  if (props.multiple) {
    const arr = Array.isArray(props.modelValue) ? [...props.modelValue] : []
    const i = arr.indexOf(name)
    if (i >= 0) arr.splice(i, 1)
    else arr.push(name)
    emit('update:modelValue', arr)
  } else {
    emit('update:modelValue', props.modelValue === name ? '' : name)
  }
}
</script>

<style scoped>
.site-tiles {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}
.site-tile {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border: 1.5px solid #d9d9d9;
  border-radius: 8px;
  cursor: pointer;
  user-select: none;
  transition: all 0.15s ease;
  background: #fafafa;
}
.site-tile:hover {
  border-color: #1677ff;
}
.site-tile.active {
  border-color: #1677ff;
  background: #e6f4ff;
  color: #1677ff;
  font-weight: 600;
}
.tile-empty {
  color: #999;
  padding: 12px 0;
}
.tile-tag {
  margin: 0;
}
</style>
