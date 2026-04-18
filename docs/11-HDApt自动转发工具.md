# HDApt Auto Transfer 项目深度分析报告

## 一、项目概述

### 1.1 项目定位

HDApt Auto Transfer 是一款 **PT站点自动转发与发种工具**，基于 Python 与 Docker 实现。支持从 M-Team (MT) 或 TTG 等站点自动抓取种子，下载完成后自动解析视频信息、截取截图、上传图床，最终发布到 HDArea (HDA) 等目标站点。

### 1.2 核心特性

| 特性 | 说明 |
|------|------|
| **全流程自动化** | 抓取 → 下载 → 解析 → 截图 → 传图床 → 发布 → 辅种 |
| **精准 MediaInfo** | 基于 libmediainfo 和 ffmpeg，智能映射编码信息 |
| **多源站点支持** | 支持 TTG (RSS/HTML) 和 M-Team (API) |
| **Web UI 监控** | Flask 实现的轻量级管理面板 |
| **空间保护** | 自动检测磁盘容量，防止爆盘 |
| **限速保护** | 全局/单种限速，确保稳定运行 |

### 1.3 技术栈

| 组件 | 版本/说明 |
|------|----------|
| **Python** | 3.9-slim |
| **Flask** | Web UI 框架 |
| **qbittorrent-api** | qBittorrent 客户端库 |
| **BeautifulSoup4** | HTML/XML 解析 |
| **pymediainfo** | MediaInfo 解析 |
| **ffmpeg** | 视频截图 |
| **Docker** | 容器化部署 |

---

## 二、架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                    HDApt Auto Transfer 架构                     │
├─────────────────────────────────────────────────────────────────┤
│  ┌─────────────┐                                               │
│  │   Web UI    │  Flask (端口 8888)                            │
│  │  (管理面板) │  - 配置管理                                   │
│  └──────┬──────┘  - 日志查看                                   │
│         │                                                       │
│         ▼                                                       │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │                  TransferEngine (主引擎)                  │ │
│  │  run_one_cycle():                                          │ │
│  │    1. reload_config()     - 热重载配置                    │ │
│  │    2. check_disk_space()  - 磁盘空间检查                  │ │
│  │    3. scan_sources()      - 扫描源站点                    │ │
│  │    4. watch_qb_progress() - 追踪下载进度                  │ │
│  │    5. process_and_upload()- 后处理与发布                  │ │
│  │    6. check_and_cleanup() - 做种清理                      │ │
│  │    7. reannounce_seeding_torrents() - 强制汇报Tracker     │ │
│  └───────────────────────────────────────────────────────────┘ │
│         │                                                       │
│         ├────────────────┬────────────────┐                    │
│         ▼                ▼                ▼                    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │ TTGCrawler  │  │MTeamCrawler │  │  QBManager  │            │
│  │ (TTG爬虫)   │  │ (MT API)    │  │ (下载器)    │            │
│  └─────────────┘  └─────────────┘  └─────────────┘            │
│         │                │                │                    │
│         ▼                ▼                ▼                    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐            │
│  │MediaProcessor│ │MetadataEngine│ │  HDUploader │            │
│  │ (媒体处理)  │  │ (元数据)    │  │ (HDA发布)   │            │
│  └─────────────┘  └─────────────┘  └─────────────┘            │
│         │                                 │                    │
│         ▼                                 ▼                    │
│  ┌─────────────┐                   ┌─────────────┐            │
│  │PixHostUploader│                 │  HDArea    │            │
│  │ (图床上传)  │                   │  (目标站)  │            │
│  └─────────────┘                   └─────────────┘            │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 工作流程

```
┌──────────────────────────────────────────────────────────────────┐
│                        完整工作流程                              │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐      │
│  │ Phase A │───→│ Phase B │───→│ Phase C │───→│ Phase D │      │
│  │ 扫描源站 │    │ 追踪下载 │    │ 后处理  │    │ 做种清理 │      │
│  └─────────┘    └─────────┘    └─────────┘    └─────────┘      │
│       │              │              │              │            │
│       ▼              ▼              ▼              ▼            │
│  ┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐      │
│  │TTG/MT   │    │检查进度 │    │MediaInfo│    │检查做种 │      │
│  │RSS/API  │    │≥100%?   │    │截图     │    │时间/分享│      │
│  │抓取种子 │    │         │    │图床上传 │    │率/速度  │      │
│  │         │    │         │    │发布HDA  │    │自动删除 │      │
│  └─────────┘    └─────────┘    └─────────┘    └─────────┘      │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

---

## 三、核心模块分析

### 3.1 主引擎 (TransferEngine)

**文件**: `main.py`

```python
class TransferEngine:
    def __init__(self):
        self.config = yaml.safe_load(open('config.yaml', 'r', encoding='utf-8'))
        self.state = self._load_state()  # 持久化状态
        
        # 初始化各模块
        self.qb = QBManager(self.config['qbittorrent'])
        self.processor = MediaProcessor(...)
        self.imghost = PixHostUploader()
        self.meta_engine = MetadataEngine(self.config)
        self.hd = HDUploader(self.config['sites']['hdarea'], ...)
        
        self.crawlers = {
            'TTG': TTGCrawler(self.config['sites']['ttg']),
            'MT': MTeamCrawler(self.config['sites']['mteam'])
        }
    
    def run_one_cycle(self):
        # 完整工作周期
        self.reload_config()           # 热重载配置
        self.check_disk_space()        # 磁盘空间检查
        self.scan_sources()            # 扫描源站点
        self.watch_qb_progress()       # 追踪下载进度
        self.process_and_upload()      # 后处理与发布
        self.qb.check_and_cleanup()    # 做种清理
        self.qb.reannounce_seeding_torrents()  # 强制汇报Tracker
