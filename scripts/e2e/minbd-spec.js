const h = require('./helpers')
;(async () => {
  const { browser, page } = await h.launch()
  try {
    await h.login(page)
    await h.seedList(page, 'PT0', 'Inception')
    await h.openEditor(page, 'Inception')
    await h.switchTab(page, '种子详情')
    await page.waitForTimeout(2000)
    const text = await page.$eval('.ant-drawer-content', el => el.innerText)
    const lines = text.split('\n').filter(l => /媒介|Encode|encode|原盘|标签/i.test(l))
    console.log('SPEC_LINES=' + JSON.stringify(lines.slice(0, 6)))
    // 断言：媒介应显示 Encode 非原盘
    if (text.includes('Encode') && !text.includes('原盘\t')) {
      console.log('ASSERT_PASS: MINBD 媒介=Encode')
    } else {
      console.log('ASSERT_FAIL: MINBD 媒介应 Encode')
      process.exitCode = 1
    }
  } catch (e) {
    console.log('ERR=' + e.message)
    process.exitCode = 1
  } finally { await browser.close() }
})()
