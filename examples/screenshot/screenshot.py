#!/usr/bin/env python3

import os
import re
import sys
import json
import random
import subprocess
import glob
import mimetypes
from pathlib import Path
from urllib.request import Request, urlopen, ProxyHandler, build_opener, install_opener
from urllib.error import URLError, HTTPError
from io import BytesIO

CONFIG_FILE = "./config.conf"
OUTPUT_FILE = "./output.txt"
SCREENSHOT_COUNT = 5
MIN_INTERVAL = 30

HTTP_PROXY = "http://10.0.2.5:7897"
HTTPS_PROXY = "https://10.0.2.5:7897"

VIDEO_EXTENSIONS = {'.mp4', '.mkv', '.avi', '.mov', '.wmv', '.flv', '.webm', '.m4v', '.ts'}

TEXT_SUBTITLES = {'subrip', 'srt', 'ass', 'ssa', 'webvtt', 'vtt', 'mov_text'}
GRAPHIC_SUBTITLES = {'hdmv_pgs_subtitle', 'pgs', 'vobsub', 'dvd_subtitle', 'dvb_subtitle'}

PIXHOST_API_URL = "https://api.pixhost.to/images"


class MultiPartForm:
    def __init__(self):
        self.form_fields = []
        self.files = []
        self.boundary = '----WebKitFormBoundary' + ''.join(random.choices('abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789', k=16))
    
    def add_field(self, name, value):
        self.form_fields.append((name, value))
    
    def add_file(self, fieldname, filename, file_content, mimetype=None):
        if mimetype is None:
            mimetype = mimetypes.guess_type(filename)[0] or 'application/octet-stream'
        self.files.append((fieldname, filename, file_content, mimetype))
    
    def get_content_type(self):
        return f'multipart/form-data; boundary={self.boundary}'
    
    def get_body(self):
        body = BytesIO()
        boundary = self.boundary.encode('utf-8')
        
        for name, value in self.form_fields:
            body.write(b'--' + boundary + b'\r\n')
            body.write(f'Content-Disposition: form-data; name="{name}"\r\n\r\n'.encode('utf-8'))
            body.write(value.encode('utf-8') + b'\r\n')
        
        for fieldname, filename, content, mimetype in self.files:
            body.write(b'--' + boundary + b'\r\n')
            body.write(f'Content-Disposition: form-data; name="{fieldname}"; filename="{filename}"\r\n'.encode('utf-8'))
            body.write(f'Content-Type: {mimetype}\r\n\r\n'.encode('utf-8'))
            body.write(content + b'\r\n')
        
        body.write(b'--' + boundary + b'--\r\n')
        return body.getvalue()


def clean_old_files():
    for f in glob.glob("*.jpg"):
        try:
            os.remove(f)
        except:
            pass
    if os.path.exists(OUTPUT_FILE):
        os.remove(OUTPUT_FILE)


def parse_config(config_file):
    video_file = ""
    video_dir = ""
    
    with open(config_file, 'r', encoding='utf-8') as f:
        for line in f:
            line = line.strip()
            if line.startswith('#') or not line:
                continue
            if line.startswith('VIDEO_FILE='):
                value = line.split('=', 1)[1].strip()
                if value.startswith('"') and value.endswith('"'):
                    value = value[1:-1]
                video_file = value
            elif line.startswith('VIDEO_DIR='):
                value = line.split('=', 1)[1].strip()
                if value.startswith('"') and value.endswith('"'):
                    value = value[1:-1]
                video_dir = value
    
    return video_file, video_dir


def find_largest_video(directory):
    largest_file = None
    largest_size = 0
    
    for root, dirs, files in os.walk(directory):
        for f in files:
            ext = os.path.splitext(f)[1].lower()
            if ext in VIDEO_EXTENSIONS:
                filepath = os.path.join(root, f)
                try:
                    size = os.path.getsize(filepath)
                    if size > largest_size:
                        largest_size = size
                        largest_file = filepath
                except:
                    pass
    
    return largest_file


def get_file_size(filepath):
    try:
        size = os.path.getsize(filepath)
        for unit in ['B', 'KB', 'MB', 'GB', 'TB']:
            if size < 1024:
                return f"{size:.1f}{unit}"
            size /= 1024
        return f"{size:.1f}PB"
    except:
        return "未知"


def get_subtitle_info(filepath):
    try:
        result = subprocess.run(
            ['ffprobe', '-v', 'error', '-select_streams', 's',
             '-show_entries', 'stream=index,codec_name,disposition:stream_tags=language,title',
             '-of', 'json', filepath],
            capture_output=True, text=True, timeout=30
        )
        return json.loads(result.stdout)
    except:
        return None