```

#### 磁盘空间保护

```python
def check_disk_space(self):
    min_space = self.config.get('settings', {}).get('min_free_space_gb', 30)
    free_gb = self.qb.get_free_space_gb(local_path)
    
    if free_gb < min_space:
        if not self.low_space:
            print_now("! 警告: 磁盘空间不足，进入下载保护模式")
            self.qb.pause_all_downloads()
            self.low_space = True
    else:
        if self.low_space:
            print_now("√ 恢复: 磁盘空间充足，退出保护模式")
            self.qb.resume_all_downloads()
            self.low_space = False
```

#### 配置热重载

```python
def reload_config(self):
    """重新读取配置文件方案并应用"""
    with open(config_path, 'r', encoding='utf-8') as f:
        new_config = yaml.safe_load(f)
        
        # 同步限速参数
        self.qb.max_global_upload_speed_mb = new_config['qbittorrent'].get('max_global_upload_speed_mb', 90)
        self.qb.max_torrent_upload_speed_mb = new_config['qbittorrent'].get('max_torrent_upload_speed_mb', 90)
        
        # 同步爬虫配置
        self.crawlers['TTG'].cookie = new_config['sites']['ttg'].get('cookie', '')
        self.crawlers['MT'].api_key = new_config['sites']['mteam'].get('api_key', '')
        
        # 重新下发限速
        self.qb.apply_limits()
```

### 3.2 TTG 爬虫 (TTGCrawler)

**文件**: `modules/crawler.py`

```python
class TTGCrawler:
    def __init__(self, config):
        self.base_url = config['url'].rstrip('/')
        self.cookie = config.get('cookie', '')
        self.monitor_urls = config.get('monitor_urls', [])
        self.headers = {
            'User-Agent': 'Mozilla/5.0 ...',
            'Cookie': self.cookie,
        }
    
    def fetch_all_torrents(self):
        all_torrents = []
        for url in self.monitor_urls:
            if 'rss' in url:
                torrents = self._fetch_rss_torrents(url)
            else:
                torrents = self._fetch_url_torrents(url)
            all_torrents.extend(torrents)
        return all_torrents
    
    def _fetch_rss_torrents(self, url):
        # 解析 RSS XML
        soup = BeautifulSoup(response.text, 'lxml-xml')
        
        for item in soup.find_all('item'):
            # 提取基本信息
            title = item.find('title').text.strip()
            link = item.find('link').text.strip()
            download_url = item.find('enclosure').get('url')
            
            # 修复 TTG RSS bug
            title = title.replace('{@}', '.')
            
            # 分离英文主标与中文副标
            hda_name, subtitle_part = self._split_title(title)
            
            # 获取详情页信息
            imdb_id, douban_id = self._fetch_details(link)
            
            results.append({
                'id': torrent_id,
                'title': hda_name,
                'subtitle': subtitle_part,
                'category': category,
                'hda_type_key': self._map_hda_type(category),
                'details_url': link,
                'download_url': download_url,
                'imdb_id': imdb_id,
                'douban_id': douban_id,
                'size_gb': size_gb
            })
        
        return results
```

### 3.3 M-Team 爬虫 (MTeamCrawler)

**文件**: `modules/mteam.py`

```python
class MTeamCrawler:
    # MT API 编码映射表
    MT_VIDEOCODEC_MAP = {
        '1':  'x264',    # H.264(x264/AVC)
        '16': 'x265',    # H.265(x265/HEVC)
        '2':  'VC-1',
        '4':  'MPEG-2',
        '3':  'Xvid',
        '19': 'VP8/9',
        '21': 'AV1',
        '22': 'AVS',
    }
    
    MT_AUDIOCODEC_MAP = {
        '6':  'AAC',
        '8':  'AC3',            # DD5.1/AC-3
        '3':  'DTS',
        '11': 'DTS-HD MA',      # DTS-HD MA/DTS XLL
        '12': 'DDP/E-AC-3',
        '13': 'DDP Atmos',
        '9':  'TrueHD',
        '10': 'TrueHD Atmos',
        '14': 'LPCM',
        '1':  'FLAC',
        '2':  'APE',
    }
    
    def __init__(self, config):
        self.base_url = config.get('url', 'https://m-team.cc')
        self.api_key = config.get('api_key', '')
        self.free_only = config.get('free_only', True)
        self.headers = {
            'x-api-key': self.api_key,
            'version': '1.1.4',
            'Content-Type': 'application/json; charset=utf-8',
        }
    
    def fetch_all_torrents(self):
        for raw_url in self.monitor_urls:
            payload = self._url_to_payload(raw_url)
            raw_items = self._search_api(payload)
            
            for item in raw_items:
                # 免费过滤
                if self.free_only:
                    discount = item.get('status', {}).get('discount', 'NONE')
                    if discount != "FREE":
                        continue
                
                mapped_item = self._map_item(item)
                final_torrents.append(mapped_item)
        
        return final_torrents
    
    def _url_to_payload(self, raw_url):
        """将 URL 参数转换为 API 请求体"""
        parsed = urlparse(raw_url)
        qs = parse_qs(parsed.query)
        
        payload = {
            "pageNumber": 1,
            "pageSize": 100,
            "mode": "normal",
            "visible": 1
        }
        
        # 参数映射
        mapping = {
            'cat': 'categories',
            'medium': 'mediums',
            'videoCodec': 'videoCodecs',
            'audioCodec': 'audioCodecs',
        }
        
        for url_key, api_key in mapping.items():
            vals = qs.get(url_key)
            if vals:
                payload[api_key] = [str(v) for v in vals]
        
        return payload
