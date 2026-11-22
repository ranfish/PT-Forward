// §59.553 P2-08: Blob 下载单点——收敛三处视图的 URL.createObjectURL 样板。
// 文件名/MIME/BOM 由调用方构造（PublishLogs CSV 的 \ufeff 不能丢）。
export function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  a.click()
  URL.revokeObjectURL(url)
}
