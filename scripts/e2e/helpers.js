// scripts/e2e/helpers.js — PT-Forward UI 自动化基础库（§59.319 附十三）
//
// 沉淀自本会话 5 轮 Playwright 迭代的交互知识，供所有 e2e 用例复用：
//   登录（antd 表单）→ 菜单导航（submenu 展开）→ ant-select 下拉交互
//   → 种子列表（先选下载器才显行）→ 搜索 → 打开编辑器 → 切 tab → 断言。
//
// 运行环境：Playwright 容器（见 README.md），环境变量：
//   PTF_BASE  默认 http://localhost:8765
//   PTF_USER  默认 admin
//   PTF_PASS  必填
const { chromium } = require('playwright-core')

const BASE = process.env.PTF_BASE || 'http://localhost:8765'
const USER = process.env.PTF_USER || 'admin'
const PASS = process.env.PTF_PASS

async function launch() {
  const browser = await chromium.launch({ args: ['--no-sandbox'] })
  const page = await browser.newPage({ viewport: { width: 1600, height: 900 } })
  page.setDefaultTimeout(15000)
  return { browser, page }
}

// login — 登录并等待进入主界面（侧栏菜单出现）
async function login(page) {
  await page.goto(BASE + '/')
  await page.waitForTimeout(2500)
  const userInput = await page.$('input[type="text"]')
  if (userInput) {
    // 登录页可见（已登录态会直接进主界面，跳过）
    await userInput.fill(USER)
    await page.fill('input[type="password"]', PASS)
    await page.click('button[type="submit"], button:has-text("登")')
  }
  await page.waitForSelector('.ant-menu, .ant-layout-sider', { timeout: 15000 })
  await page.waitForTimeout(1500)
  return page.url()
}

// navMenu — 侧栏导航：先展开分组（如"发布管理"），再点子项（如"种子配置"）
async function navMenu(page, group, item) {
  const submenu = await page.$(`.ant-menu-submenu-title:has-text("${group}")`)
  if (submenu) {
    const expanded = await submenu.getAttribute('aria-expanded')
    if (expanded !== 'true') await submenu.dispatchEvent('click')
    await page.waitForTimeout(800)
  }
  await (await page.$(`.ant-menu-item:has-text("${item}")`)).dispatchEvent('click')
  await page.waitForTimeout(2500)
}

// selectAntDropdown — 在 scope 选择器范围内选 ant-select 下拉项（按可见文本包含匹配）
// 用于：下载器筛选、编辑器内任意下拉
async function selectAntDropdown(page, scopeSel, valueText) {
  const sel = await page.$(`${scopeSel} .ant-select:not(.ant-select-open) .ant-select-selector`)
  if (!sel) throw new Error(`no ant-select in ${scopeSel}`)
  await sel.dispatchEvent('mousedown') // antd Select 展开监听 mousedown
  await page.waitForSelector('.ant-select-dropdown:visible .ant-select-item-option', { timeout: 8000 })
  const opt = await page.$(`.ant-select-dropdown:visible .ant-select-item-option:has-text("${valueText}")`)
  if (!opt) {
    const all = await page.$$eval('.ant-select-dropdown:visible .ant-select-item-option-content',
      els => els.map(e => e.textContent.trim()))
    throw new Error(`option "${valueText}" not found; available: ${JSON.stringify(all)}`)
  }
  await opt.dispatchEvent('click')
  await page.waitForTimeout(2000)
}