```

### 3.4 qBittorrent 管理器 (QBManager)

**文件**: `modules/client.py`

```python
class QBManager:
    def __init__(self, config):
        self.client = qbittorrentapi.Client(
            host=config['host'],
            username=config['username'],
            password=config['password']
        )
        self.max_global_upload_speed_mb = config.get('max_global_upload_speed_mb', 90)
        self.max_torrent_upload_speed_mb = config.get('max_torrent_upload_speed_mb', 90)
    
    def apply_limits(self):
        """应用全局限速设置"""
        limit_bytes = int(self.max_global_upload_speed_mb * 1024 * 1024)
        self.client.transfer_set_upload_limit(limit=limit_bytes)
        
        # 设置最大活跃数
        self.client.app_set_preferences(prefs={
            'max_active_uploads': int(self.max_active_uploads),
            'max_active_downloads': int(self.max_active_downloads),
        })
    
    def set_torrent_limit(self, torrent_hash):
        """为单个种子设置限速"""
        limit_bytes = int(self.max_torrent_upload_speed_mb * 1024 * 1024)
        self.client.torrents_set_upload_limit(limit=limit_bytes, torrent_hashes=torrent_hash)
    
    def reannounce_seeding_torrents(self):
        """强制重新向 Tracker 汇报所有做种中的种子
        
        原因: HDA Tracker 的自然汇报间隔通常为 30~60 分钟。
        在此期间新加入的下载者无法发现本节点。
        每轮循环主动重新汇报，保证 Peer 列表始终新鲜。
        """
        torrents = self.client.torrents_info()
        hashes = []
        for t in torrents:
            is_repost = t.category == 'PT_Repost' or 'REPOST_' in t.tags
            if is_repost and t.progress >= 1.0:
                hashes.append(t.hash)
        
        if hashes:
            self.client.torrents_reannounce(torrent_hashes=hashes)
    
    def get_free_space_gb(self, path):
        """获取指定路径的可用空间 (GB)"""
        usage = shutil.disk_usage(check_path)
        return usage.free / (1024**3)
    
    def pause_all_downloads(self):
        """暂停所有正在下载的任务"""
        torrents = self.client.torrents_info(status_filter='downloading')
        hashes = [t.hash for t in torrents]
        if hashes:
            self.client.torrents_pause(torrent_hashes=hashes)
    
    def resume_all_downloads(self):
        """恢复所有已暂停的下载任务"""
        torrents = self.client.torrents_info(status_filter='paused')
        hashes = [t.hash for t in torrents]
        if hashes:
            self.client.torrents_resume(torrent_hashes=hashes)
```

### 3.5 媒体处理器 (MediaProcessor)

**文件**: `modules/processor.py`

```python
class MediaProcessor:
    def find_main_video(self, folder_path):
        """在文件夹中寻找最大的视频文件作为主文件"""
        video_extensions = ('.mkv', '.mp4', '.ts', '.m2ts')
        
        # 如果路径本身就是一个视频文件，直接返回
        if os.path.isfile(folder_path) and folder_path.lower().endswith(video_extensions):
            return folder_path
        
        max_size = 0
        main_file = None
        
        for root, dirs, files in os.walk(folder_path):
            for file in files:
                if file.lower().endswith(video_extensions):
                    fp = os.path.join(root, file)
                    size = os.path.getsize(fp)
                    if size > max_size:
                        max_size = size
                        main_file = fp
        return main_file
    
    def get_full_mediainfo(self, file_path):
        """获取完整的 MediaInfo 文本 (BBCode 格式)"""
        res = subprocess.run(['mediainfo', file_path], capture_output=True, text=True)
        text = res.stdout.strip()
        return f"[quote]\n{text}\n[/quote]"
    
    def parse_media_attributes(self, file_path):
        """从 MediaInfo 提取具体的硬件级属性"""
        attrs = {}
        mi = MediaInfo.parse(file_path)
        
        for track in mi.tracks:
            if track.track_type == 'Video' and 'codec' not in attrs:
                # 分辨率判断
                w, h = track.width, track.height
                if w >= 3800 or h >= 2100:
                    attrs['resolution'] = '2160p'
                elif w >= 1900 or h >= 1000:
                    attrs['resolution'] = '1080p' if track.scan_type != 'Interlaced' else '1080i'
                elif w >= 1200 or h >= 700:
                    attrs['resolution'] = '720p'
                
                # 编码判断
                fmt = (track.format or '').lower()
                if 'hevc' in fmt or 'h265' in fmt:
                    attrs['codec'] = 'H.265(x265/HEVC)'
                elif 'avc' in fmt or 'h264' in fmt:
                    attrs['codec'] = 'H.264(x264/AVC)'
                elif 'av1' in fmt:
                    attrs['codec'] = 'AV1'
                # ...
            
            elif track.track_type == 'Audio' and 'audio' not in attrs:
                # 音频编码判断
                fmt = (track.format or '').lower()
                comm_name = (track.commercial_name or '').lower()
                
                if 'flac' in fmt: attrs['audio'] = 'FLAC'
                elif 'dts' in fmt:
                    if 'x' in comm_name: attrs['audio'] = 'DTS:X'
                    elif 'hd' in comm_name and 'ma' in comm_name: attrs['audio'] = 'DTS-HD MA/DTS XLL'
                    else: attrs['audio'] = 'DTS'
                elif 'e-ac-3' in fmt:
                    if 'atmos' in comm_name: attrs['audio'] = 'DDP Atmos'
                    else: attrs['audio'] = 'DDP/E-AC-3'
                # ...
        
        return attrs
    
    def take_screenshots(self, file_path, count=4):
        """使用 FFmpeg 在视频中均匀截取截图"""
        # 获取视频总时长
        res = subprocess.run(['ffprobe', ...], capture_output=True)
        duration = float(res.stdout.strip())
        
        # 均匀选取时间点 (跳过开头5%和结尾5%)
        start_off = duration * 0.05
        end_off = duration * 0.95
        step = (end_off - start_off) / count
        
        for i in range(count):
            timestamp = start_off + step * i
            cmd = f'ffmpeg -y -ss {timestamp} -i "{file_path}" -vframes 1 -q:v 2 "{out_file}"'
            subprocess.run(cmd, shell=True)
