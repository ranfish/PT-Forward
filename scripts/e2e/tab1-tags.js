// scripts/e2e/tab1-tags.js — §59.319 附十七浏览器验证：Under Current
// 编辑器 Tab1 标签（high_bitrate 应出现）
const h = require('./helpers')

;(async () => {
  const { browser, page } = await h.launch()
  try {
    await h.login(page)
    await h.seedList(page, 'PT0', 'Under Current')
    await h.openEditor(page, 'Under Current')
    await h.switchTab(page, '种子详情')
    await page.waitForTimeout(2000)
    const text = await page.$eval('.ant-drawer-content', el => el.innerText)
    // 找标签相关行
    const lines = text.split('\n').filter(l => /标签|高码|high_bitrate|原盘|中字/i.test(l))
    console.log('TAG_LINES=' + JSON.stringify(lines.slice(0, 10)))
    // 断言高码出现
    if (!text.includes('高码') && !text.includes('high_bitrate')) {
      console.log('ASSERT_FAIL: expect 高码/high_bitrate in Tab1')
      process.exitCode = 1
    } else {
      console.log('ASSERT_PASS')
    }
  } catch (e) {
    console.log('ERR=' + e.message)
    process.exitCode = 1
  } finally {
    await browser.close()
  }
})()
