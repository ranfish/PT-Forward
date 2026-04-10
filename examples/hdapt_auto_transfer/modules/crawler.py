import requests
from bs4 import BeautifulSoup
import time
import re
import os

class TTGCrawler:
    def __init__(self, config):
        self.base_url = config['url'].rstrip('/')
        # 还原 Cookie 逻辑，不进行任何过滤，防止破坏有效字段
        self.cookie = config.get('cookie', '')
        self.monitor_urls = [u.strip() for u in config.get('monitor_urls', []) if u.strip()]
        self.headers = {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36',
            'Cookie': self.cookie,
            'Accept': 'text/html,application/xhtml+xml,application/xml;'
        }

    def fetch_all_torrents(self):
        all_torrents = []
        for url in self.monitor_urls:
            print(f"Polling TTG URL: {url}", flush=True)
            if 'putrssmc' in url or 'rss' in url:
                torrents = self._fetch_rss_torrents(url)
            else:
                torrents = self._fetch_url_torrents(url)
            all_torrents.extend(torrents)
            time.sleep(2)
        return all_torrents

    def _fetch_rss_torrents(self, url):
        try:
            response = requests.get(url, headers=self.headers, timeout=15)
            if response.encoding == 'ISO-8859-1':
                response.encoding = 'utf-8'

            soup = BeautifulSoup(response.text, 'lxml-xml')
            if not soup.find('item'):
                soup = BeautifulSoup(response.text, 'lxml')

            results = []
            seen_ids = set()
            for item in soup.find_all('item'):
                title_elem = item.find('title')
                if not title_elem: continue
                full_title = title_elem.text.strip()
                
                link_elem = item.find('link')
                details_url = link_elem.text.strip() if link_elem else ""
                
                enc_elem = item.find('enclosure')
                download_url = enc_elem.get('url') if enc_elem else ""
                
                torrent_id = ""
                id_match = re.search(r'id=(\d+)', details_url)
                if id_match:
                    torrent_id = id_match.group(1)
                
                if not torrent_id or torrent_id in seen_ids: continue
                
                # 修复 TTG RSS 奇怪的 XML bug (它似乎会把 5.1 变成 5 {@} 1)
                full_title = full_title.replace('{@}', '.')
                # 修复有时候会变成 5 1 缺少点的情况
                full_title = re.sub(r'(\d)\s+(\d)-', r'\1.\2-', full_title)
                
                # 1. 尝试提取并剥离末尾的文件大小 (RSS 标题末尾通常有大小)
                # 兼容格式: "... [Subtitle 11.25 GB ]" 或 "... [Subtitle] 11.25 GB"
                size_gb = 0.0
                size_match = re.search(r'(\d+(?:\.\d+)?)\s*([KMGTP]B)\s*\]?\s*$', full_title, re.I)
                if size_match:
                    size_gb = self._parse_size_gb(size_match.group(1) + " " + size_match.group(2))
                    # 剥离大小部分
                    full_title = full_title[:size_match.start()].strip()
                
                # 2. 如果末尾还残留有 ]，也一并去掉
                full_title = full_title.rstrip(']').strip()

                # 3. 分离英文主标与中文副标
                # TTG RSS 通常格式为: English Title [Chinese Subtitle] *Extras
                hda_name = full_title
                subtitle_part = ""
                idx_start = full_title.find('[')
                if idx_start != -1:
                    hda_name = full_title[:idx_start].strip()
                    tail = full_title[idx_start+1:].strip()
                    
                    # 寻找匹配的闭合括号 ]
                    # 考虑到副标题内部可能还有小括号，我们寻找最后一个 ] 或者是第一个出现的 ]？
                    # TTG RSS 的结构通常是 English [Chinese / Alts] Extras. 
                    # 应该寻找第一个 ] 作为副标结束符。
                    idx_end = tail.find(']')
                    if idx_end != -1:
                        # 组合括号内的内容和括号外的 Extras
                        inside = tail[:idx_end].strip()
                        outside = tail[idx_end+1:].strip()
                        subtitle_part = (inside + " " + outside).strip()
                    else:
                        subtitle_part = tail
                
                category = "Unknown"
                imdb_id = ""
                douban_id = ""
                
                try:
                    res_det = requests.get(details_url, headers=self.headers, timeout=10)
                    if res_det.encoding == 'ISO-8859-1': res_det.encoding = 'utf-8'
                    det_soup = BeautifulSoup(res_det.text, 'lxml')
                    
                    type_td = det_soup.find(lambda tag: tag.name == 'td' and tag.text.strip() == '类型')
                    if type_td:
                        val_td = type_td.find_next_sibling('td')
                        if val_td: category = val_td.get_text(strip=True)
                            
                    imdb_link = det_soup.find('a', href=re.compile(r'imdb\.com/title/(tt\d+)'))
                    if imdb_link:
                        m = re.search(r'(tt\d+)', imdb_link['href'])
                        if m: imdb_id = m.group(1)

                    douban_match = re.search(r'douban\.com/subject/(\d+)', det_soup.text)
                    if douban_match:
                        douban_id = douban_match.group(1)
                except Exception as ex:
                    print(f"Crawler RSS Detail Fetch Error: {ex}")
                    
                hda_type_key = self._map_hda_type(category)
                
                results.append({
                    'id': torrent_id,
                    'title': hda_name,
                    'subtitle': subtitle_part,
                    'category': category,
                    'hda_type_key': hda_type_key,
                    'ttl_hours': 1, # fresh
                    'details_url': details_url,
                    'download_url': download_url,
                    'imdb_id': imdb_id,
                    'douban_id': douban_id,
                    'size_gb': size_gb
                })
                seen_ids.add(torrent_id)
                time.sleep(1) # Be gentle to TTG server
            
            print(f"Crawler Debug RSS: 本页记录完结，最终生成条目 {len(results)} 个", flush=True)
            return results
        except Exception as e:
            print(f"Crawler RSS Exception: {str(e)}", flush=True)
            return []


    def _fetch_url_torrents(self, url):
        try:
            response = requests.get(url, headers=self.headers, timeout=15)
            # 严格处理编码，确保中文识别
            # 严格处理编码，确保中文识别
            if response.encoding == 'ISO-8859-1':
                response.encoding = 'utf-8' # 性能优化：抛弃耗时的 apparent_encoding 预测
            
            soup = BeautifulSoup(response.text, 'lxml')
            
            # 严格限制在主种子表格内寻找，排除侧边栏、推荐位等无用链接
            torrent_table = soup.find('table', id='torrent_table')
            if not torrent_table:
                print(f"Crawler Debug: 未找到种子表格，跳过本页")
                return []
            
            detail_links = torrent_table.find_all('a', href=re.compile(r'/t/(\d+)/|details\.php\?id=(\d+)'))
            
            results = []
            seen_ids = set()
            for link in detail_links:
                href = link['href']
                # 兼容提取 ID
                id_match = re.search(r'/t/(\d+)/', href) or re.search(r'id=(\d+)', href)
                if not id_match: continue
                torrent_id = id_match.group(1) or id_match.group(2)
                
                # 关键过滤：TTG 活跃种子 ID 目前都在 500,000 以上。
                # 167398 (Anger) 等属于侧边栏热搜或历史记录，在此处排除。
                if int(torrent_id) < 500000: continue
                
                if not torrent_id or torrent_id in seen_ids: continue
                
                tr = link.find_parent('tr')
                if not tr: continue
                cols = tr.find_all('td')
                if len(cols) < 5: continue
                
                # 1. 标题识别 (处理 <b> 标签)
                b_tag = link.find('b')
                title = b_tag.get_text(strip=True) if b_tag else link.get_text(strip=True)
                
                # 尝试抓取副标题 (跟在换行符后)
                subtitle = ""
                br_tag = link.find_next_sibling('br')
                if br_tag and getattr(br_tag, 'next_sibling', None):
                    if type(br_tag.next_sibling).__name__ == 'NavigableString':
                        subtitle = str(br_tag.next_sibling).strip()
                if not subtitle and link.next_sibling and type(link.next_sibling).__name__ == 'NavigableString':
                    subtitle = str(link.next_sibling).strip()
                
                # 2. TTL 识别 (优化逻辑，防止误读)
                ttl_hours = 9999
                # 跳过前两列 (类别图标、标题列)，因为标题列常包含促销倒计时（如"剩余 19 小时"）干扰正确 TTL 取值
                for td in cols[2:]:
                    txt = td.get_text(strip=True)
                    if "小时" in txt:
                        match = re.search(r'(\d+)\s*小时', txt)
                        if match: ttl_hours = int(match.group(1)); break
                    elif "天" in txt:
                        match = re.search(r'(\d+)\s*天', txt)
                        if match: ttl_hours = int(match.group(1)) * 24; break
                    elif "分钟" in txt:
                        ttl_hours = 1; break

                # 3. 分类识别 (严格提取 alt)
                category = ""
                cat_img = cols[0].find('img')
                if cat_img:
                    category = (cat_img.get('alt') or cat_img.get('title') or "").strip()
                if not category: category = "Unknown"

                # 4. IMDb 识别
                imdb_id = ""
                imdb_match = re.search(r'imdb\.com/title/(tt\d+)', str(tr))
                if imdb_match: imdb_id = imdb_match.group(1)

                # 5. 下载链接
                dl_link = tr.find('a', class_='dl_a') or tr.find('a', href=re.compile(r'/dl/'))
                if dl_link and dl_link['href'].startswith('/'):
                    download_url = f"{self.base_url}{dl_link['href']}"
                elif dl_link:
                    download_url = f"{self.base_url}/{dl_link['href']}"
                else: download_url = f"{self.base_url}/download.php?id={torrent_id}"

                # 6. 大小识别 (倒序查找，避免受列位置变动影响)
                size_gb = 0.0
                for td in reversed(cols):
                    txt = td.get_text(strip=True).upper()
                    if 'GB' in txt or 'MB' in txt or 'TB' in txt or 'KB' in txt:
                        parsed_size = self._parse_size_gb(txt)
                        if parsed_size > 0:
                            size_gb = parsed_size
                            break

                # 7. 映射 HDA 类型
                hda_type_key = self._map_hda_type(category)

                results.append({
                    'id': torrent_id,
                    'title': title,
                    'subtitle': subtitle,
                    'category': category,
                    'hda_type_key': hda_type_key,
                    'ttl_hours': ttl_hours,
                    'details_url': f"{self.base_url}/{href.lstrip('/')}",
                    'download_url': download_url,
                    'imdb_id': imdb_id,
                    'size_gb': size_gb
                })
                seen_ids.add(torrent_id)
            
            print(f"Crawler Debug: 本页记录完结，最终生成条目 {len(results)} 个", flush=True)
            return results
        except Exception as e:
            print(f"Crawler Exception: {str(e)}", flush=True)
            return []

    def _parse_size_gb(self, size_str):
        try:
            # 常见格式: "45.23 GB", "850.5 MB", "1.2 TB"
            size_str = size_str.upper().replace('B', '').strip()
            if 'G' in size_str:
                return float(size_str.replace('G', '').strip())
            elif 'M' in size_str:
                return float(size_str.replace('M', '').strip()) / 1024
            elif 'T' in size_str:
                return float(size_str.replace('T', '').strip()) * 1024
            elif 'K' in size_str:
                return float(size_str.replace('K', '').strip()) / (1024 * 1024)
            return 0.0
        except: return 0.0

    def _map_hda_type(self, category):
        # UHD原盘 / 影视2160p -> 300 (Movie UHD-4K)
        if category in ("UHD原盘", "影视2160p"):
            return "Movie UHD-4K"
        # BluRay原盘 -> 401 (Movies Blu-ray)
        elif category == "BluRay原盘":
            return "Movies Blu-ray"
        # 电影1080i/p -> 410 (Movies 1080p)
        elif category == "电影1080i/p":
            return "Movies 1080p"
        # 电影720p -> 411 (Movies 720p)
        elif category == "电影720p":
            return "Movies 720p"
        # 电影DVDRip -> 414
        elif category == "电影DVDRip":
            return "Movies DVDRip"
        # 各类剧集 -> 402 (TV SERIES)
        elif category in ("欧美剧720p", "欧美剧1080i/p", "高清日剧", "大陆港台剧1080i/p",
                          "大陆港台剧720p", "高清韩剧", "欧美剧包", "日剧包", "韩剧包", "华语剧包"):
            return "TV SERIES"
        # 纪录片 -> 404 (Documentaries)
        elif category in ("纪录片720p", "纪录片1080i/p", "纪录片BluRay原盘"):
            return "Documentaries"
        # 音乐/MV -> 406 (Music Videos) & 408 (HQ Audio)
        elif category in ("MV&演唱会",):
            return "Music Videos"
        elif category in ("(电影原声&Game)OST", "无损音乐FLAC&APE", "补充音轨"):
            return "HQ Audio"
        # 体育节目 -> 407 (SPORTS)
        elif category == "高清体育节目":
            return "SPORTS"
        # 动漫 -> 405 (Animations)
        elif category in ("高清动漫", "动漫原盘"):
            return "Animations"
        # 综艺 -> 403 (TV SHOWS)
        elif category in ("韩国综艺", "日本综艺", "高清综艺"):
            return "TV SHOWS"
        # 移动端视频 -> 417 (Movies iPad)
        elif category == "iPhone/iPad视频":
            return "Movies iPad"
        # 其他综合 -> 409 (Misc)
        elif category == "MiniVideo":
            return "Misc"
        # 兜底：未知分类
        return "Movies 1080p"

    def parse_title_attributes(self, title):
        # 默认值兜底
        attrs = {'codec': 'x264', 'audio': 'DTS', 'resolution': '1080p', 'medium': 'Encode', 'team': 'other'}
        
        # 1. 视频编码识别
        if re.search(r'x265|HEVC', title, re.I): attrs['codec'] = 'x265'
        
        # 2. 音频识别
        if re.search(r'Atmos', title, re.I): attrs['audio'] = 'TrueHD Atmos'
        elif re.search(r'TrueHD', title, re.I): attrs['audio'] = 'TrueHD'
        elif re.search(r'DD5\.1|AC3', title, re.I): attrs['audio'] = 'AC3'
        
        # 3. 分辨率识别 (新增 1080i 和 720p)
        if re.search(r'2160p|4K', title, re.I): attrs['resolution'] = '2160p'
        elif re.search(r'1080i', title, re.I): attrs['resolution'] = '1080i'
        elif re.search(r'720p', title, re.I): attrs['resolution'] = '720p'
        
        # 4. 媒介/来源识别 (区分 Encode 和 BluRay)
        if re.search(r'HDTV', title, re.I): attrs['medium'] = 'HDTV'
        elif re.search(r'WEB-DL|WEB', title, re.I): attrs['medium'] = 'WEB-DL'
        elif re.search(r'BluRay|Blu-ray', title, re.I):
            if re.search(r'x264|x265|H264|H\.264|H\.265|HEVC', title, re.I):
                attrs['medium'] = 'Encode'
            else:
                attrs['medium'] = 'BluRay'
        elif re.search(r'REMUX', title, re.I): attrs['medium'] = 'REMUX'
        
        # 5. 制作组识别
        if re.search(r'WiKi', title, re.I): attrs['team'] = 'WiKi'
        elif re.search(r'NGB', title, re.I): attrs['team'] = 'NGB'
        elif re.search(r'ARiN', title, re.I): attrs['team'] = 'ARiN'
        elif re.search(r'TTG', title, re.I): attrs['team'] = 'TTG'
        
        return attrs

    def get_imdb_id(self, details_url):
        try:
            response = requests.get(details_url, headers=self.headers, timeout=10)
            if response.encoding == 'ISO-8859-1': response.encoding = 'utf-8'
            soup = BeautifulSoup(response.text, 'lxml')
            imdb_link = soup.find('a', href=re.compile(r'imdb\.com/title/(tt\d+)'))
            if imdb_link:
                m = re.search(r'(tt\d+)', imdb_link['href'])
                return m.group(1) if m else None
        except: pass
        return None

    def get_download_url_from_details(self, details_url):
        try:
            res = requests.get(details_url, headers=self.headers, timeout=10)
            if res.encoding == 'ISO-8859-1': res.encoding = 'utf-8'
            from bs4 import BeautifulSoup
            soup = BeautifulSoup(res.text, 'lxml')
            dl_link = soup.find('a', href=re.compile(r'/dl/\d+'))
            if dl_link:
                if dl_link['href'].startswith('/'): return f"{self.base_url}{dl_link['href']}"
                else: return f"{self.base_url}/{dl_link['href']}"
        except Exception as e:
            print(f"Extraction Error: {e}")
        return None


    def download_torrent(self, download_url, save_path):
        try:
            res = requests.get(download_url, headers=self.headers, timeout=30)
            if res.status_code == 200:
                import os
                os.makedirs(os.path.dirname(save_path), exist_ok=True)
                with open(save_path, 'wb') as f:
                    f.write(res.content)
                
                # 从 Content-Disposition 获取原始文件名
                cd = res.headers.get('Content-Disposition', '')
                # 修复: 避免贪婪匹配将 filename*= 也一起抓进去
                filename_match = re.search(r'filename="([^"]+)"', cd) or re.search(r'filename=([^;]+)', cd)
                import urllib.parse
                if filename_match:
                    original_filename = urllib.parse.unquote(filename_match.group(1))
                    # 去除多余的 [TTG] 或 [TTG] 前缀，HDA 要求干净的全英文名
                    original_filename = re.sub(r'^\[TTG\]\s*', '', original_filename, flags=re.I)
                    # 去掉中文等多余字符产生的后缀（假如有的话），因为用户说“不要把中文啥的加在上面”
                    original_filename = re.sub(r'[\u4e00-\u9fa5]+.*\.torrent$', '.torrent', original_filename)
                else:
                    original_filename = os.path.basename(save_path)
                    
                return True, original_filename
        except: pass
        return False, None
