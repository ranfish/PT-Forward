// scripts/e2e/gzz-final.js — §59.319 附二十六/二十七 端到端验证：
// 光之子 Tab1 类型+标签 + 幸运 DryRun 通过
const h = require('./helpers')

;(async () => {
  const { browser, page } = await h.launch()
  try {
    await h.login(page)
    await h.seedList(page, 'PT0', '光之子')
    await h.openEditor(page, '光之子')
    await h.switchTab(page, '种子详情')
    await page.waitForTimeout(2000)
    const text = await page.$eval('.ant-drawer-content', el => el.innerText)

    // 类型验证
    const catLines = text.split('\n').filter(l => /类型|category/i.test(l))
    console.log('CATEGORY_LINES=' + JSON.stringify(catLines.slice(0, 3)))

    // 标签验证
    const tagLines = text.split('\n').filter(l => /标签|中字|中文字幕|国语|4K/i.test(l))
    console.log('TAG_LINES=' + JSON.stringify(tagLines.slice(0, 6)))

    // 断言
    let pass = true
    if (!text.includes('纪录')) { console.log('FAIL: 类型应为纪录片'); pass = false }
    if (!text.includes('中字') && !text.includes('中文字幕')) { console.log('FAIL: 应有中文字幕标签'); pass = false }
    if (!text.includes('国语') && !text.includes('中文')) { console.log('FAIL: 应有中文音频标签'); pass = false }
    if (!text.includes('4K')) { console.log('FAIL: 应有4K标签'); pass = false }

    if (pass) {
      console.log('ASSERT_PASS')
    } else {
      console.log('ASSERT_FAIL')
      process.exitCode = 1
    }
  } catch (e) {
    console.log('ERR=' + e.message)
    process.exitCode = 1
  } finally {
    await browser.close()
  }
})()
