# 13城 站点适配器设计

## 站点信息

- **站点名称**: 13城
- **站点地址**: https://13city.org
- **框架**: NexusPHP
- **Tracker URL**: https://tracker.13city.org/announce.php

---

## 一、发种规范

### 1.1 影片类资源

#### 分辨率要求
- **通用规则**: 所有作品原则上不得低于 1080P
- **例外情况**: 远古影视作品（1988 年及以前发行的影片）因技术限制，分辨率不作硬性要求

#### 容量要求
- **电影作品**: 文件大小不得小于 1GB，以确保画质和音频质量

#### 发布形式
- **已完结的影视系列作品**: 必须以整部合集形式发布，不得拆分单集上传
- **网络短剧**: 暂不接收网络短剧（没开专栏）
- **禁转内容**: 概不接收！！！

### 1.2 电子书籍类资源

#### 基本要求
- 发布电子书籍时，必须附带清晰的封面图片和详细的内容概述
- 方便其他用户了解书籍信息

#### 连载刊物
- 发布需按年份区分
- 已完结年份的内容必须打包发放，并提供封面及概述
- 正在连载的刊物，仅可发布当年更新的内容

### 1.3 有声图书及音乐专辑类资源

#### 完整性要求
- 严禁拆分发布有声图书和音乐专辑
- 需确保资源内容完整，以专辑或系列为单位进行发布

#### 配套信息
- 发布时需提供：
  - 专辑封面
  - 曲目列表（音乐专辑）
  - 有声书内容简介等必要信息

#### 年份限定
- 2024年前（含2024年份）音乐作品按年为单位打包发布

#### 网络歌手
- 因网络歌手没有完整体系并发布的歌曲数量与受众问题
- 暂不推荐发布
- 单曲类型禁止发布

### 1.4 违规处理

违反上述发种规范的资源，一经发现，将视情节轻重采取以下处罚措施：
- 警告
- 扣除分享率
- 限制发种权限
- 封禁账号

---

## 二、发布页面表单字段分析

### 2.1 基础信息字段

| 字段名 | 字段类型 | 必填 | 说明 | 示例 |
|--------|----------|------|------|------|
| `file` | file | ✓ | 种子文件 | - |
| `name` | text | - | 标题（若不填将使用种子文件名。要求规范填写） | Blade Runner 1982 Final Cut 720p HDDVD DTS x264-ESiR |
| `small_descr` | text | - | 副标题（将在种子页面种子标题下显示） | 银翼杀手 720p @ 4615 kbps - DTS 5.1 @ 1536 kbps |
| `url` | text | - | IMDb链接 | https://www.imdb.com/title/tt0468569/ |
| `pt_gen` | text | - | PT-Gen（来自imdb/douban/bangumi/indienova的链接） | - |
| `nfo` | file | - | NFO文件（不允许Power User以下用户查看） | - |
| `descr` | textarea | ✓ | 简介（支持BBCode格式） | - |
| `technical_info` | textarea | - | MediaInfo/BDInfo | - |

### 2.2 质量选择字段（类型选择后显示）

#### 类型（`type`）- 必填

| 值 | 显示名称 |
|----|----------|
| 401 | 电影/Movies |
| 402 | 剧集/TV Series |
| 403 | 综艺/TV Shows |
| 405 | 动漫/Animations |
| 406 | 演唱会、MV/Concert、Music Videos |
| 408 | 音乐/Music |
| 413 | 纪录片/Docmentaries |
| 409 | 有声读物/Audiobook |

#### 媒介（`medium_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 9 | Track |
| 8 | CD |
| 6 | DVDR |
| 5 | HDTV |
| 4 | MiniBD |
| 7 | Encode |
| 3 | Remux |
| 2 | HD DVD |
| 1 | Blu-ray |
| 10 | WEB-DL |
| 11 | BluRay |
| 12 | WEBRip |
| 13 | Other |

#### 编码（`codec_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | AVC/H.264/x264 |
| 2 | HEVC/H.265/x265 |
| 3 | MPEG-2 |
| 4 | VC-1 |
| 5 | VPB/VP9 |
| 6 | Xvid |
| 7 | Other |

