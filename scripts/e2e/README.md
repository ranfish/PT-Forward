# PT-Forward UI 自动化（e2e）

Playwright 容器化 UI 验证（§59.319 附十三基建）——登录/导航/antd 下拉/
种子列表/编辑器/tab 切换/断言封装在 `helpers.js`，用例只写业务断言。

## 运行（29/243 任一环境的容器外宿主机执行）

```bash
# 环境变量（默认 localhost:8765/admin；PTF_PASS 必填）
export PTF_BASE=http://10.0.0.243:8765   # 可选
export PTF_PASS='目标环境密码'

sg docker -c "docker run --rm --network host \
  -v $(pwd)/scripts/e2e:/work -w /work \
  -e PTF_BASE=$PTF_BASE -e PTF_PASS=\"$PTF_PASS\" \
  -e PLAYWRIGHT_BROWSERS_PATH=/ms-playwright \
  mcr.microsoft.com/playwright:v1.53.0-noble \
  bash -c 'npm install playwright-core@1.53.0 --no-save --silent 2>/dev/null; node tab3-subtitle.js'"
```

## 用例

| 脚本 | 断言 |
|------|------|
| `tab3-subtitle.js` | 原盘种 Tab3 字幕轨下拉显示语言（English/Chinese） |

## 约定

- 用例退出码：0=断言通过，1=失败（ERR=/ASSERT_FAIL= 行有诊断信息）
- 新用例：require('./helpers')，只用 helpers 提供的交互原语；
  交互知识（antd 选择器细节）只沉淀在 helpers，用例不写选择器
- 目标站为 SPA：`history.pushState` 直跳路由对部分页面无效，
  统一走 `navMenu` 菜单点击
- 种子配置列表**必须先选下载器**（不选无行）——`seedList` 已内聚
