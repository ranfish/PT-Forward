import { describe, expect, it } from 'vitest'
import {
  convertDuration,
  durationDisplayValue,
  durationStoreValue,
  durationUnitSeconds,
  isDurationUnit,
  smartDurationUnit,
} from './duration'

// §59.596 时长单位统一——换算单点锚（UI 原子「数值+单位」·底层秒基准）。
// 三阴性：A=退固定 3600（store 忽略 unit）→ days 用例 FAIL
//         B=删缺省/白名单（isDurationUnit 收窄）→ 合法值 FAIL
//         C=smart 边界改档 → 边界用例 FAIL

describe('durationUnitSeconds（秒基准单点）', () => {
  it('三选单位因子', () => {
    expect(durationUnitSeconds('minutes')).toBe(60)
    expect(durationUnitSeconds('hours')).toBe(3600)
    expect(durationUnitSeconds('days')).toBe(86400)
  })
  it('非法单位返回 0（白名单外）', () => {
    expect(durationUnitSeconds('seconds')).toBe(0)
    expect(durationUnitSeconds('')).toBe(0)
    expect(durationUnitSeconds('week')).toBe(0)
  })
})

describe('isDurationUnit（白名单判别）', () => {
  it('三值合法', () => {
    expect(isDurationUnit('minutes')).toBe(true)
    expect(isDurationUnit('hours')).toBe(true)
    expect(isDurationUnit('days')).toBe(true)
  })
  it('其余非法（含 seconds——用户定案不加秒）', () => {
    expect(isDurationUnit('seconds')).toBe(false)
    expect(isDurationUnit(undefined)).toBe(false)
    expect(isDurationUnit('')).toBe(false)
  })
})

describe('convertDuration（切换单位自动换算·§59.596 决策②）', () => {
  it('30 分 → 小时 = 0.5（用户原始用例）', () => {
    expect(convertDuration(30, 'minutes', 'hours')).toBe(0.5)
  })
  it('0.5 小时 → 分 = 30（往返无损）', () => {
    expect(convertDuration(0.5, 'hours', 'minutes')).toBe(30)
  })
  it('50 分 → 小时 = 0.8333（4 位小数·回切误差 <1 秒）', () => {
    expect(convertDuration(50, 'minutes', 'hours')).toBe(0.8333)
  })
  it('2 天 → 小时 = 48', () => {
    expect(convertDuration(2, 'days', 'hours')).toBe(48)
  })
  it('0 恒 0（fit_time 0=立即语义天然兼容）', () => {
    expect(convertDuration(0, 'minutes', 'days')).toBe(0)
  })
})

describe('durationStoreValue（显示 → 底层存储）', () => {
  it('seed_time（base 秒）选天填 10 → 864000（核心场景）', () => {
    expect(durationStoreValue(10, 'days', 1)).toBe(864000)
  })
  it('hr_seed_time_h（base 小时）选天填 2 → 48', () => {
    expect(durationStoreValue(2, 'days', 3600)).toBe(48)
  })
  it('选分填 30 → 1800 秒', () => {
    expect(durationStoreValue(30, 'minutes', 1)).toBe(1800)
  })
  it('0 天然兼容（fit_time 0=立即）', () => {
    expect(durationStoreValue(0, 'hours', 1)).toBe(0)
  })
})

describe('durationDisplayValue（底层存储 → 显示·store 的逆）', () => {
  it('864000 秒 unit=days → 10（还原「10 天」形态）', () => {
    expect(durationDisplayValue(864000, 'days', 1)).toBe(10)
  })
  it('864000 秒 unit=hours → 240（存量无 unit 缺省链——与旧 ÷3600 逐字一致）', () => {
    expect(durationDisplayValue(864000, 'hours', 1)).toBe(240)
  })
  it('0.5 天落库 43200 → 还原 0.5（UI 原子保真）', () => {
    expect(durationDisplayValue(43200, 'days', 1)).toBe(0.5)
  })
})

describe('store/display 往返（端到端换算核）', () => {
  const cases: [number, 'minutes' | 'hours' | 'days', number][] = [
    [0.5, 'days', 1],
    [10, 'days', 1],
    [240, 'hours', 1],
    [30, 'minutes', 1],
    [2, 'days', 3600], // hr_seed_time_h 小时 base
  ]
  it.each(cases)('display %s %s(base %s) 往返无损', (display, unit, base) => {
    const store = durationStoreValue(display, unit, base)
    expect(durationDisplayValue(store, unit, base)).toBe(display)
  })
})

describe('smartDurationUnit（fit_time 智能回显）', () => {
  it('≥1 天 → days', () => {
    expect(smartDurationUnit(86400)).toBe('days')
    expect(smartDurationUnit(864000)).toBe('days')
  })
  it('≥1 小时 <1 天 → hours', () => {
    expect(smartDurationUnit(3600)).toBe('hours')
    expect(smartDurationUnit(86399)).toBe('hours')
  })
  it('<1 小时 → minutes（含 0）', () => {
    expect(smartDurationUnit(3599)).toBe('minutes')
    expect(smartDurationUnit(0)).toBe('minutes')
  })
})