#### 分辨率（`standard_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | 8K |
| 2 | 4K |
| 3 | 1080p |
| 4 | 1080i |
| 5 | Other |

#### 音频编码（`audiocodec_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | FLAC |
| 2 | APE |
| 3 | DTS-HD/DTS |
| 4 | MP3 |
| 5 | OGG |
| 6 | AAC |
| 7 | DDP/E-AC3 |
| 8 | TrueHD |
| 9 | TrueHD Atmos |
| 10 | LPCM |
| 11 | Other |
| 12 | DD/AC3 |

#### 区域（`processing_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | 中国（含港澳台） |
| 2 | 日本 |
| 3 | 泰国 |
| 4 | 印度 |
| 5 | 韩国 |
| 6 | 欧美 |
| 7 | Other（其他） |

#### 制作组（`team_sel[4]`）

| 值 | 显示名称 |
|----|----------|
| 1 | 13City |
| 2 | AilMWeb |
| 3 | 52pt |
| 4 | HHWEB |
| 5 | CHDWEB |
| 6 | FRDS |
| 7 | MTeam |
| 8 | QHstudIo |
| 9 | PTerWEB |
| 10 | rainweb |
| 11 | Other |
| 12 | SewageWeb |
| 13 | UBWEB |
| 14 | ZmWeb |
| 15 | ADWeb |
| 16 | UBits |
| 17 | WiKi |
| 18 | HDSWEB |
| 19 | QHstudIo |

### 2.3 标签字段（`tags[4][]`）- 复选框

该字段为复选框数组，具体选项需在页面上查看

### 2.4 其他字段

| 字段名 | 字段类型 | 必填 | 说明 |
|--------|----------|------|------|
| `uplver` | checkbox | - | 匿名发布 |
| `qr` | submit | ✓ | 发布按钮 |

---

## 三、Hook 实现

### 3.1 Hook 类定义

```typescript
import { SiteHook, TorrentInfo, PublishContext } from '@pt-forward/core';

export class City13Hook implements SiteHook {
  readonly siteId = 'city13';

  /**
   * 发布前验证
   */
  async beforePublish(context: PublishContext): Promise<void> {
    await this.validateResolution(context.torrentInfo);
    await this.validateFileSize(context.torrentInfo);
    await this.validateSeries(context.torrentInfo);
  }

  /**
   * 验证分辨率要求
   */
  private async validateResolution(torrentInfo: TorrentInfo): Promise<void> {
    const { videoInfo, releaseYear } = torrentInfo;

    // 远古影视作品（1988年及以前）不做分辨率要求
    if (releaseYear && releaseYear <= 1988) {
      return;
    }

    // 验证分辨率不得低于 1080p
    if (videoInfo.resolution) {
      const height = this.parseResolutionHeight(videoInfo.resolution);
      if (height && height < 1080) {
        throw new Error(`13City 要求分辨率不低于 1080p，当前: ${videoInfo.resolution}`);
      }
    }
  }

  /**
   * 验证文件大小要求
   */
  private async validateFileSize(torrentInfo: TorrentInfo): Promise<void> {
    const { type, totalSize } = torrentInfo;

    // 电影作品要求文件大小不得小于 1GB
    if (type === 'movie' && totalSize) {
      const oneGB = 1024 * 1024 * 1024; // 1GB in bytes
      if (totalSize < oneGB) {
        throw new Error(`13City 要求电影文件大小不得小于 1GB，当前: ${(totalSize / oneGB).toFixed(2)}GB`);
      }
    }
  }

  /**
   * 验证系列作品发布形式
   */
  private async validateSeries(torrentInfo: TorrentInfo): Promise<void> {
    const { type, seriesInfo } = torrentInfo;

    // 已完结的影视系列作品，必须以整部合集形式发布
    if (type === 'tv' && seriesInfo) {
      if (seriesInfo.status === 'completed' && seriesInfo.isSingleEpisode) {
        throw new Error('13City 要求已完结的影视系列作品必须以整部合集形式发布，不得拆分单集上传');
      }
    }
  }

  /**
   * 解析分辨率高度
   */
  private parseResolutionHeight(resolution: string): number | null {
    const match = resolution.match(/(\d{3,4})[piP]/);
    return match ? parseInt(match[1], 10) : null;
  }

  /**
   * 发布后处理
   */
  async afterPublish(context: PublishContext, torrentId: string): Promise<void> {
    // 13City 暂无特殊后处理逻辑
  }
}
```

