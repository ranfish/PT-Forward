# 我的PT 站点适配器设计

> docs/32 无文档站第三例（§59.385 咖啡/§59.406 海的宝路径）——sites.json
> form 资产+站方实时 HTML 为源建档（2026-11-01，49.3KB 代理抓取）。

## 站点信息

| 项目 | 内容 |
|------|------|
| 站点名称 | 我的PT |
| 站点地址 | https://cc.mypt.cc |
| 站点框架 | NexusPHP |
| 官方组 | 无 groups 资产（转载站形态——team 表 Other/转载兜底） |
| 网络入口 | 全局代理（use_global_proxy=1） |
| 发布页面 | `upload.php` |
| 提交地址 | `takeupload.php`（POST multipart/form-data） |
| Tracker | `https://cc.mypt.cc/announce.php` |

## 表单结构（双区互斥）

**双区站**：type×2 同 name 控件（§59.372 后覆盖陷阱形态+§59.388 模式
过滤自动护——[5] 槽跳过）：

| 区域 | mode | 内容 | 槽位 |
|------|------|------|------|
| 影视区 | 4 | 电影/剧集/节目/纪录/动画/MTV/体育/短剧/戏曲 | type+medium_sel[4]+codec_sel[4]+standard_sel[4]+team_sel[4]+tags[4][]（17 项） |
| 资料区 | 5 | 有声读物/知识库/素材/软件/游戏/图片/漫画/音乐/图书/限制级 | type+source_sel[5]+audiocodec_sel[5]+processing_sel[5]+tags[5][]（13 项） |

**sites.json form 资产的两区混录陷阱**：资产 19 项 category/14 项
audio_codec/6 项 source_sel/6 项 processing_sel 是两区合集——
audiocodec_sel[5] 实为资料区**文件格式槽**（app/ipa+exe/win+iso/esd+
rar/zip+jpg/png+doc/text+gho/img+pdf/ppt+epub/mobi+ps/3dmax——非音频）。

## 发布映射（mode=4 影视区——migration101 §59.417）

### type（9 项）

| 值 | label | standard_keys |
|----|-------|---------------|
| 401 | Movies/电影 | category.movie |
| 402 | TV Series/剧集 | category.tv_series |
| 403 | TV Shows/节目 | category.tv_shows |
| 404 | DOCU/纪录片 | category.documentary+documentaries 双键 |
| 405 | Animations/动画片 | category.animation |
| 406 | MTV/演唱会/唱片 | [mv,music_video,concert] 三键第八站 |
| 407 | Sports/运动/体育 | category.sports |
| 408 | Playlet/短剧 | category.short_drama 第十站 |
| 409 | Opera/戏曲 | label-only（无键） |

**资料类种子（ebook/software/game 等）发布时 type miss 落空**——其值
在 [5] 区 type，影视区表单不可达，发布预览人工（双区站固有限制）。

### medium_sel[4]（10 项乱序全 canonical）

1 Blu-ray=[bluray,uhd_bluray]（无 UHD 独立归并第五例）/2 HD DVD/
3 Remux=[remux,uhd_remux]/4 MiniBD/5 HDTV/6 DVDR/7 Encode/8 CD/
9 Track/10 WEB-DL=[webdl,webrip]——全配键。

### codec_sel[4]（7 项）

1 H.264/2 H.265 分立（无 X26x 联动零触发）/3 Xvid/4 MPEG-2/
5 MP4/AVI=[mpeg4]/**6 RM/RMVB label-only**（无键）/7 Other。
无 AV1/VP9/VC-1（miss 落空）。

### standard_sel[4]（8 项乱序）

1 other/其它=[resolution.other]/2 1080i/P=[r1080p,r1080i] 双键/
3 720p/4 2k/高清=[r1440p]/5 4k/超清=[r2160p]/6 8k/蓝光=[r4320p]/
**7 3D/DTS label-only**（语义怪异无 canonical 键）/8 360p/480p=[sd]。

### 无 audiocodec 槽（影视区）

### team_sel[4]（5 项 label-only）

HDS/CHD/MySiLU/MY/发布者+**Other/转载(5) 兜底**（FRDS 不在表——
lookupOtherOption 前缀变体增强 §59.417：精确匹配 miss 后
HasPrefix("other") 扫描命中"Other/转载"）。

### tags[4][]（17 项配键 8）

| 值 | label | standard_keys |
|----|-------|---------------|
| 1 | 严禁转载 | tag.restricted（extended 活键） |
| 4 | DIY资源 | tag.diy |
| 5 | 简繁中字 | tag.chinese_subtitle |
| 6 | 国语 | tag.chinese_audio |
| 7 | HDR(原盘) | [hdr10,hdr10_plus,hdr_vivid,hlg] 四键泛家族第十站 |
| 13 | 已完结 | tag.complete |
| 12 | 粤语 | tag.cantonese_audio |
| 16 | 英语 | tag.english_audio（幸运先例——本站无排斥站规证据） |

**官方首发(2) 绝不配键**（转存发布禁勾首发铁律）+日韩剧/火种(零魔)/
长期保种/恐怖暴力/少儿不宜/欧美剧/国产剧/港奥台 9 项 label-only。

### 字段

url（IMDb）+technical_info+uplver；**无 pt_gen**（控件实证）；nfo/
pos_state/pos_state_until 不配。

## 验证（v0.0.1270）

裸对全槽（双区影视区口径）+formdiff 同栈 0 条（双零）+DryRun 双形态
全绿：WEB-DL 纪录（404+WEB-DL+team=5 兜底）+压制 FRDS（401+Encode+
**team=5 Other/转载 前缀兜底命中**+tags=[简繁中字,英语,HDR(原盘)]
三键全中）。

## 遗留

首颗真实发布；资料区（mode=5）发布需求出现时按需评估。
