import requests
from bs4 import BeautifulSoup
import os
import re

class HDUploader:
    def __init__(self, config, mapping):
        self.base_url = config['url'].rstrip('/')
        self.cookie = config['cookie']
        self.mapping = mapping if mapping else {}
        self.headers = {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36',
            'Cookie': self.cookie,
            'Referer': f"{self.base_url}/upload.php"
        }
        self.session = requests.Session()
        self.session.headers.update(self.headers)

    def upload(self, torrent_file_path, metadata, parsed_attrs):
        """发布到 HDArea，增加深度调试输出"""
        url = f"{self.base_url}/takeupload.php"
        
        # 映射字段
        types_map = self.mapping.get('types', {})
        mediums_map = self.mapping.get('mediums', {})
        codecs_map = self.mapping.get('codecs', {})
        audio_map = self.mapping.get('audio_codecs', {})
        standards_map = self.mapping.get('standards', {})
        teams_map = self.mapping.get('teams', {})

        hda_type = types_map.get(parsed_attrs.get('type_key'), 410)
        hda_medium = mediums_map.get(parsed_attrs.get('medium'), 7)
        hda_codec = codecs_map.get(parsed_attrs.get('codec'), 7)
        hda_audiocodec = audio_map.get(parsed_attrs.get('audio'), 3)
        hda_standard = standards_map.get(parsed_attrs.get('resolution'), 1)
        hda_team = teams_map.get(parsed_attrs.get('team'), 6)
        
        # 严格对照 takeupload.php 恢复字段名的 _sel 后缀
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

        # 强力修正：强行写死英文文件名，避免 NexusPHP 无法解析非 ASCII 字符导致的表单静默失败
        # 这绝不会影响最终发布页面显示的种子标题。
        hda_torrent_name = "pt_auto_upload.torrent"
        
        # 移除 payload 中所有字符串类型字段的 4 字节 Emoji (NexusPHP MySQL utf8 错误截断缺陷)
        # 兼容性更强且更安全的方法：利用字元本身的 Unicode ord 值过滤掉超出 BMP 范围（0x10000 及以上）的字符
        for k, v in payload.items():
            if isinstance(v, str):
                payload[k] = ''.join(c for c in v if ord(c) < 0x10000)
            
        files = {
            'file': (hda_torrent_name, open(torrent_file_path, 'rb'), 'application/x-bittorrent')
        }

        try:
            # 1. 预热会话 (GET upload.php) - 解决 Cloudflare/Session 认证预检
            warmup_url = f"{self.base_url}/upload.php"
            print(f"    - 正在预热 HDA 会话 ({warmup_url})...", flush=True)
            self.session = requests.Session()
            self.session.headers.update(self.headers)
            self.session.get(warmup_url, timeout=30)

            print(f"    - 正在提交表单至 HDArea... (文件: {hda_torrent_name})", flush=True)
            response = self.session.post(url, data=payload, files=files, timeout=60, allow_redirects=False)
            
            print(f"    - HDA 响应码: {response.status_code}", flush=True)

            # 1. 检查跳转
            if response.status_code == 302:
                loc = response.headers.get('Location', '')
                if "survey-smiles.com" in loc:
                    print(f"    !!! 致命错误: HDArea Cookie 校验失败，被重定向至广告页 ({loc}) !!!")
                    print(f"    !!! 请务必检查 Web 页面中 HDA 的 Cookie 字符串是否包含完整 UID, Pass 等 !!!")
                    return None
                
                id_match = re.search(r'id=(\d+)', loc)
                if id_match:
                    hda_id = id_match.group(1)
                    print(f"    √ 发布成功! 新种子 ID: {hda_id}", flush=True)
                    return hda_id
                else:
                    print(f"    ! 302 跳转至未知位置: {loc}", flush=True)
            
            # 2. 检查页面内容 (失败通常会 200 并显示 bark 错误)
            if response.status_code == 200:
                soup = BeautifulSoup(response.text, 'lxml')
                # 尝试寻找 NexusPHP 常见的错误提示区域
                error_td = soup.find('td', class_='text')
                if error_td:
                    msg_text = error_td.get_text(strip=True)
                else:
                    error_h_or_p = soup.find('h2') or soup.find('p')
                    msg_text = error_h_or_p.get_text(strip=True) if error_h_or_p else "未发现明确错误文字"
                    
                print(f"    × 发布被拒绝 (NexusPHP 返回 200 OK): {msg_text[:200]}", flush=True)
                if "未发现明确错误文字" in msg_text:
                    print(f"    DEBUG (页面前200字): {response.text[:200]}...", flush=True)
            
            return None
        except Exception as e:
            print(f"    × 网络或系统组件异常: {e}", flush=True)
            return None
        finally:
            files['file'][1].close()

    def download_result_torrent(self, hda_id, save_path):
        dl_url = f"{self.base_url}/download.php?id={hda_id}"
        try:
            res = requests.get(dl_url, headers=self.headers, timeout=30)
            if res.status_code == 200:
                with open(save_path, 'wb') as f:
                    f.write(res.content)
                return True
        except: pass
        return False