### 3.2 辅助函数

```typescript
/**
 * 检查是否为远古影视作品
 */
function isAncientFilm(releaseYear?: number): boolean {
  if (!releaseYear) return false;
  return releaseYear <= 1988;
}

/**
 * 验证电子书籍信息
 */
function validateEbookInfo(torrentInfo: TorrentInfo): void {
  if (torrentInfo.type === 'ebook') {
    if (!torrentInfo.coverImage) {
      console.warn('13City 建议电子书籍附带清晰的封面图片');
    }
    if (!torrentInfo.description) {
      console.warn('13City 建议电子书籍提供详细的内容概述');
    }
  }
}

/**
 * 验证音乐专辑完整性
 */
function validateAlbumIntegrity(torrentInfo: TorrentInfo): void {
  if (torrentInfo.type === 'music' || torrentInfo.type === 'audiobook') {
    if (torrentInfo.albumInfo) {
      if (!torrentInfo.albumInfo.cover) {
        console.warn('13City 建议提供专辑封面');
      }
      if (!torrentInfo.albumInfo.tracklist) {
        console.warn('13City 建议提供曲目列表或内容简介');
      }
    }
  }
}
```

---

## 四、配置示例

### 4.1 类型映射

```typescript
export const city13TypeMapping = {
  movie: 401,
  tv: 402,
  variety: 403,
  anime: 405,
  concert: 406,
  mv: 406,
  music: 408,
  documentary: 413,
  audiobook: 409,
  ebook: null, // 13City 可能不支持电子书分类
};
```

### 4.2 媒介映射

```typescript
export const city13MediumMapping = {
  track: 9,
  cd: 8,
  dvdr: 6,
  hdtv: 5,
  minibd: 4,
  encode: 7,
  remux: 3,
  hddvd: 2,
  bluray: 1,
  webdl: 10,
  bluray_disc: 11,
  webr: 12,
  other: 13,
};
```

### 4.3 编码映射

```typescript
export const city13CodecMapping = {
  avc: 1,
  h264: 1,
  x264: 1,
  hevc: 2,
  h265: 2,
  x265: 2,
  mpeg2: 3,
  'mpeg-2': 3,
  vc1: 4,
  'vc-1': 4,
  vp9: 5,
  vp8: 5,
  xvid: 6,
  other: 7,
};
```

### 4.4 分辨率映射

```typescript
export const city13ResolutionMapping = {
  '8k': 1,
  '4k': 2,
  '2160p': 2,
  '1080p': 3,
  '1080i': 4,
  '720p': 5,
  '480p': 5,
  other: 5,
};
```

### 4.5 音频编码映射

```typescript
export const city13AudioCodecMapping = {
  flac: 1,
  ape: 2,
  dts: 3,
  'dts-hd': 3,
  mp3: 4,
  ogg: 5,
  aac: 6,
  eac3: 7,
  'dd+': 7,
  truehd: 8,
  atmos: 9,
  'truehd.atmos': 9,
  lpcm: 10,
  'pcm': 10,
  ac3: 12,
  dd: 12,
  other: 11,
};
```

### 4.6 区域映射

```typescript
export const city13RegionMapping = {
  china: 1,
  cn: 1,
  hk: 1,
  tw: 1,
  japan: 2,
  jp: 2,
  thailand: 3,
  th: 3,
  india: 4,
  in: 4,
  korea: 5,
  kr: 5,
  usa: 6,
  uk: 6,
  europe: 6,
  other: 7,
};
```

### 4.7 制作组映射

```typescript
export const city13TeamMapping = {
  '13city': 1,
  'ailmweb': 2,
  '52pt': 3,
  'hhweb': 4,
  'chdweb': 5,
  'frds': 6,
  'mteam': 7,
  'qhstudio': 8,
  'pterweb': 9,
  'rainweb': 10,
  'other': 11,
  'sewageweb': 12,
  'ubweb': 13,
  'zmweb': 14,
  'adweb': 15,
  'ubits': 16,
  'wiki': 17,
  'hdsweb': 18,
};
```

