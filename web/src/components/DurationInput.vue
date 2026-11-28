<script setup lang="ts">
// §59.596 刀2: 时长输入公共组件——「数值+单位」双件套（分/时/天三选）。
// 抽象契机（三次法则）：刀1 fit_time 手搓模式复制到第 3+ 处即须抽组件
// （§59.26/§59.156/§59.159 重复造轮子纪律）。
//
// 用法（配合智能回显的 load/save 换算——各字段底层单位不动·弱读）：
//   加载：unit = smartDurationField(store); display = durationDisplayValue(store, unit, baseSec)
//   保存：store = durationStoreValue(display, unit, baseSec)
//   模板：<DurationInput v-model:display="xxDisplay" v-model:unit="xxUnit" :min="0" />
//
// 切换单位自动换算显示值（决策②：30 分 → 切小时 = 0.5·4 位小数）。
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { convertDuration } from '@/utils/duration'
import type { DurationUnit } from '@/utils/duration'

const props = withDefaults(defineProps<{
  display: number
  unit: DurationUnit
  min?: number
  max?: number
  step?: number
}>(), { min: 0 })

const emit = defineEmits<{
  (e: 'update:display', v: number): void
  (e: 'update:unit', u: DurationUnit): void
}>()

const { t } = useI18n()
const units = computed(() => [
  { value: 'minutes', label: t('common.unitMinutes') },
  { value: 'hours', label: t('common.unitHours') },
  { value: 'days', label: t('common.unitDays') },
])

function onUnitChange(u: DurationUnit) {
  const n = props.display
  if (Number.isFinite(n)) {
    emit('update:display', convertDuration(n, props.unit, u))
  }
  emit('update:unit', u)
}
</script>

<template>
  <div style="display: flex; gap: 8px">
    <a-input-number
      :value="display"
      :min="min"
      :max="max"
      :step="step"
      style="flex: 1"
      @change="(v: number | null) => emit('update:display', v ?? 0)"
    />
    <a-select :value="unit" style="width: 90px" @change="(u: DurationUnit) => onUnitChange(u)">
      <a-select-option v-for="u in units" :key="u.value" :value="u.value">{{ u.label }}</a-select-option>
    </a-select>
  </div>
</template>