```

### 3.6 元数据引擎 (MetadataEngine)

**文件**: `modules/metadata.py`

```python
class MetadataEngine:
    def __init__(self, config):
        self.imdb_to_douban_url = config.get('metadata_api', {}).get('imdb_to_douban')
    
    def get_bbcode_intro(self, imdb_id):
        """从 API 获取原始 BBCode 简介（IMDb ID）"""
        if not self.imdb_to_douban_url or not imdb_id:
            return ""
        url = self.imdb_to_douban_url.format(imdb_id=imdb_id)
        return self._fetch_bbcode(url)
    
    def get_bbcode_intro_by_douban(self, douban_id):
        """通过豆瓣 ID 获取 BBCode 简介"""
        url = self.imdb_to_douban_url.format(imdb_id=douban_id)
        return self._fetch_bbcode(url)
    
    def extract_douban_id(self, bbcode_text):
        """从 BBCode 文本中提取豆瓣 ID"""
        match = re.search(r'douban\.com/subject/(\d+)', bbcode_text)
        return match.group(1) if match else ""
    
    def extract_imdb_id(self, bbcode_text):
        """从 BBCode 文本中提取 IMDb ID"""
        match = re.search(r'(tt\d{7,10})', bbcode_text)
        return match.group(1) if match else ""
```

### 3.7 图床上传器 (PixHostUploader)

**文件**: `modules/imghost.py`

```python
class PixHostUploader:
    def __init__(self):
        self.api_url = "https://api.pixhost.to/images"
    
    def upload_image(self, file_path):
        """上传单张图片到 PixHost.to 并提取直接大图链接"""
        payload = {'content_type': '1', 'max_th_size': '500'}
        
        with open(file_path, 'rb') as f:
            files = {'img': (os.path.basename(file_path), f)}
            res = requests.post(self.api_url, data=payload, files=files)
        
        if res.status_code == 200:
            data = res.json()
            th_url = data.get('th_url', '')
            
            # 重构直链
            # th_url: https://t2.pixhost.to/thumbs/6769/709022967_..._thumb.jpg
            # direct: https://img2.pixhost.to/images/6769/709022967_...
            direct = th_url.replace('/thumbs/', '/images/').replace('_thumb.', '.')
            direct = direct.replace('://t', '://img', 1)
            
            return direct
        return None
    
    def upload_batch_to_bbcode(self, file_paths):
        """上传多张图片并生成 BBCode"""
        bbcodes = []
        for path in file_paths:
            direct_url = self.upload_image(path)
            if direct_url:
                bbcodes.append(f"[img]{direct_url}[/img]")
        return "\n".join(bbcodes)
```

### 3.8 HDA 上传器 (HDUploader)

**文件**: `modules/uploader.py`

```python
class HDUploader:
    def __init__(self, config, mapping):
        self.base_url = config['url'].rstrip('/')
        self.cookie = config['cookie']
        self.mapping = mapping  # 编码映射表
        self.session = requests.Session()
    
    def upload(self, torrent_file_path, metadata, parsed_attrs):
        """发布到 HDArea"""
        url = f"{self.base_url}/takeupload.php"
        
        # 映射字段
        types_map = self.mapping.get('types', {})
        mediums_map = self.mapping.get('mediums', {})
        codecs_map = self.mapping.get('codecs', {})
        audio_map = self.mapping.get('audio_codecs', {})
        
        hda_type = types_map.get(parsed_attrs.get('type_key'), 410)
        hda_medium = mediums_map.get(parsed_attrs.get('medium'), 7)
        hda_codec = codecs_map.get(parsed_attrs.get('codec'), 7)
        hda_audiocodec = audio_map.get(parsed_attrs.get('audio'), 3)
        
        # 构建表单
        payload = {
            'name': metadata['title'],
            'small_descr': metadata.get('subtitle', ''),
            'url': metadata.get('imdb_url', ''),
            'dburl': metadata.get('douban_id', ''),
            'descr': metadata.get('description_bbcode', ''),
            'type': hda_type,
            'medium_sel': hda_medium,
            'codec_sel': hda_codec,
            'audiocodec_sel': hda_audiocodec,
            'standard_sel': hda_standard,
            'team_sel': hda_team,
            'uplver': 'yes',
        }
        
        # 移除 4 字节 Emoji (NexusPHP MySQL utf8 缺陷)
        for k, v in payload.items():
            if isinstance(v, str):
                payload[k] = ''.join(c for c in v if ord(c) < 0x10000)
        
        files = {
            'file': ('pt_auto_upload.torrent', open(torrent_file_path, 'rb'), 'application/x-bittorrent')
        }
        
        # 预热会话 (解决 Cloudflare)
        self.session.get(f"{self.base_url}/upload.php")
        
        # 提交表单
        response = self.session.post(url, data=payload, files=files, allow_redirects=False)
        
        # 解析结果
        if response.status_code == 302:
            loc = response.headers.get('Location', '')
            id_match = re.search(r'id=(\d+)', loc)
            if id_match:
                return id_match.group(1)  # 返回新种子 ID
        
        return None
```

---

## 四、配置系统

### 4.1 配置文件结构

**文件**: `config.example.yaml`

```yaml
# 分类映射 (源站分类 → HDA 分类)
category_mapping:
  UHD原盘: Movie UHD-4K
  影视2160p: Movie UHD-4K
  BluRay原盘: Movies Blu-ray
  电影1080i/p: Movies 1080p
  电影720p: Movies 720p
  欧美剧720p: TV SERIES
  纪录片1080i/p: Documentaries
  # ...

# 清理规则
cleanup_rules:
  enabled: true
  max_seed_time_hours: 48        # 最大做种时间
  min_seeders_for_deletion: 5    # 最小做种人数
  min_ratio_for_deletion: 1.1    # 最小分享率
  low_speed_threshold_kb: 20     # 低速阈值 (KB/s)
  low_speed_time_minutes: 10     # 低速持续时间

# 并发控制
concurrency:
  max_active_downloads: 5
  max_active_uploads: 40

