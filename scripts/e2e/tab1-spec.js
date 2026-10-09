// scripts/e2e/tab1-spec.js — §59.319 附十六浏览器验证：Under Current
// 编辑器 Tab1 技术规格（Encode 应为 false/原盘形态展示）
const h = require('./helpers')

;(async () => {
  const { browser, page } = await h.launch()
  try {
    await h.login(page)
    await h.seedList(page, 'PT0', 'Under Current')
    await h.openEditor(page, 'Under Current')
    // Tab1（第一个 tab——作品/种子信息）
    const tabs = await page.$$eval('.ant-drawer-content .ant-tabs-tab', els => els.map(e => e.textContent.trim()))
    console.log('TABS=' + JSON.stringify(tabs))
    if (tabs.length > 0) {
      await h.switchTab(page, tabs[0].slice(0, 4))
    }
    await page.waitForTimeout(2000)
    const text = await page.$eval('.ant-drawer-content', el => el.innerText)
    // 找规格/Encode 相关行
    const lines = text.split('\n').filter(l => /规格|Encode|encode|原盘|压制|Blu-?ray/i.test(l))
    console.log('SPEC_LINES=' + JSON.stringify(lines.slice(0, 12)))
  } catch (e) {
    const snap = await h.snapshot(page).catch(() => ({}))
    console.log('ERR=' + e.message)
    console.log('SNAPSHOT=' + JSON.stringify(snap).slice(0, 300))
    process.exitCode = 1
  } finally {
    await browser.close()
  }
})()
