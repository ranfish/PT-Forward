import requests
import time
import os
import re
from datetime import datetime, timedelta, timezone
from urllib.parse import urlparse, parse_qs

class MTeamCrawler:
    """
    M-Team v2 官方 API 对齐模式爬虫 (String 数组版)
    根据用户本地 PowerShell 测试结果，categories 等字段应为字符串数组。
    """

    # MT API 返回的 videoCodec 字段值 → 对应 HDA config 中的 codec 键名
    # 来源: https://kp.m-team.cc/browse?videoC=1&videoC=16&videoC=2&videoC=4&videoC=3&videoC=19&videoC=21&videoC=22
    MT_VIDEOCODEC_MAP = {
        '1':  'x264',   # H.264(x264/AVC)
        '16': 'x265',   # H.265(x265/HEVC)
        '2':  'VC-1',
        '4':  'MPEG-2',
        '3':  'Xvid',
        '19': 'VP8/9',
        '21': 'AV1',
        '22': 'AVS',
        '18': 'x264',   # AVC1（旧版/变体 H.264，实测出现在非标准内容中）
        # '0' 表示无视频编码（软件/音频类种子），不做映射，让标题正则兜底
    }

    # MT API 返回的 audioCodec 字段值 → 对应 HDA config 中的 audio_codecs 键名
    # 来源: https://kp.m-team.cc/browse?audioC=6&audioC=8&audioC=3&audioC=11&audioC=12&audioC=13&audioC=9&audioC=10&audioC=14&audioC=15&audioC=1&audioC=2&audioC=4&audioC=5&audioC=7
    MT_AUDIOCODEC_MAP = {
        '6':  'AAC',
        '8':  'AC3',            # AC3(DD)       → DD5.1/AC-3
        '3':  'DTS',
        '11': 'DTS-HD MA',      # DTS-HD MA     → DTS-HD MA/DTS XLL
        '12': 'DDP/E-AC-3',     # E-AC3(DDP)
        '13': 'DDP Atmos',      # E-AC3 Atoms(DDP Atmos)
        '9':  'TrueHD',
        '10': 'TrueHD Atmos',
        '14': 'LPCM',           # LPCM/PCM
        '15': 'WAV',
        '1':  'FLAC',
        '2':  'APE',
        '4':  'MP3',            # MP2/3
        '5':  'Vorbis',         # OGG
        '7':  'Other',
    }

    def __init__(self, config):
        self.base_url = config.get('url', 'https://api.m-team.cc').rstrip('/')
        self.api_key = config.get('api_key', '')
        self.monitor_urls = config.get('monitor_urls', [])
        self.free_only = config.get('free_only', True)
        self.headers = {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36',
            'x-api-key': self.api_key,
            'version': '1.1.4',
            'did': '2ea0ff386fe4439aabbe31a8c91a6010',
            'Accept': 'application/json, text/plain, */*',
            'Content-Type': 'application/json; charset=utf-8',
            'Origin': 'https://kp.m-team.cc'
        }

    def fetch_all_torrents(self):
        all_results = {}
        if not self.monitor_urls:
            return []

        final_torrents = []
        for raw_url in self.monitor_urls:
            if not raw_url.strip(): continue
            
            print(f"Polling MT URL (Local Style): {raw_url}", flush=True)
            payload = self._url_to_payload(raw_url)
            
            raw_items = self._search_api(payload, referer=raw_url)
            for item in raw_items:
                tid = str(item['id'])
                title = item.get('name', '')
                if tid in all_results: continue
                
                mapped_item = self._map_item(item)
                
                # 1. 免费过滤 (不记录原因，保持安静)
                if self.free_only:
                    discount = item.get('status', {}).get('discount', 'NONE')
                    if discount != "FREE":
                        continue
                
                # 2. TTL 过滤 (由 main.py 处理，但此处也可以记录)
                # 注：如果我们要让 main.py 里的 (TTL:1h) 更有意义，我们在 _map_item 里算好了 ttl_hours
                
                all_results[tid] = mapped_item
                final_torrents.append(mapped_item)
            
            time.sleep(2)
            
        return final_torrents

    def _url_to_payload(self, raw_url):
        """
        根据用户测试脚本，将参数作为字符串数组传递。
        """
        try:
            parsed = urlparse(raw_url)
            qs = parse_qs(parsed.query)
            
            payload = {
                "pageNumber": 1,
                "pageSize": 100,
                "mode": "normal",
                "visible": 1
            }
            
            # 参数映射 (保持复数形式，且内容为字符串数组)
            mapping = {
                'cat': 'categories',
                'medium': 'mediums',
                'source': 'sources',
                'standard': 'standards',
                'videoCodec': 'videoCodecs',
                'audioCodec': 'audioCodecs',
                'team': 'teams'
            }
            
            for url_key, api_key in mapping.items():
                vals = qs.get(url_key)
                if vals:
                    payload[api_key] = [str(v) for v in vals]
            
            kw = qs.get('keyword')
            if kw: payload['keyword'] = kw[0]
            
            return payload
        except Exception as e:
            print(f"MT Payload Error: {e}", flush=True)
            return {"pageNumber": 1, "pageSize": 100, "mode": "normal", "visible": 1}

    def _map_item(self, item):
        created_str = item.get('createdDate', '')
        ttl_hours = 0
        try:
            mt_tz = timezone(timedelta(hours=8))
            created_dt = datetime.strptime(created_str, "%Y-%m-%d %H:%M:%S").replace(tzinfo=mt_tz)
            now_beijing = datetime.now(mt_tz)
            ttl_hours = (now_beijing - created_dt).total_seconds() / 3600
        except: pass

        title = item.get('name', '')
        mt_cat = str(item.get('category', ''))
        hda_type_key = "Movies 1080p" # 默认兜底
        forced_medium = None
        
        # --- 精准分类逻辑开始 ---
        if mt_cat == '401':   # 电影/SD
            if re.search(r'DVD', title, re.I):
                hda_type_key = "Movies DVD"
                forced_medium = "DVD"
            else:
                hda_type_key = "Movies 720p"
        elif mt_cat == '419': # 电影/HD (1080p 编码)
            if re.search(r'2160p|4K', title, re.I):
                hda_type_key = "Movie UHD-4K"
            elif re.search(r'720p', title, re.I):
                hda_type_key = "Movies 720p"
            else:
                hda_type_key = "Movies 1080p"
        elif mt_cat == '420': # 電影/DVDiSo
            hda_type_key = "Movies DVD"
        elif mt_cat == '421': # 電影/Blu-Ray
            hda_type_key = "Movies Blu-ray"
            forced_medium = "BluRay"
        elif mt_cat == '439': # 電影/Remux
            hda_type_key = "Movies REMUX"
            forced_medium = "REMUX"
        elif mt_cat in ['403', '402', '438', '435']: # 各种影劇/綜藝
            hda_type_key = "TV Series"
            if mt_cat == '438': forced_medium = "BluRay"
            if mt_cat == '435': forced_medium = "DVD"
        elif mt_cat == '404': # 紀錄
            hda_type_key = "Documentaries"
        elif mt_cat == '442': # 教育影片
            hda_type_key = "Documentaries"
        elif mt_cat == '434': # Music(無損)
            hda_type_key = "HQ Audio"
        elif mt_cat == '427': # 有聲書
            hda_type_key = "HQ Audio"
        elif mt_cat == '406': # 演唱
            hda_type_key = "Music Videos"
        elif mt_cat == '405': # 動畫
            hda_type_key = "Animations"
        elif mt_cat == '407': # 運動
            hda_type_key = "Sports"
        elif mt_cat in ['423', '448', '422', '451', '409']: # PC游戏/TV游戏/软件/Misc/汽车
            hda_type_key = "Misc"
        else:
            # 基于 standard 字段的兜底逻辑 (针对未来新增未知分类)
            standard = str(item.get('standard', ''))
            if standard == '6': hda_type_key = "Movie UHD-4K"
            elif standard in ['1', '2']: hda_type_key = "Movies 1080p"
            elif standard == '3': hda_type_key = "Movies 720p"
            if not hda_type_key:
                if "2160p" in title or "4K" in title: hda_type_key = "Movie UHD-4K"
                elif "1080p" in title or "1080i" in title: hda_type_key = "Movies 1080p"
                elif "720p" in title: hda_type_key = "Movies 720p"

        # 解析详细属性（基于标题正则）
        attrs = self.parse_title_attributes(title, forced_medium=forced_medium)

        # 用 MT API 返回的字段精确覆盖标题正则结果（更可靠）
        mt_vc = str(item.get('videoCodec', '') or '')
        if mt_vc and mt_vc in self.MT_VIDEOCODEC_MAP:
            attrs['codec'] = self.MT_VIDEOCODEC_MAP[mt_vc]
            # print(f"  [MT] videoCodec={mt_vc} → codec={attrs['codec']}", flush=True)

        mt_ac = str(item.get('audioCodec', '') or '')
        if mt_ac and mt_ac in self.MT_AUDIOCODEC_MAP:
            attrs['audio'] = self.MT_AUDIOCODEC_MAP[mt_ac]
            # print(f"  [MT] audioCodec={mt_ac} → audio={attrs['audio']}", flush=True)

        return {
            'id': str(item['id']),
            'title': title,
            'category': mt_cat,
            'ttl_hours': ttl_hours,
            'size_gb': float(item.get('size', 0)) / (1024**3),
            'download_url': str(item['id']),
            'details_url': f"https://kp.m-team.cc/detail/{item['id']}",
            'hda_type_key': hda_type_key,
            'subtitle': item.get('smallDescr', ''),
            'imdb_id': self._extract_id(item.get('imdb', '')),
            'douban_id': self._extract_id(item.get('douban', '')),
            'tag': f"REPOST_MT_{item['id']}",
            'attrs': attrs # 把解析好的属性传下去，方便后面直接使用
        }

    def _extract_id(self, url):
        if not url: return ""
        match = re.search(r'/(tt\d+)/?$', url) or re.search(r'/subject/(\d+)/?$', url)
        return match.group(1) if match else ""

    def _search_api(self, payload, referer=None):
        url = f"{self.base_url}/api/torrent/search"
        headers = self.headers.copy()
        if referer: headers['Referer'] = referer
        
        try:
            response = requests.post(url, headers=headers, json=payload, timeout=20)
            if response.status_code == 200:
                data = response.json()
                if data.get('code') == '0':
                    return data.get('data', {}).get('data', [])
        except: pass
        return []

    def parse_title_attributes(self, title, forced_medium=None):
        """
        解析标题属性 (Codec, Audio, Resolution, Medium, Team) 供 HDA 接口使用
        """
        attrs = {'codec': 'x264', 'audio': 'DTS', 'resolution': '1080p', 'medium': 'Encode', 'team': 'other'}
        
        # 编码识别
        if re.search(r'x265|HEVC', title, re.I): attrs['codec'] = 'x265'
        
        # 音频识别
        if re.search(r'Atmos', title, re.I): attrs['audio'] = 'TrueHD Atmos'
        elif re.search(r'TrueHD', title, re.I): attrs['audio'] = 'TrueHD'
        elif re.search(r'DD5\.1|AC3', title, re.I): attrs['audio'] = 'AC3'
        
        # 分辨率识别
        if re.search(r'2160p|4K', title, re.I): attrs['resolution'] = '2160p'
        elif re.search(r'1080i', title, re.I): attrs['resolution'] = '1080i'
        elif re.search(r'720p', title, re.I): attrs['resolution'] = '720p'

        # 媒介识别 (核心逻辑)
        if forced_medium:
            attrs['medium'] = forced_medium
        else:
            if re.search(r'WEB-DL|WEB', title, re.I): 
                attrs['medium'] = 'WEB-DL'
            elif re.search(r'HDTV', title, re.I): 
                attrs['medium'] = 'HDTV'
            elif re.search(r'BluRay|Blu-ray', title, re.I):
                # 如果是 BluRay 标题，优先通过编码区分是原盘还是压制
                if re.search(r'x264|x265|H264|H\.264|H\.265|HEVC', title, re.I):
                    attrs['medium'] = 'Encode'
                else:
                    attrs['medium'] = 'BluRay'
            elif re.search(r'REMUX', title, re.I): 
                attrs['medium'] = 'REMUX'
        
        # 制作组识别
        if re.search(r'WiKi', title, re.I): attrs['team'] = 'WiKi'
        elif re.search(r'MTeam', title, re.I): attrs['team'] = 'MTeam'
        
        return attrs

    def download_torrent(self, torrent_id_str, save_path):
        try:
            torrent_id = int(torrent_id_str)
        except: return False, None
        token_url = f"{self.base_url}/api/torrent/genDlToken"
        token_headers = self.headers.copy()
        token_headers['Content-Type'] = 'application/x-www-form-urlencoded'
        try:
            resp = requests.post(token_url, headers=token_headers, data={'id': torrent_id}, timeout=20)
            if resp.status_code == 200:
                res_json = resp.json()
                if res_json.get('code') == '0':
                    dl_url = res_json.get('data')
                    if dl_url:
                        file_resp = requests.get(dl_url, headers=self.headers, timeout=30)
                        if file_resp.status_code == 200:
                            disp = file_resp.headers.get('Content-Disposition', '')
                            filename = f"{torrent_id}.torrent"
                            if 'filename=' in disp:
                                filename = disp.split('filename=')[-1].strip('"')
                            with open(save_path, 'wb') as f:
                                f.write(file_resp.content)
                            return True, filename
        except: pass
        return False, None