---

## 五、测试用例

### 5.1 分辨率验证测试

```typescript
describe('City13Hook - 分辨率验证', () => {
  const hook = new City13Hook();

  test('1080p 应该通过验证', async () => {
    const torrentInfo: TorrentInfo = {
      type: 'movie',
      videoInfo: { resolution: '1920x1080' },
      releaseYear: 2020,
    };
    const context = { torrentInfo } as PublishContext;

    await expect(hook.beforePublish(context)).resolves.not.toThrow();
  });

  test('720p 应该拒绝（非远古作品）', async () => {
    const torrentInfo: TorrentInfo = {
      type: 'movie',
      videoInfo: { resolution: '1280x720' },
      releaseYear: 2020,
    };
    const context = { torrentInfo } as PublishContext;

    await expect(hook.beforePublish(context)).rejects.toThrow(
      '13City 要求分辨率不低于 1080p'
    );
  });

  test('720p 应该通过（远古作品）', async () => {
    const torrentInfo: TorrentInfo = {
      type: 'movie',
      videoInfo: { resolution: '720p' },
      releaseYear: 1985,
    };
    const context = { torrentInfo } as PublishContext;

    await expect(hook.beforePublish(context)).resolves.not.toThrow();
  });
});
```

### 5.2 文件大小验证测试

```typescript
describe('City13Hook - 文件大小验证', () => {
  const hook = new City13Hook();

  test('电影文件大于1GB 应该通过', async () => {
    const torrentInfo: TorrentInfo = {
      type: 'movie',
      totalSize: 2 * 1024 * 1024 * 1024, // 2GB
    };
    const context = { torrentInfo } as PublishContext;

    await expect(hook.beforePublish(context)).resolves.not.toThrow();
  });

  test('电影文件小于1GB 应该拒绝', async () => {
    const torrentInfo: TorrentInfo = {
      type: 'movie',
      totalSize: 500 * 1024 * 1024, // 500MB
    };
    const context = { torrentInfo } as PublishContext;

    await expect(hook.beforePublish(context)).rejects.toThrow(
      '13City 要求电影文件大小不得小于 1GB'
    );
  });
});
```

### 5.3 系列作品验证测试

```typescript
describe('City13Hook - 系列作品验证', () => {
  const hook = new City13Hook();

  test('已完结剧集单集发布应该拒绝', async () => {
    const torrentInfo: TorrentInfo = {
      type: 'tv',
      seriesInfo: {
        status: 'completed',
        isSingleEpisode: true,
      },
    };
    const context = { torrentInfo } as PublishContext;

    await expect(hook.beforePublish(context)).rejects.toThrow(
      '13City 要求已完结的影视系列作品必须以整部合集形式发布'
    );
  });

  test('已完结剧集合集发布应该通过', async () => {
    const torrentInfo: TorrentInfo = {
      type: 'tv',
      seriesInfo: {
        status: 'completed',
        isSingleEpisode: false,
      },
    };
    const context = { torrentInfo } as PublishContext;

    await expect(hook.beforePublish(context)).resolves.not.toThrow();
  });
});
```

---

## 六、注意事项

1. **分辨率验证**: 13城对分辨率有严格要求，必须 ≥1080p（1988年及以前作品除外）
2. **文件大小**: 电影作品必须 ≥1GB
3. **系列作品**: 已完结作品必须以合集形式发布，不得拆分单集
4. **禁转内容**: 13城不接收禁转内容，必须确保资源可以转存
5. **音乐专辑**: 必须以专辑或系列为单位发布，严禁拆分
6. **电子书籍**: 必须附带封面图片和内容概述
7. **连载刊物**: 需按年份区分发布
8. **网络歌手**: 单曲类型禁止发布

---

## 七、参考资料

- 发种规范: https://13city.org/forums.php?action=viewtopic&forumid=1&topicid=13
- 发布页面: https://13city.org/upload.php
- NexusPHP 框架文档
