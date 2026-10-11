const h = require('./helpers')
;(async () => {
  const { browser, page } = await h.launch()
  try {
    await h.login(page)
    await h.seedList(page, 'PT0', 'Mercy')
    await h.openEditor(page, 'Mercy')
    await h.switchTab(page, '种子详情')
    await page.waitForTimeout(2000)
    const text = await page.$eval('.ant-drawer-content', el => el.innerText)
    const lines = text.split('\n').filter(l => /媒介|原盘|UHD|标签|HDR|Atmos|高码|中字/i.test(l))
    console.log('SPEC_LINES=' + JSON.stringify(lines.slice(0, 8)))
  } catch (e) {
    console.log('ERR=' + e.message)
    process.exitCode = 1
  } finally { await browser.close() }
})()
