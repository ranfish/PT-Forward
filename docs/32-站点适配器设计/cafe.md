# 咖啡 站点适配器设计

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 咖啡 |
| 站点地址 | https://ptcafe.club |
| 站点框架 | NexusPHP |
| Tracker | https://tracker.ptcafe.club/announce.php |

## 发布表单（upload.php?mode=advanced 实时 HTML 2026-10-31）

**重要**：upload.php 默认页是**外部发布器简版**（仅 file/generator/
name/nfo/small_descr/descr）；完整表单需 `mode=advanced` 参数。
**表单无 url/pt_gen 字段**；有 uplver（匿名）。

### 槽位（label=站方原文逐字）

- type 9 项：401 电影/402 剧集/403 综艺/404 纪录/405 动漫/406 演唱/
  407 体育/408 音乐/409 其他
- source_sel[4] 7 项（**产区槽**：大陆/港台/欧美/日本/韩国/印度/
  其他）——发布链无产区数据不配（npFieldMap "source"=媒介来源
  ≠本槽语义——同名异义陷阱）
- medium_sel[4] 13 项（行 label"质量"）：UHD Blu-ray 原盘(1)/
  UHD Blu-ray DIY(2)/UHD Remux(3)/Blu-ray 原盘(4)/Blu-ray DIY(5)/
  Remux(6)/Encode(7)/WEB-DL(8)/TV(9)/DVD(10)/CD(11)/Track(12)/
  Other(13)
- codec_sel[4] 11 项：H.265/HEVC(1)/H.264/AVC(2)/X265(3)/X264(4)/
  VC-1(5)/MPEG-2(6)/MPEG-4(7)/XVID(8)/VP9(9)/DIVX(10)/Other(11)
  ——H 标准名+X 编码器名并存（§59.380 压制联动第四站）
- audiocodec_sel[4] 18 项：DTS-HDMA:X 7.1(1)/DTS-HDMA(2)/
  DTS-HDHR(3)/DTS-HD(4)/DTS-X(5)/LPCM(6)/AC3(7)/Atmos(8)/AAC(9)/
  TrueHD(10)/DTS(11)/FLAC(12)/APE(13)/MP3(14)/WAV(15)/OPUS(16)/
  OGG(17)/Other(18)——**无 DDP/DD 独立项**（DDP 并入 AC3）
- standard_sel[4] 6 项（**p/i 合并形态**）：4320P/8K/FUHD(1)/
  2160P/4K/UHD(2)/1080p/1080i/FHD(3)/720p/720i/HD(4)/
  360p/360i/SD(5)/Other(6)
- team_sel[4] 30 项（全库最多）：ADE/ADWeb/Audies/beAst/BeiTai/
  BeyondHD/BtsTV/CafeTV/CafeWEB/CHDBits/CHDWEB/CMCT/DJWEB/FRDS/
  HDCTV/HDH/HDHome/HDSky/HDSWEB/HHWEB/MTeam/MWeb/OurBits/OurTV/
  PTCafe/PTerWEB/QHstudIo/TTG/WiKi/Other——CafeTV/CafeWEB/PTCafe
  为本站官组
- tags[4][] 15 项（value 6 缺失）：官方(1)/首发(2)/完结(3)/原创(4)/
  禁转(5)/国语(7)/粤语(8)/中字(9)/备胎(10)/杜比视界(11)/HDR(12)/
  DIY(13)/应求(14)/高码高帧(15)/月月(16)

## 适配记录（v0.0.1234 §59.385）

- migration72 分发+publish_supported 开闸；实时 HTML 102 项 label
  零漂移自校（对照源=站方 HTML 原文——§59.384 附教训落实）。
- DryRun 双形态全绿：WEB-DL 原生 404/8/1/9/2/30；压制种 401/7
  Encode/**3 X265（§59.380 联动第四站）**/2 DTS-HDMA/14 FRDS。
- HDMA:X(1) label-only（组合粒度 meta 不可达保守不猜）；独立
  DTS-X(5)=dtsx（与冬樱形态差异：冬樱无独立项 dtsx 给 HDMA:X）。
- 遗留：首颗真实发布（source_sel 空值若被站方校验拒再议产区建设）。