def select_chinese_subtitle(subtitle_info):
    if not subtitle_info or 'streams' not in subtitle_info:
        return None, None
    
    best_ass_sid = 0
    best_srt_sid = 0
    best_pgs_sid = 0
    best_ass_score = 0
    best_srt_score = 0
    best_pgs_score = 0
    sid = 1
    
    for stream in subtitle_info['streams']:
        codec = stream.get('codec_name', '').lower()
        tags = stream.get('tags', {})
        disposition = stream.get('disposition', {})
        
        lang = tags.get('language', '').lower()
        title = tags.get('title', '').lower()
        
        comment = disposition.get('comment', 0)
        hearing = disposition.get('hearing_impaired', 0)
        visual = disposition.get('visual_impaired', 0)
        
        if comment or hearing or visual:
            sid += 1
            continue
        
        score = 0
        sub_type = None
        
        if codec in TEXT_SUBTITLES:
            sub_type = 'text'
        elif codec in GRAPHIC_SUBTITLES:
            sub_type = 'graphic'
        
        if sub_type:
            if lang in ('chi', 'zho', 'zh'):
                score += 10
            
            if any(kw in title for kw in ['简', 'chs', 'sc']):
                score += 5
            elif any(kw in title for kw in ['繁', 'cht', 'tc']):
                score += 3
            elif any(kw in title for kw in ['中', 'chinese']):
                score += 2
            
            if codec == 'ass' and score > best_ass_score:
                best_ass_score = score
                best_ass_sid = sid
            elif codec == 'subrip' and score > best_srt_score:
                best_srt_score = score
                best_srt_sid = sid
            elif sub_type == 'graphic' and score > best_pgs_score:
                best_pgs_score = score
                best_pgs_sid = sid
        
        sid += 1
    
    if best_ass_score > 0:
        return best_ass_sid, 'text'
    elif best_srt_score > 0:
        return best_srt_sid, 'text'
    elif best_pgs_score > 0:
        return best_pgs_sid, 'graphic'
    
    return None, None


def get_duration(filepath):
    try:
        result = subprocess.run(
            ['ffprobe', '-v', 'error', '-show_entries', 'format=duration',
             '-of', 'default=noprint_wrappers=1:nokey=1', filepath],
            capture_output=True, text=True, timeout=30
        )
        return int(float(result.stdout.strip()))
    except:
        return 0


