import os
import subprocess
import time
from pymediainfo import MediaInfo
import requests
import re

class MetadataEngine:
    def __init__(self, config):
        self.screenshot_count = config.get('settings', {}).get('screenshot_count', 3)
        self.meta_env = config.get('metadata_api', {})
        self.imdb_to_douban_url = self.meta_env.get('imdb_to_douban')

    def _fetch_bbcode(self, url, label="DoubanInfo API"):
        """内部通用方法：发起带一次重试的 GET 请求，返回有效 BBCode 文本或空字符串"""
        max_attempts = 2
        for attempt in range(1, max_attempts + 1):
            try:
                print(f"Calling {label} (attempt {attempt}/{max_attempts}): {url}")
                response = requests.get(url, timeout=15)
                if response.status_code == 200:
                    text = response.text.strip()
                    if text and len(text) >= 50 and ('[img]' in text.lower() or '◎' in text):
                        return text
                    else:
                        print(f"  {label}: 响应内容无效，跳过 (len={len(text)})")
                        return ""  # 非超时类失败，直接放弃，无需重试
                else:
                    print(f"  {label}: HTTP {response.status_code}，跳过")
                    return ""
            except requests.exceptions.Timeout:
                print(f"  {label}: 请求超时 (attempt {attempt}/{max_attempts})")
                if attempt < max_attempts:
                    print(f"  {label}: 等待 3 秒后重试...")
                    time.sleep(3)
            except Exception as e:
                print(f"  {label}: 请求异常: {e}")
                return ""  # 非超时异常不重试
        return ""

    def get_bbcode_intro(self, imdb_id):
        """核心: 从 API 获取原始 BBCode 简介（IMDb ID）"""
        if not self.imdb_to_douban_url or not imdb_id:
            return ""
        url = self.imdb_to_douban_url.format(imdb_id=imdb_id)
        return self._fetch_bbcode(url, label=f"DoubanInfo API (IMDb:{imdb_id})")

    def get_bbcode_intro_by_douban(self, douban_id):
        """新增: 直接通过豆瓣 ID 从豆影 API 获取 BBCode 简介"""
        if not self.imdb_to_douban_url or not douban_id:
            return ""
        # 既然 API 统一使用 &url= 参数，我们直接复用配置文件里的模板
        # 完美生成 https://...&url=37814641 格式
        url = self.imdb_to_douban_url.format(imdb_id=douban_id)
        return self._fetch_bbcode(url, label=f"DoubanInfo API (Douban:{douban_id})")

    def extract_douban_id(self, bbcode_text):
        """从 BBCode 文本中提取豆瓣 ID (用于 HDA 表单)"""
        if not bbcode_text:
            return ""
        # 常见格式 [Douban ID] https://movie.douban.com/subject/30433456/
        match = re.search(r'douban\.com/subject/(\d+)', bbcode_text)
        if match:
            return match.group(1)
        return ""

    def extract_imdb_id(self, bbcode_text):
        """从 BBCode 文本中反向提取 IMDb ID"""
        if not bbcode_text:
            return ""
        # 常见格式 IMDb链接: https://www.imdb.com/title/tt14850054/ 或者是直接带标签
        match = re.search(r'(tt\d{7,10})', bbcode_text)
        if match:
            return match.group(1)
        return ""

    def get_mediainfo(self, file_path):
        try:
            media_info = MediaInfo.parse(file_path)
            return media_info.to_json()
        except:
            return ""

    def take_screenshots(self, file_path, output_dir):
        try:
            probe_cmd = f"ffprobe -v error -show_entries format=duration -of default=noprint_wrappers=1:nokey=1 \"{file_path}\""
            duration = float(subprocess.check_output(probe_cmd, shell=True))
            intervals = [duration * (i + 1) / (self.screenshot_count + 1) for i in range(self.screenshot_count)]
            screenshots = []
            for i, timestamp in enumerate(intervals):
                out_file = os.path.join(output_dir, f"screenshot_{i}.jpg")
                cmd = f"ffmpeg -y -ss {timestamp} -i \"{file_path}\" -vframes 1 -q:v 2 \"{out_file}\""
                subprocess.run(cmd, shell=True, check=True)
                screenshots.append(out_file)
            return screenshots
        except Exception as e:
            print(f"FFmpeg Error: {e}")
            return []
