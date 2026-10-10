// scripts/e2e/dd-spec.js — §59.319 附二十验证：Detective.Dee ISO 编辑器
// Tab1 技术规格完整性
const h = require('./helpers')

;(async () => {
  const { browser, page } = await h.launch()
  try {
    await h.login(page)
    await h.seedList(page, 'PT0', 'Detective')
    const rows = await h.tableRows(page, 3)
    console.log('ROWS=' + JSON.stringify(rows))
    await h.openEditor(page, 'Detective')
    await h.switchTab(page, '种子详情')
    await page.waitForTimeout(2000)
    const text = await page.$eval('.ant-drawer-content', el => el.innerText)
    // 技术规格区全部行
    const lines = text.split('\n').filter(l =>
      /规格|片源|媒介|分辨率|编码|音频|声道|HDR|bit|帧率|制作组|标签|Encode|原盘/i.test(l))
    console.log('SPEC_LINES=' + JSON.stringify(lines.slice(0, 20)))
    // 空值检查（"—" 表示缺失）
    const missing = lines.filter(l => /—/.test(l))
    console.log('MISSING_FIELDS=' + JSON.stringify(missing))
  } catch (e) {
    console.log('ERR=' + e.message)
    process.exitCode = 1
  } finally {
    await browser.close()
  }
})()