// seedList — 种子配置页：选下载器（必做——不选无行）+可选搜索
async function seedList(page, downloaderText, keyword) {
  await navMenu(page, '发布管理', '种子配置')
  await page.waitForSelector('.ant-select', { timeout: 10000 })
  await page.waitForTimeout(1500)
  // 筛选区第一个 ant-select = 下载器下拉（页面语境唯一；容器类名不定，
  // 不限定 scope——取主内容区第一个未展开的）
  const sel = await page.$('.ant-layout-content .ant-select:not(.ant-select-open) .ant-select-selector')
  if (!sel) throw new Error('downloader filter select not found')
  await sel.dispatchEvent('mousedown') // antd Select 展开监听 mousedown
  await page.waitForSelector('.ant-select-dropdown:visible .ant-select-item-option', { timeout: 8000 })
  const opt = await page.$(`.ant-select-dropdown:visible .ant-select-item-option:has-text("${downloaderText}")`)
  if (!opt) {
    const all = await page.$$eval('.ant-select-dropdown:visible .ant-select-item-option-content',
      els => els.map(e => e.textContent.trim()))
    throw new Error(`downloader "${downloaderText}" not found; available: ${JSON.stringify(all)}`)
  }
  await opt.dispatchEvent('click')
  await page.waitForTimeout(3000)
  if (keyword) {
    const search = await page.$('input[placeholder*="搜索"], input[placeholder*="名"]')
    if (search) {
      await search.fill(keyword)
      await search.press('Enter')
      await page.waitForTimeout(3000)
    }
    // §59.319 附十三：搜索可靠性——验证行内含关键词，否则点搜索按钮兜底
    // （antd Input.Search 的 Enter 触发 onSearch 有竞态；列表分页 792 条时
    // 未过滤的首页不含目标行）
    let rows = await tableRows(page, 5)
    if (!rows.some(r => r.includes(keyword.split(' ')[0]))) {
      const searchBtn = await page.$('.ant-input-search-button, button:has-text("搜索")')
      if (searchBtn) {
        await searchBtn.dispatchEvent('click')
        await page.waitForTimeout(3000)
      }
      rows = await tableRows(page, 5)
    }
    if (!rows.some(r => r.includes(keyword.split(' ')[0]))) {
      throw new Error(`search "${keyword}" not effective; first rows: ${JSON.stringify(rows)}`)
    }
  }
}

// openEditor — 打开列表行的编辑器（"编辑"按钮；行内操作按钮 hover 才显示则先 hover）
async function openEditor(page, rowText) {
  const rowTextSel = rowText ? `tr:has-text("${rowText}")` : '.ant-table-tbody tr'
  // antd 固定列克隆 DOM：操作列存在原件+克隆两份（其一 hidden）——
  // hover/click 的 actionability 检查对 hidden 份永久超时；统一
  // dispatchEvent 直派 DOM click（React 合成事件监听底层 click，
  // 克隆份同样绑定 onClick——100% 触发且零 actionability 检查）
  let btn = await page.$(`${rowTextSel} button:has-text("编辑")`)
  if (!btn) {
    const btns = await page.$$eval('.ant-table-tbody button',
      els => [...new Set(els.map(e => e.textContent.trim()))].filter(Boolean))
    throw new Error(`edit btn not found; row buttons: ${JSON.stringify(btns)}`)
  }
  await btn.dispatchEvent('click')
  await page.waitForSelector('.ant-tabs-tab', { timeout: 10000 })
  await page.waitForTimeout(2000)
}

// switchTab — 切换编辑器/页面 tab（按文本，如"截图"）
async function switchTab(page, tabText) {
  const tab = await page.$(`.ant-tabs-tab:has-text("${tabText}")`)
  if (!tab) {
    const tabs = await page.$$eval('.ant-tabs-tab', els => els.map(e => e.textContent.trim()))
    throw new Error(`tab "${tabText}" not found; tabs: ${JSON.stringify(tabs)}`)
  }
  await tab.dispatchEvent('click')
  await page.waitForTimeout(2000)
}

// dropdownOptions — 展开 scope 内第一个 ant-select，返回全部选项文本
async function dropdownOptions(page, scopeSel) {
  const sel = await page.$(`${scopeSel || ''} .ant-select .ant-select-selector`.trim())
  if (!sel) return null
  await sel.dispatchEvent('mousedown') // antd Select 展开监听 mousedown
  await page.waitForSelector('.ant-select-dropdown:visible .ant-select-item-option', { timeout: 8000 })
  await page.waitForTimeout(800)
  const opts = await page.$$eval('.ant-select-dropdown:visible .ant-select-item-option-content',
    els => els.map(e => e.textContent.trim()))
  await page.keyboard.press('Escape') // 收起下拉（不改变已选值）
  return opts
}

// tableRows — 返回当前列表行文本（前 N 列拼接），用于断言/调试
async function tableRows(page, limit = 8) {
  return page.$$eval('.ant-table-tbody tr',
    (trs, lim) => trs.slice(0, lim).map(tr => tr.innerText.split('\n').slice(0, 3).join('|')),
    limit)
}

// snapshot — 调试辅助：页面标题+当前 URL+可见文本片段
async function snapshot(page) {
  const text = (await page.$$eval('body', els => els[0].innerText)).slice(0, 400)
  return { url: page.url(), title: await page.title(), text }
}

module.exports = {
  BASE, launch, login, navMenu, selectAntDropdown, seedList,
  openEditor, switchTab, dropdownOptions, tableRows, snapshot,
}
