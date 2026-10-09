// scripts/e2e/tab3-subtitle.js — 首个 e2e 用例（§59.319 附十三基建验收）
//
// 断言：原盘种 Under Current 的编辑器 Tab3（截图）页字幕轨下拉
// 显示语言（English/Chinese——附九语言化+附十一正则修复）。
// 运行：见 scripts/e2e/README.md
const h = require('./helpers')

;(async () => {
  const { browser, page } = await h.launch()
  try {
    await h.login(page)
    await h.seedList(page, 'PT0', 'Under Current')
    const rows = await h.tableRows(page, 3)
    console.log('ROWS=' + JSON.stringify(rows))
    await h.openEditor(page, 'Under Current')
    await h.switchTab(page, '截图')
    const opts = await h.dropdownOptions(page, '.ant-drawer-content')
    console.log('SUBTITLE_OPTIONS=' + JSON.stringify(opts))
    const joined = (opts || []).join('|')
    if (!joined.includes('English') || !joined.includes('Chinese')) {
      console.log('ASSERT_FAIL: expect English & Chinese in options')
      process.exitCode = 1
    } else {
      console.log('ASSERT_PASS')
    }
  } catch (e) {
    const snap = await h.snapshot(page).catch(() => ({}))
    console.log('ERR=' + e.message)
    console.log('SNAPSHOT=' + JSON.stringify(snap))
    process.exitCode = 1
  } finally {
    await browser.close()
  }
})()
