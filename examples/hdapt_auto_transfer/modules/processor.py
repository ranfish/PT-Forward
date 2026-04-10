import os
import subprocess
import re
import hashlib
import time
from pymediainfo import MediaInfo

class MediaProcessor:
    def __init__(self, temp_dir="/app/data/.pt_transfer"):
        self.temp_dir = temp_dir
        if not os.path.exists(self.temp_dir):
            os.makedirs(self.temp_dir)

    def find_main_video(self, folder_path):
        """在文件夹中寻找最大的视频文件作为主文件"""
        video_extensions = ('.mkv', '.mp4', '.ts', '.m2ts')
        
        # 0. 如果路径本身就是一个视频文件，直接返回
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
        try:
            res = subprocess.run(['mediainfo', file_path], capture_output=True, text=True)
            text = res.stdout.strip()
            return f"[quote]\n{text}\n[/quote]"
        except Exception as e:
            return f"MediaInfo 提取失败: {e}"

    def parse_media_attributes(self, file_path):
        """从 MediaInfo 提取具体的硬件级属性 (编码、分辨率、音频)"""
        attrs = {}
        try:
            mi = MediaInfo.parse(file_path)
            for track in mi.tracks:
                if track.track_type == 'Video' and 'codec' not in attrs:
                    # 分辨率判断
                    w = track.width
                    h = track.height
                    if w and h:
                        if w >= 3800 or h >= 2100:
                            attrs['resolution'] = '2160p'
                        elif w >= 1900 or h >= 1000:
                            if track.scan_type == 'Interlaced':
                                attrs['resolution'] = '1080i'
                            else:
                                attrs['resolution'] = '1080p'
                        elif w >= 1200 or h >= 700:
                            attrs['resolution'] = '720p'
                        
                    # 编码判断
                    fmt = (track.format or '').lower()
                    codec_id = (track.codec_id or '').lower()
                    if 'hevc' in fmt or 'h265' in fmt or 'x265' in fmt:
                        attrs['codec'] = 'H.265(x265/HEVC)'
                    elif 'avc' in fmt or 'h264' in fmt or 'x264' in fmt:
                        attrs['codec'] = 'H.264(x264/AVC)'
                    elif 'av1' in fmt:
                        attrs['codec'] = 'AV1'
                    elif 'vp8' in fmt or 'vp9' in fmt:
                        attrs['codec'] = 'VP8/9'
                    elif 'avs' in fmt:
                        attrs['codec'] = 'AVS'
                    elif 'mpeg-2' in fmt or 'mpeg-video' in fmt or 'mpeg2' in fmt:
                        attrs['codec'] = 'MPEG-2'
                    elif 'xvid' in fmt or 'xvid' in codec_id:
                        attrs['codec'] = 'Xvid'
                    elif 'vc-1' in fmt or 'vc1' in fmt:
                        attrs['codec'] = 'VC-1'
                    elif 'mp4' in fmt or 'mp4v' in codec_id or 'mpeg-4' in fmt:
                        attrs['codec'] = 'MPEG-4'
                    else:
                        attrs['codec'] = 'Other'
                
                elif track.track_type == 'Audio' and 'audio' not in attrs:
                    # 音频编码判断
                    fmt = (track.format or '').lower()
                    comm_name = (track.commercial_name or '').lower()
                    channels = str(getattr(track, 'channel_s', '') or getattr(track, 'channels', ''))
                    
                    if 'flac' in fmt: attrs['audio'] = 'FLAC'
                    elif 'ape' in fmt or 'monkeys' in fmt: attrs['audio'] = 'APE'
                    elif 'dts' in fmt:
                        if 'x' in comm_name: attrs['audio'] = 'DTS:X'
                        elif 'hd' in comm_name and 'ma' in comm_name: attrs['audio'] = 'DTS-HD MA/DTS XLL'
                        elif 'hd' in comm_name and ('hra' in comm_name or 'hr' in comm_name): attrs['audio'] = 'DTS-HD HR/HRA'
                        elif 'ma' in comm_name or 'master' in comm_name: attrs['audio'] = 'DTS-HD MA/DTS XLL'
                        else: attrs['audio'] = 'DTS'
                    elif 'e-ac-3' in fmt or 'eac3' in fmt:
                        if 'atmos' in comm_name: attrs['audio'] = 'DDP Atmos'
                        else: attrs['audio'] = 'DDP/E-AC-3'
                    elif 'ac-3' in fmt or 'ac3' in fmt:
                        if '2' in channels: attrs['audio'] = 'DD2.0/AC-3'
                        else: attrs['audio'] = 'DD5.1/AC-3'
                    elif 'aac' in fmt: attrs['audio'] = 'AAC'
                    elif 'truehd' in fmt or 'mlp' in fmt:  # TrueHD sometimes shows as MLP
                        if 'atmos' in comm_name: attrs['audio'] = 'TrueHD Atmos'
                        else: attrs['audio'] = 'TrueHD'
                    elif 'pcm' in fmt: attrs['audio'] = 'LPCM'
                    elif 'wav' in fmt: attrs['audio'] = 'WAV'
                    elif 'dsd' in fmt: attrs['audio'] = 'DSD'
                    elif 'mpeg' in fmt:
                        if 'h' in fmt or 'h' in comm_name: attrs['audio'] = 'MPEG-H'
                        else: attrs['audio'] = 'MPEG'
                    elif 'vorbis' in fmt: attrs['audio'] = 'Vorbis'
                    elif 'tta' in fmt: attrs['audio'] = 'TTA'
                    elif 'av3a' in fmt: attrs['audio'] = 'AV3A'
                    elif 'mp3' in fmt or 'mpeg audio' in fmt: attrs['audio'] = 'MP3'
                    elif 'alac' in fmt: attrs['audio'] = 'ALAC'
                    elif 'opus' in fmt: attrs['audio'] = 'Opus'
                    elif 'wma' in fmt: attrs['audio'] = 'WMA'
                    elif 'ac-4' in fmt or 'ac4' in fmt: attrs['audio'] = 'AC-4'
                    elif 'mqa' in fmt: attrs['audio'] = 'MQA'
                    else: attrs['audio'] = 'Other'
                            
            return attrs
        except Exception as e:
            print(f"提取 MediaInfo 属性失败: {e}")
            return {}

    def take_screenshots(self, file_path, count=4):
        """使用 FFmpeg 在视频中均匀截取 X 张图"""
        try:
            # 1. 获取视频总时长 (秒)
            res = subprocess.run([
                'ffprobe', '-v', 'error', '-show_entries', 'format=duration',
                '-of', 'default=noprint_wrappers=1:nokey=1', file_path
            ], capture_output=True, text=True)
            duration = float(res.stdout.strip())
            
            output_paths = []
            # 2. 均匀选取时间点 (跳过开头的 5% 和结尾的 5%)
            start_off = duration * 0.05
            end_off = duration * 0.95
            step = (end_off - start_off) / (count if count > 0 else 1)
            
            for i in range(count):
                ts = start_off + i * step
                # 使用 MD5 生成随机且干净的文件名，符合用户给出的 PixHost 风格
                rand_name = hashlib.md5(f"{file_path}_{i}_{time.time()}".encode()).hexdigest()[:16]
                out_name = f"{rand_name}.jpg"
                out_path = os.path.join(self.temp_dir, out_name)
                
                # 截图指令 (两段式 seek: 先 -ss 粗定位跳过大段，再 -ss 精定位强制解码)
                # 高码率/HEVC/4K 资源如果只用 input seek (-ss 在 -i 前)，
                # FFmpeg 不会完整解码上下文帧，导致截图花屏/乱码。
                # 两段式：快速跳到目标前 30s，再用 output seek 精定位 30s 内完整解码。
                pre_seek = max(0.0, ts - 30)
                fine_seek = ts - pre_seek
                subprocess.run([
                    'ffmpeg', '-y',
                    '-ss', str(pre_seek),       # 粗定位（快速跳帧，在 -i 之前）
                    '-i', file_path,
                    '-ss', str(fine_seek),      # 精定位（完整解码，在 -i 之后）
                    '-frames:v', '1',
                    '-q:v', '2',
                    '-threads', '2',            # 限制解码线程，避免大文件 OOM
                    out_path
                ], capture_output=True)
                
                if os.path.exists(out_path):
                    output_paths.append(out_path)
            
            return output_paths
        except Exception as e:
            print(f"Screenshot Error: {e}")
            return []
