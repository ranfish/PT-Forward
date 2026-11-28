// §59.596 时长单位统一（分/时/天三选）——换算单点。
//
// 与 formatDurationSec（utils/format.ts）构成「输入换算/展示格式」双单点：
// - 本模块管规则/配置输入侧的「显示值 ↔ 底层存储值」换算（可逆·可锚）
// - formatDurationSec 管只读展示（秒 → "1h 0m"）
//
// 单位枚举固定三选（用户定案：不加秒——防秒粒度短时调度）。
// 底层存储单位由字段声明（base seconds / hours），存储层零变更。
//
// 存量兼容（回显双轨）：
// - 无 unit 的存量条件按字段缺省单位回显（hours 字段=小时——与旧
//   ÷3600 行为逐字一致·864000 → 240 小时·零惊扰）
// - 新条件默认 minutes（用户决策②）

export type DurationUnit = 'minutes' | 'hours' | 'days'

const UNIT_SECONDS: Record<DurationUnit, number> = {
  minutes: 60,
  hours: 3600,
  days: 86400,
}

/** 单位 → 秒因子（分 60 / 时 3600 / 天 86400）。非法单位返回 0。 */
export function durationUnitSeconds(u: string): number {
  return (UNIT_SECONDS as Record<string, number>)[u] ?? 0
}

/** DurationUnit 白名单判别（后端 ValidateConditions 同词表·§59.596）。 */
export function isDurationUnit(u: unknown): u is DurationUnit {
  return u === 'minutes' || u === 'hours' || u === 'days'
}

/**
 * 切换单位时换算显示值（用户决策②：30 分 → 切小时 = 0.5）。
 * 保留 4 位小数（50 分 → 0.8333 时·回切误差 < 1 秒量级）。
 * 非有限数值原样返回（空输入不换算）。
 */
export function convertDuration(value: number, from: DurationUnit, to: DurationUnit): number {
  if (!Number.isFinite(value)) return value
  return Math.round((value * UNIT_SECONDS[from] / UNIT_SECONDS[to]) * 10000) / 10000
}

/**
 * 显示值 → 底层存储值。
 * store = display × unitSeconds(unit) ÷ baseSeconds（base 为字段底层单位秒数）。
 * 例：seed_time（base 秒）选「天」填 10 → 10×86400/1 = 864000。
 * hr_seed_time_h（base 小时=3600）选「天」填 2 → 2×86400/3600 = 48。
 */
export function durationStoreValue(display: number, unit: DurationUnit, baseSeconds: number): number {
  if (!Number.isFinite(display)) return display
  return Math.round((display * UNIT_SECONDS[unit]) / baseSeconds)
}

/**
 * 底层存储值 → 显示值（durationStoreValue 的逆）。
 * display = store × baseSeconds ÷ unitSeconds(unit)·保留 2 位小数。
 * 例：864000 秒（addedTime 存量）unit=hours → 240；unit=days → 10。
 */
export function durationDisplayValue(store: number, unit: DurationUnit, baseSeconds: number): number {
  if (!Number.isFinite(store)) return store
  return Math.round((store * baseSeconds / UNIT_SECONDS[unit]) * 100) / 100
}

/**
 * 按秒值智能选单位（fit_time 等无持久 unit 的回显）：
 * ≥1 天 → days·≥1 小时 → hours·否则 minutes。0 → minutes。
 */
export function smartDurationUnit(seconds: number): DurationUnit {
  if (seconds >= 86400) return 'days'
  if (seconds >= 3600) return 'hours'
  return 'minutes'
}