# HDA 编码映射
hdarea_mapping:
  audio_codecs:
    FLAC: 1
    APE: 2
    DTS: 3
    DTS-HD MA/DTS XLL: 4
    DD5.1/AC-3: 5
    AAC: 6
    TrueHD: 7
    TrueHD Atmos: 10
    DTS:X: 12
    # ...
  
  codecs:
    MPEG-4: 1
    VC-1: 2
    Xvid: 3
    MPEG-2: 4
    H.265(x265/HEVC): 6
    H.264(x264/AVC): 7
    AV1: 8
    # ...
  
  mediums:
    Blu-ray: 1
    HD DVD: 2
    REMUX: 3
    MiniBD: 4
    HDTV: 5
    DVDR: 6
    Encode: 7
    WEB-DL: 9
    # ...
  
  standards:
    4K: 1
    1080p: 2
    1080i: 3
    720p: 4
    # ...
```

### 4.2 站点配置

```yaml
sites:
  ttg:
    url: "https://totheglory.im"
    cookie: "your_ttg_cookie"
    monitor_urls:
      - "https://totheglory.im/putrssmc/..."
  
  mteam:
    url: "https://m-team.cc"
    api_key: "your_api_key"
    free_only: true
    monitor_urls:
      - "https://kp.m-team.cc/browse?cat=419"
  
  hdarea:
    url: "https://hdarea.club"
    cookie: "your_hda_cookie"
```

### 4.3 qBittorrent 配置

```yaml
qbittorrent:
  host: "http://127.0.0.1:8080"
  username: "admin"
  password: "password"
  save_path: "/downloads/disk1"
  max_global_upload_speed_mb: 90
  max_torrent_upload_speed_mb: 90
  use_super_seeding: true
```

### 4.4 元数据 API 配置

```yaml
metadata_api:
  imdb_to_douban: "https://api.example.com/douban?imdb={imdb_id}"
```

---

## 五、状态管理

### 5.1 状态持久化

**文件**: `state.json`

```json
{
    "TTG:12345": {
        "site": "TTG",
        "title": "Movie.Name.2024.1080p.BluRay",
        "status": "completed",
        "source_id": "12345",
        "source_url": "https://totheglory.im/details.php?id=12345",
        "source_torrent_path": "/app/data/.pt_transfer/TTG_12345.torrent",
        "hash": "abc123def456...",
        "local_path": "/app/data/Movie.Name.2024",
        "save_path": "/downloads/disk1",
        "hda_id": "67890",
        "processed_time": 1712345678
    }
}
```

### 5.2 状态流转

```
┌───────────────┐
│ pending_space │ ← 磁盘空间不足时
└───────┬───────┘
        │ 空间恢复
        ▼
┌───────────────┐
│ downloading   │ ← 添加到 qBittorrent
└───────┬───────┘
        │ progress >= 100%
        ▼
┌───────────────┐
│ready_to_process│ ← 下载完成
└───────┬───────┘
        │ 处理完成
        ▼
┌───────────────┐
│  completed    │ ← 发布成功
└───────────────┘

┌───────────────┐
│  abandoned    │ ← 任务丢失/手动删除
└───────────────┘
```

### 5.3 状态清理

```python
def _save_state(self):
    # 仅保留最近 36 小时的记录，防止 state.json 无限增大
    cutoff = time.time() - (36 * 3600)
    cleaned_state = {
        k: v for k, v in self.state.items() 
        if v.get('status') != 'completed' or v.get('processed_time', time.time()) > cutoff
    }
    self.state = cleaned_state
    with open(self.state_file, 'w', encoding='utf-8') as f:
        json.dump(cleaned_state, f, ensure_ascii=False, indent=2)
```

---

## 六、Web UI

### 6.1 功能模块

**文件**: `web_server.py`

| 路由 | 方法 | 功能 |
|------|------|------|
| `/` | GET | 主控制面板 |
| `/login` | GET/POST | 登录认证 |
| `/logout` | GET | 退出登录 |
| `/save` | POST | 保存配置 |
| `/clear_cache` | POST | 清空状态缓存 |
| `/logs` | GET | 查看日志 |

### 6.2 认证机制

```python
def is_authenticated(config):
    password = get_web_password(config)
    if not password:
        return True
    return session.get("web_ui_authed") is True

def require_login(view):
    @wraps(view)
    def wrapped(*args, **kwargs):
        config = load_config()
        if not is_authenticated(config):
            flash("请先输入 Web UI 密码。", "error")
            return redirect(url_for("login", next=request.path))
        return view(*args, **kwargs)
    return wrapped
```

### 6.3 配置保存

```python
@app.route("/save", methods=["POST"])
@require_login
def save():
    config = load_config()
    
    # 保存站点配置
    config["sites"]["ttg"]["cookie"] = request.form.get("ttg_cookie")
    config["sites"]["hdarea"]["cookie"] = request.form.get("hda_cookie")
    config["sites"]["mteam"]["api_key"] = request.form.get("mt_api_key")
    config["sites"]["mteam"]["free_only"] = request.form.get("mt_free_only") == "on"
    
    # 保存 QB 配置
    config["qbittorrent"]["host"] = request.form.get("qb_host")
    config["qbittorrent"]["max_global_upload_speed_mb"] = float(request.form.get("qb_global_speed", 90))
    
    # 保存清理规则
    config["cleanup_rules"]["max_seed_time_hours"] = int(request.form.get("max_seed_time", 48))
    
    save_config(config)
    flash("配置已保存。", "success")
    return redirect(url_for("index"))
```

---

## 七、Docker 部署

### 7.1 Dockerfile

```dockerfile
FROM python:3.9-slim

# 安装系统级媒体依赖库
RUN apt-get update && apt-get install -y \
    libmediainfo0v5 \
    mediainfo \
    ffmpeg \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# 复制依赖并安装
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# 复制整个项目
COPY . .

# 清除 Python 字节码缓存
RUN find /app -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true

EXPOSE 8888

CMD ["python", "main.py"]
```

### 7.2 docker-compose.yml

```yaml
version: '3.8'

