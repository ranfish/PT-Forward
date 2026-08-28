// §59.145: 分类公共展示——类型 tag 颜色单点（名称来自 generated/dict CATEGORY_LABELS 与后端同源）
const CATEGORY_TAG_COLORS: Record<string, string> = {
  'category.movie': 'blue',
  'category.tv_series': 'purple',
  'category.tv_shows': 'orange',
  'category.animation': 'pink',
  'category.documentary': 'cyan',
  'category.music': 'green',
  'category.sports': 'gold',
  'category.other': 'default',
}

export function categoryTagColor(category?: string): string {
  if (!category) return 'default'
  return CATEGORY_TAG_COLORS[category] || 'default'
}