def generate_time_points(duration, count, min_interval):
    golden_start = duration * 30 // 100
    golden_end = duration * 80 // 100
    
    if golden_end <= golden_start:
        golden_start = 1
        golden_end = duration - 1
    
    range_val = golden_end - golden_start
    interval = range_val // count
    
    if interval < min_interval:
        interval = min_interval
    
    points = []
    current = golden_start
    
    for _ in range(count):
        random_offset = random.randint(0, interval // 2)
        point = current + random_offset
        
        if point > golden_end:
            point = golden_end
        
        points.append(point)
        current += interval
    
    return points


def format_time(seconds):
    h = seconds // 3600
    m = (seconds % 3600) // 60
    s = seconds % 60
    return f"{h:02d}h{m:02d}m{s:02d}s"


def take_screenshot(filepath, timestamp, subtitle_sid=None):
    cmd = [
        'mpv', '--vo=image', '--ao=null', '--no-audio',
        f'--start={timestamp}', '--frames=1',
        '--no-terminal', filepath
    ]
    
    if subtitle_sid:
        cmd.extend([f'--sid={subtitle_sid}', '--sub-visibility=yes', '--blend-subtitles=yes'])
    else:
        cmd.append('--sid=no')
    
    try:
        subprocess.run(cmd, capture_output=True, timeout=60)
        if os.path.exists('00000001.jpg'):
            return True
    except:
        pass
    
    return False


def setup_proxy():
    if HTTP_PROXY or HTTPS_PROXY:
        proxy_handler = ProxyHandler({
            'http': HTTP_PROXY if HTTP_PROXY else None,
            'https': HTTPS_PROXY if HTTPS_PROXY else None
        })
        opener = build_opener(proxy_handler)
        install_opener(opener)
        if HTTP_PROXY:
            print(f"已配置 HTTP 代理: {HTTP_PROXY}")
        if HTTPS_PROXY:
            print(f"已配置 HTTPS 代理: {HTTPS_PROXY}")


def get_direct_image_url(show_url):
    try:
        request = Request(show_url)
        request.add_header('User-Agent', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36')
        response = urlopen(request, timeout=60)
        html = response.read().decode('utf-8')
        
        import re
        pattern = r'https://img\d+\.pixhost\.to/images/[^"\']+\.jpg'
        match = re.search(pattern, html)
        if match:
            return match.group(0)
    except Exception as e:
        print(f"获取直接链接失败: {e}")
    
    return None


def upload_to_pixhost(image_path):
    try:
        with open(image_path, 'rb') as f:
            image_content = f.read()
        
        form = MultiPartForm()
        form.add_field('content_type', '0')
        form.add_field('max_th_size', '420')
        form.add_file('img', os.path.basename(image_path), image_content, 'image/jpeg')
        
        request = Request(PIXHOST_API_URL)
        request.add_header('Content-Type', form.get_content_type())
        request.add_header('Accept', 'application/json')
        request.data = form.get_body()
        
        response = urlopen(request, timeout=180)
        result = json.loads(response.read().decode('utf-8'))
        
        show_url = result.get('show_url', '')
        if show_url:
            direct_url = get_direct_image_url(show_url)
            if direct_url:
                return direct_url
            else:
                print("获取直接链接失败，需要重试")
                return None
        
        return ''
    except Exception as e:
        print(f"上传失败: {e}")
    
    return None


def write_output(image_urls):
    with open(OUTPUT_FILE, 'w', encoding='utf-8') as f:
        for url in image_urls:
            f.write(f"[img]{url}[/img]\n")
    print(f"已写入 {len(image_urls)} 个图片链接到 {OUTPUT_FILE}")


def main():
    setup_proxy()
    clean_old_files()
    
    if not os.path.exists(CONFIG_FILE):
        print(f"错误: 配置文件 {CONFIG_FILE} 不存在")
        sys.exit(1)
    
    video_file, video_dir = parse_config(CONFIG_FILE)
    
    video_path = None
    
    if video_file and os.path.isfile(video_file):
        video_path = video_file
        print(f"使用指定视频文件: {video_path}")
    elif video_dir and os.path.isdir(video_dir):
        video_path = find_largest_video(video_dir)
        if not video_path:
            print(f"错误: 在目录 {video_dir} 中未找到视频文件")
            sys.exit(1)
        print(f"自动选择最大视频文件: {video_path}")
    else:
        print("错误: 请在配置文件中设置有效的 VIDEO_FILE 或 VIDEO_DIR")
        sys.exit(1)
    
    print(f"文件大小: {get_file_size(video_path)}")
    
    subtitle_sid = None
    subtitle_info = get_subtitle_info(video_path)
    
    if subtitle_info and subtitle_info.get('streams'):
        subtitle_sid, subtitle_type = select_chinese_subtitle(subtitle_info)
        if subtitle_sid:
            codec_name = "ASS" if subtitle_type == "text" else "PGS"
            print(f"检测到 {codec_name} 字幕，选择轨道: {subtitle_sid}")
        else:
            print("未检测到中文字幕")
    else:
        print("未检测到内嵌字幕")
    
    duration = get_duration(video_path)
    if duration < 5:
        print("错误: 视频时长太短")
        sys.exit(1)
    
    print(f"视频时长: {duration} 秒")
    
    time_points = generate_time_points(duration, SCREENSHOT_COUNT, MIN_INTERVAL)
    print(f"截取时间点: {' '.join(map(str, time_points))}")
    
    screenshot_files = []
    
    for i, timestamp in enumerate(time_points):
        time_str = format_time(timestamp)
        output_file = f"s{i+1}_{time_str}.jpg"
        
        print(f"正在截取第 {i+1} 张图片，时间点: {timestamp}秒 ({time_str})...")
        
        if take_screenshot(video_path, timestamp, subtitle_sid):
            os.rename('00000001.jpg', output_file)
            print(f"已保存: {output_file}")
            screenshot_files.append(output_file)
        else:
            print(f"截取失败: {output_file}")
    
    print("截图完成！")
    
    print("\n开始上传图片到 pixhost.to...")
    image_urls = []
    max_retries = 3
    
    for i, screenshot in enumerate(screenshot_files):
        print(f"正在上传第 {i+1}/{len(screenshot_files)} 张图片: {screenshot}")
        
        for attempt in range(max_retries):
            url = upload_to_pixhost(screenshot)
            if url:
                print(f"上传成功: {url}")
                image_urls.append(url)
                break
            else:
                if attempt < max_retries - 1:
                    print(f"上传失败，第 {attempt + 2} 次重试...")
                else:
                    print(f"上传失败（已重试 {max_retries} 次）: {screenshot}")
    
    if image_urls:
        write_output(image_urls)
        print(f"\n全部完成！共上传 {len(image_urls)} 张图片")
    else:
        print("\n没有图片上传成功")


if __name__ == '__main__':
    main()