services:
  pt_transfer:
    build: .
    image: pt_auto_transfer:latest
    container_name: pt_transfer
    restart: unless-stopped
    ports:
      - "8888:8888"
    volumes:
      - ./config.yaml:/app/config.yaml
      - ./transfer_state.json:/app/transfer_state.json
      - /root/ardtu/pt:/app/data  # 视频文件挂载
```

### 7.3 路径映射

```yaml
settings:
  path_mapping:
    /downloads/disk1: /app/data
```

**说明**: qBittorrent 显示的下载路径 `/downloads/disk1` 映射到容器内的 `/app/data`。

---

## 八、与 PT-Forward 对比分析

### 8.1 功能对比

| 功能 | HDApt Auto Transfer | PT-Forward |
|------|---------------------|------------|
| **源站点** | TTG, M-Team | 可配置多站点 |
| **目标站点** | HDArea | 可配置多站点 |
| **种子来源** | RSS/API 爬取 | RSS 爬取 |
| **MediaInfo** | 自动解析 | 自动解析 |
| **截图上传** | PixHost 图床 | 支持多种图床 |
| **Web UI** | Flask 管理面板 | 无 |
| **Docker** | 完整支持 | 支持 |
| **状态持久化** | JSON 文件 | SQLite |
| **限速保护** | 全局/单种限速 | 无 |
| **空间保护** | 自动检测 | 无 |
| **Tracker 汇报** | 强制重新汇报 | 无 |

### 8.2 架构对比

| 维度 | HDApt Auto Transfer | PT-Forward |
|------|---------------------|------------|
| **语言** | Python | Python |
| **框架** | Flask | 无框架 |
| **运行模式** | 常驻循环 | 定时脚本 |
| **配置方式** | YAML | YAML |
| **日志系统** | 控制台输出 | 文件日志 |
| **状态存储** | JSON 文件 | SQLite |

### 8.3 集成可能性

PT-Forward 可以借鉴 HDApt Auto Transfer 的以下设计：

1. **Web UI**: Flask 管理面板实现可视化配置
2. **状态持久化**: JSON 文件存储任务状态
3. **限速保护**: 全局/单种上传限速
4. **空间保护**: 磁盘空间检测与自动暂停
5. **Tracker 汇报**: 强制重新汇报确保 Peer 新鲜
6. **M-Team API**: 借鉴 MT 编码映射表

---

## 九、关键文件索引

| 文件路径 | 说明 |
|----------|------|
| `main.py` | 主引擎，工作流程控制 |
| `modules/crawler.py` | TTG 爬虫 (RSS/HTML) |
| `modules/mteam.py` | M-Team API 爬虫 |
| `modules/client.py` | qBittorrent 管理器 |
| `modules/processor.py` | 媒体处理器 (MediaInfo/截图) |
| `modules/metadata.py` | 元数据引擎 (豆瓣/IMDb) |
| `modules/imghost.py` | PixHost 图床上传 |
| `modules/uploader.py` | HDA 上传器 |
| `web_server.py` | Flask Web UI |
| `config.example.yaml` | 配置文件模板 |
| `Dockerfile` | Docker 构建文件 |
| `docker-compose.yml` | Docker Compose 配置 |
| `requirements.txt` | Python 依赖 |
| `templates/index.html` | Web UI 主页模板 |
| `templates/login.html` | Web UI 登录模板 |

---

## 十、字段映射架构深度分析

### 10.1 四层映射流水线

```
源站原始数据 → 中间属性名（字符串） → config.yaml 映射表 → HDArea 表单数字 ID
```

| 层级 | 职责 | 代码位置 |
|------|------|----------|
| L1 源站爬虫 | 提取原始数据，产出 `attrs` dict + `hda_type_key` | `mteam.py` / `crawler.py` |
| L2 MediaProcessor | 从实际文件 MediaInfo 覆盖 codec 和 audio | `processor.py` |
| L3 config.yaml | 中间名称 → HDArea 表单 ID 的查找表 | `config.example.yaml` |
| L4 HDUploader | 最终查找 + POST 表单 | `uploader.py` |

### 10.2 分类映射（Category）

#### M-Team 分类 ID → `hda_type_key`（`mteam.py`）

| M-Team cat ID | 中文名 | → hda_type_key | 备注 |
|---------------|--------|----------------|------|
| 401 | 电影/SD | Movies 720p / Movies DVD | 标题含 DVD → DVD |
| 419 | 电影/HD | Movies 1080p / UHD-4K / 720p | 标题正则辅助 |
| 420 | 电影/DVDiSo | Movies DVD | |
| 421 | 电影/Blu-Ray | Movies Blu-ray | forced_medium=BluRay |
| 439 | 电影/Remux | Movies REMUX | forced_medium=REMUX |
| 403,402,438,435 | 影剧/综艺 | TV Series | 438→BluRay, 435→DVD |
| 404 | 纪录 | Documentaries | |
| 442 | 教育影片 | Documentaries | |
| 434 | Music(无损) | HQ Audio | |
| 427 | 有声书 | HQ Audio | |
| 406 | 演唱 | Music Videos | |
| 405 | 动画 | Animations | |
| 407 | 运动 | Sports | |

#### TTG 分类文本 → `hda_type_key`（`crawler.py`）

| TTG 分类 | → hda_type_key |
|----------|----------------|
| UHD原盘, 影视2160p | Movie UHD-4K |
| BluRay原盘 | Movies Blu-ray |
| 电影1080i/p | Movies 1080p |
| 电影720p | Movies 720p |
| 欧美剧720p/1080i/p, 大陆港台剧... | TV SERIES |
| 纪录片* | Documentaries |
| MV&演唱会 | Music Videos |
| 无损音乐FLAC&APE, OST | HQ Audio |
| 高清体育节目 | SPORTS |
| 高清动漫, 动漫原盘 | Animations |
| 综艺类 | TV SHOWS |

#### `hda_type_key` → HDArea 表单 ID（`config.yaml`）

| hda_type_key | HDA type ID |
|--------------|-------------|
| Movie UHD-4K | 300 |
| Movies Blu-ray | 401 |
| TV Series / TV SERIES | 402 |
| TV Shows / TV SHOWS | 403 |
| Documentaries | 404 |
| Animations | 405 |
| Music Videos | 406 |
| Sports / SPORTS | 407 |
| HQ Audio | 408 |
| Misc | 409 |
| Movies 1080p | 410 |
| Movies 720p | 411 |
| Movies WEB-DL | 412 |
| Movies HDTV | 413 |
| Movies DVD / DVDRip | 414 |
| Movies REMUX | 415 |
| Movies 3D | 416 |
| Movies iPad | 417 |

### 10.3 视频编码映射（VideoCodec）

#### M-Team API `videoCodec` → 内部名称（`mteam.py`）

| M-Team ID | 名称 | 内部名称 |
|-----------|------|----------|
| 1 | H.264(x264/AVC) | x264 |
| 16 | H.265(x265/HEVC) | x265 |
| 2 | VC-1 | VC-1 |
| 4 | MPEG-2 | MPEG-2 |
| 3 | Xvid | Xvid |
| 19 | VP8/9 | VP8/9 |
| 21 | AV1 | AV1 |
| 22 | AVS | AVS |

#### MediaInfo → 内部名称（`processor.py`，覆盖 L1）

| MediaInfo format | 内部名称 |
|------------------|----------|
| HEVC/H265/X265 | H.265(x265/HEVC) |
| AVC/H264/X264 | H.264(x264/AVC) |
| AV1 | AV1 |
| VP8/VP9 | VP8/9 |
| AVS | AVS |
| MPEG-2/MPEG-Video | MPEG-2 |
| Xvid | Xvid |
| VC-1/VC1 | VC-1 |
| MP4/MPEG-4 | MPEG-4 |
| 其他 | Other |

#### 内部名称 → HDArea `codec_sel` ID（`config.yaml`）

| 内部名称 | HDA ID |
|----------|--------|
| MPEG-4 | 1 |
| VC-1 | 2 |
| Xvid | 3 |
| MPEG-2 | 4 |
| Other | 5 |
| H.265(x265/HEVC) / x265 | 6 |
| H.264(x264/AVC) / x264 | 7 |
| AV1 | 8 |
| VP8/9 | 9 |
| AVS | 10 |

### 10.4 音频编码映射（AudioCodec）

#### M-Team API `audioCodec` → 内部名称（`mteam.py`）

| M-Team ID | 名称 | 内部名称 |
|-----------|------|----------|
| 6 | AAC | AAC |
| 8 | AC3(DD) | AC3 |
| 3 | DTS | DTS |
| 11 | DTS-HD MA | DTS-HD MA |
| 12 | E-AC3(DDP) | DDP/E-AC-3 |
| 13 | E-AC3 Atoms(DDP Atmos) | DDP Atmos |
| 9 | TrueHD | TrueHD |
| 10 | TrueHD Atmos | TrueHD Atmos |
| 14 | LPCM | LPCM |
| 15 | WAV | WAV |
| 1 | FLAC | FLAC |
| 2 | APE | APE |
| 4 | MP2/3 | MP3 |
| 5 | OGG | Vorbis |
| 7 | Other | Other |

#### MediaInfo → 内部名称（`processor.py`，覆盖 L1，最详细）

| MediaInfo 条件 | 内部名称 |
|----------------|----------|
| FLAC | FLAC |
| APE / Monkeys | APE |
| DTS + X/XLL | DTS:X |
| DTS + HD + MA/Master | DTS-HD MA/DTS XLL |
| DTS + HD + HRA/HR | DTS-HD HR/HRA |
| DTS (其他) | DTS |
| E-AC-3/EAC3/DDP + Atmos | DDP Atmos |
| E-AC-3/EAC3/DDP (无 Atmos) | DDP/E-AC-3 |
| AC-3/AC3 + 2ch | DD2.0/AC-3 |
| AC-3/AC3 (其他) | DD5.1/AC-3 |
| AAC | AAC |
| TrueHD/MLP + Atmos | TrueHD Atmos |
| TrueHD/MLP (无 Atmos) | TrueHD |
| PCM | LPCM |
| WAV | WAV |
| DSD | DSD |
| MPEG + H | MPEG-H |
| MPEG (其他) | MPEG |
| Vorbis | Vorbis |
| TTA | TTA |
| AV3A | AV3A |
| MP3 | MP3 |
| ALAC | ALAC |
| Opus | Opus |
| WMA | WMA |
| AC-4/AC4 | AC-4 |
| MQA | MQA |

#### 内部名称 → HDArea `audiocodec_sel` ID（`config.yaml`）

| 内部名称 | HDA ID |
|----------|--------|
| FLAC | 1 |
| APE | 2 |
| DTS | 3 |
| DTS-HD MA/DTS XLL / DTS-HD MA | 4 |
| DD5.1/AC-3 / AC3 | 5 |
| AAC | 6 |
| TrueHD | 7 |
| LPCM | 8 |
| WAV | 9 |
| TrueHD Atmos | 10 |
| DD2.0/AC-3 | 11 |
| DTS:X | 12 |
| DTS-HD HR/HRA | 13 |
| DSD | 14 |
| DDP Atmos | 15 |
| DDP/E-AC-3 | 16 |
| MPEG | 17 |
| Vorbis | 18 |
| TTA | 19 |
| AV3A | 20 |
| MP3 | 21 |
| Other | 24 |
| Opus | 25 |
| WMA | 26 |
| AC-4 | 27 |
| MPEG-H | 28 |
| MQA | 29 |

### 10.5 分辨率映射（Standard）

#### 标题正则 → 内部名称（两个爬虫共享）

```python
if re.search(r'2160p|4K', title): '2160p'
elif re.search(r'1080i', title):  '1080i'
elif re.search(r'720p', title):   '720p'
else:                             '1080p'
```

#### 内部名称 → HDArea `standard_sel` ID（`config.yaml`）

| 内部名称 | HDA ID |
|----------|--------|
| 1080p | 1 |
| 1080i | 2 |
| 720p | 3 |
| SD | 4 |
| 4K / 2160p | 5 |

### 10.6 媒介映射（Medium）

#### 标题正则 → 内部名称（两个爬虫共享）

```python
if 'HDTV' in title:            'HDTV'
elif 'WEB-DL/WEB' in title:    'WEB-DL'
elif 'BluRay' + codec in title: 'Encode'
elif 'BluRay' no codec:        'BluRay'
elif 'REMUX' in title:         'REMUX'
else:                          'Encode'
```

M-Team 额外：分类 ID 强制覆盖（421→BluRay, 439→REMUX, 401+DVD→DVD）

#### 内部名称 → HDArea `medium_sel` ID（`config.yaml`）

| 内部名称 | HDA ID |
|----------|--------|
| Blu-ray / BluRay | 1 |
| HD DVD | 2 |
| REMUX / REMUX | 3 |
| MiniBD | 4 |
| HDTV | 5 |
| DVDR / DVD | 6 |
| Encode | 7 |
| CD | 8 |
| WEB-DL | 9 |

### 10.7 制作组映射（Team）

#### 标题正则 → 内部名称

| 源站 | 匹配的制作组 |
|------|-------------|
| TTG | WiKi, NGB, ARiN, TTG |
| M-Team | WiKi, MTeam |

#### 内部名称 → HDArea `team_sel` ID（`config.yaml`）

| 内部名称 | HDA ID |
|----------|--------|
| EPiC | 1 |
| HDArea | 2 |
| HDWING | 3 |
| WiKi | 4 |
| TTG | 5 |
| other / ARiN / NGB | 6 |
| MTeam | 7 |
| HDApad | 8 |
| CHD | 9 |
| HDAccess | 10 |
| HDATV | 11 |
| cXcY | 12 |
| CMCT | 13 |

### 10.8 关键设计决策

| 决策 | 原因 |
|------|------|
| **分辨率用标题，不用 MediaInfo** | 裁剪视频像素不标准（main.py:488-490 已注释掉） |
| **编码/音频用 MediaInfo 覆盖标题** | 文件元数据比标题更准确 |
| **种子文件重命名为 ASCII `pt_auto_upload.torrent`** | NexusPHP 不支持非 ASCII 文件名 |
| **去除 4 字节 emoji** | NexusPHP MySQL utf8（非 utf8mb4）会截断 |
| **config.yaml 外置映射表** | 不改代码即可调整映射，支持别名 |
| **映射表使用别名** | 如 `x264` → 7 和 `H.264(x264/AVC)` → 7 指向同一 ID |

### 10.9 M-Team vs TTG 源站差异

| 维度 | M-Team | TTG |
|------|--------|-----|
| 数据源 | JSON API | HTML 表格 + RSS XML |
| 认证 | x-api-key header | Cookie |
| 分类输入 | 数字 ID（419, 401...） | 中文文本（电影1080i/p...） |
| 编码精度 | API videoCodec/audioCodec 字段 | 仅标题正则 |
| 媒介判断 | 分类 ID 可强制覆盖 | 仅标题正则 |
| 下载方式 | genDlToken 两步获取临时 URL | 直接 GET + Cookie |
| IMDb/豆瓣 | API 字段 imdb/douban | HTML 详情页抓取 |
| 标题处理 | API 返回干净标题 | RSS 需修复 `{@}` bug、剥离体积后缀 |

### 10.10 上传表单字段汇总

HDArea `takeupload.php` 接收的完整表单：

| 字段 | 类型 | 说明 | 映射来源 |
|------|------|------|----------|
| `file` | file | 种子文件 | 源站下载 |
| `name` | text | 标题（英文部分） | 标题拆分 |
| `small_descr` | text | 副标题（中文部分） | 标题拆分/源站 |
| `url` | text | IMDb 链接 | 源站 API/抓取 |
| `dburl` | text | 豆瓣 ID | 源站 API/抓取 |
| `descr` | textarea | 简介（BBCode） | 豆瓣简介+截图+MediaInfo |
| `type` | select | 分类 ID | hda_type_key 查表 |
| `medium_sel` | select | 媒介 ID | medium 查表 |
| `codec_sel` | select | 视频编码 ID | codec 查表 |
| `audiocodec_sel` | select | 音频编码 ID | audio 查表 |
| `standard_sel` | select | 分辨率 ID | resolution 查表 |
| `team_sel` | select | 制作组 ID | team 查表 |
| `uplver` | checkbox | 匿名发布（固定 yes） | 硬编码 |

---

## 十一、总结

### 11.1 项目优势

1. **全流程自动化**: 从抓取到发布完全无人值守
2. **精准编码映射**: MediaInfo 智能解析，低出错率
3. **多源站点支持**: TTG (RSS/HTML) + M-Team (API)
4. **完善的保护机制**: 限速、空间检测、自动清理
5. **Web UI**: 可视化配置和监控
6. **Docker 部署**: 一键构建和启动
7. **配置热重载**: 无需重启即可更新配置

### 11.2 与 PT-Forward 互补

| 场景 | 推荐方案 |
|------|----------|
| TTG/MT → HDA 自动转发 | HDApt Auto Transfer |
| 多源站 → 多目标站 | PT-Forward |
| 需要可视化配置 | HDApt Auto Transfer |
| 需要灵活扩展 | PT-Forward |

### 11.3 集成建议

PT-Forward 可以集成 HDApt Auto Transfer 的以下功能：

1. **M-Team API 支持**: 借鉴 MT 编码映射表
2. **Web UI 模块**: Flask 管理面板
3. **保护机制**: 限速和空间检测
4. **状态管理**: JSON 持久化方案
5. **Tracker 汇报**: 强制重新汇报机制
