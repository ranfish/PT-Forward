/**
 * §59.20 BBCode → HTML 渲染（前端预览用）
 * 支持 PT 站常见 BBCode 标签：quote/b/img/url/size/color/bold/italic/align/code/pre/nl
 */

export function parseBBCode(text: string): string {
  if (!text) return ''

  // 先转义 HTML 特殊字符（防止 XSS）
  let html = text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')

  // [quote]...[/quote]
  html = html.replace(/\[quote\]/gi, '<blockquote style="margin:8px 0;padding:8px 12px;border-left:3px solid #d9d9d9;background:#fafafa;color:#666;font-size:13px">')
  html = html.replace(/\[\/quote\]/gi, '</blockquote>')

  // [code]...[/code]
  html = html.replace(/\[code\]/gi, '<pre style="margin:8px 0;padding:12px;background:#f5f5f5;border-radius:4px;font-family:monospace;font-size:12px;overflow-x:auto">')
  html = html.replace(/\[\/code\]/gi, '</pre>')

  // [b]...[/b]
  html = html.replace(/\[b\]/gi, '<strong>').replace(/\[\/b\]/gi, '</strong>')
  // [i]...[/i]
  html = html.replace(/\[i\]/gi, '<em>').replace(/\[\/i\]/gi, '</em>')
  // [u]...[/u]
  html = html.replace(/\[u\]/gi, '<u>').replace(/\[\/u\]/gi, '</u>')

  // [size=X]...[/size]
  html = html.replace(/\[size=(\d+)\]/gi, (_, size) => `<span style="font-size:${size}px">`)
  html = html.replace(/\[\/size\]/gi, '</span>')

  // [color=X]...[/color]
  html = html.replace(/\[color=([#\w]+)\]/gi, (_, color) => `<span style="color:${color}">`)
  html = html.replace(/\[\/color\]/gi, '</span>')

  // [url]...[/url]
  html = html.replace(/\[url\](https?:\/\/[^\]]+)\[\/url\]/gi, '<a href="$1" target="_blank" rel="noopener">$1</a>')
  // [url=...]...[/url]
  html = html.replace(/\[url=(https?:\/\/[^\]]+)\]([^[]*)\[\/url\]/gi, '<a href="$1" target="_blank" rel="noopener">$2</a>')

  // [img]...[/img]
  html = html.replace(/\[img\](https?:\/\/[^\]]+)\[\/img\]/gi, '<img src="$1" style="max-width:100%;border-radius:4px;margin:4px 0" />')

  // [align=center|left|right]...[/align]
  html = html.replace(/\[align=(center|left|right)\]/gi, '<div style="text-align:$1">')
  html = html.replace(/\[\/align\]/gi, '</div>')

  // 换行
  html = html.replace(/\r\n/g, '\n')
  html = html.replace(/\n/g, '<br>')

  // 清理连续 <br> 在 blockquote/pre 内的多余换行
  html = html.replace(/<blockquote><br>/gi, '<blockquote>')
  html = html.replace(/<br><\/blockquote>/gi, '</blockquote>')
  html = html.replace(/<pre><br>/gi, '<pre>')
  html = html.replace(/<br><\/pre>/gi, '</pre>')

  return html
}
