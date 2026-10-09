// scripts/e2e/list-scan-badge.js — §59.319 附十八验证：种子列表扫盘中
// 提示（disc_scan_pending 旋转图标）。清除重获后立即检查（8s 窗口内
// 源站 BDInfo 已落则 pending=False 也正确——记录实际形态）。
const h = require('./helpers')

;(async () => {
  const { browser, page } = await h.launch()
  try {
    await h.login(page)
    await h.seedList(page, 'PT0', 'Under Current')
    const rows = await h.tableRows(page, 3)
    console.log('ROWS=' + JSON.stringify(rows))
    // 检查状态列是否有 loading 图标
    const hasLoading = await page.$('.ant-table-tbody .anticon-loading')
    console.log('HAS_LOADING_ICON=' + (hasLoading ? 'true' : 'false'))
    // 检查状态 tag
    const tags = await page.$$eval('.ant-table-tbody .ant-tag', els => els.map(e => e.textContent.trim()).slice(0, 8))
    console.log('STATUS_TAGS=' + JSON.stringify(tags))
  } catch (e) {
    console.log('ERR=' + e.message)
    process.exitCode = 1
  } finally {
    await browser.close()
  }
})()
