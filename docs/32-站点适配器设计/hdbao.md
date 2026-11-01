# 海的宝（hdbao.cc）站点适配器设计

> 无文档站实时建档（§59.385 咖啡路径第二站——sites.json form+实时
> HTML 为源，2026-11-01 v0.0.1256 §59.406）。

## 发布适配记录（v0.0.1256 §59.406）

- **form_config**：migration91 分发（七槽 64 项——type 11/medium 10/
  codec 8/audiocodec 13/standard 6/team 10/tags 6）。实时 HTML（43.3KB
  直连）与 4 月资产全槽一致；tags 资产缺口实时补全（None→6 项）。
  [4] 后缀标准形态；单模式站。
- **无 url/dburl 字段**（实证——控件唯一 url 是编辑器按钮；IMDb/豆瓣
  信息走 pt_gen）；有 pt_gen+nfo+technical_info+uplver。
- **槽位特化**：type **408 Concert/演唱会=category.concert 第三站**
  （浦园/咖啡后）+406 MV 双键+409 Misc/音乐=lossless_music+410
  Tutorial=education+405 label 含 HTML 实体（&amp; 解码后配置——
  自校对照需实体解码）；medium 10 项**原盘×DIY 四分**（UHD 原盘/
  UHD DIY/BD 原盘/BD DIY——DIY 特化 label-only 冬樱先例+原盘两项
  分立配键）；codec 8 项合并命名（联动零触发）无 VP9；audiocodec
  13 项 DTS:X/Atmos/HD MA/TrueHD 独立+DD/AC3 三键+OGG/APE；
  standard 6 项乱序 **p/i 分立第五站**（5 4K/8 1080p/2 1080i/7 720p/
  10 SD/11 Other）；team 10 项 label-only（HDBao 官组+外站八组
  CHD/MySiLU/WiKi/HDC/CMCT/HDSky/HHClub/HD4FANS）；tags 6 项
  **HDR 四键泛家族第四站**+首发铁律+禁转/国语/中字/DIY 配键。
- **验证**：裸对 64 项逐字（实体解码）✓+formdiff 同栈 0 条（双零
  发版）+DryRun 双形态全绿（404 纪录双键/7 WEB-DL；压制种 401/
  6 Encode/11 DTS-HD MA/tags=[中字,HDR] 四键复利）。
- 遗留：首颗真实发布。
