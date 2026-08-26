import { TAG_GROUPS } from '@/generated/dict'

/**
 * §59.110: 标签显示名单点（TagSelector 已选区/预览④标签共用）。
 *
 * 优先级：
 * 1. displayLabels（后端权威显示名，GET tags 双形态 tag_labels——与 modelValue 索引对齐）
 * 2. 本地 TAG_GROUPS label（编辑面板分组数据同源）
 * 3. 原文（miss 保留——新词条上线旧 bundle 不显示代码的兜底由 1 覆盖）
 *
 * 沿革：§59.106 只修了 TagSelector，预览④走 CrossSeedPanel 私有 tagDisplayName
 * 仍依赖 bundle 内 dict——japanese_audio 第三次显示代码实锤，抽取单点。
 */
export function tagDisplayName(tag: string, modelValue: string[], displayLabels?: string[] | null): string {
  const idx = modelValue.indexOf(tag)
  if (displayLabels && idx >= 0 && idx < displayLabels.length && displayLabels[idx]) {
    return displayLabels[idx]
  }
  for (const g of TAG_GROUPS) {
    for (const t of g.tags) {
      if (t.key === tag) return t.label
    }
  }
  return tag
}
