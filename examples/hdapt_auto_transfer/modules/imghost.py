import requests
import os

class PixHostUploader:
    def __init__(self):
        self.api_url = "https://api.pixhost.to/images"

    def upload_image(self, file_path):
        """上传单张图片到 PixHost.to 并提取直接大图链接"""
        try:
            payload = {
                'content_type': '1', # 1 for general content
                'max_th_size': '500'
            }
            with open(file_path, 'rb') as f:
                files = {'img': (os.path.basename(file_path), f)}
                res = requests.post(self.api_url, data=payload, files=files, timeout=40)
                
            if res.status_code == 200:
                data = res.json()
                show_url = data.get('show_url', '')
                th_url = data.get('th_url', '')
                
                # 算法：从 th_url 重构直链
                # 示例 th_url: https://t2.pixhost.to/thumbs/6769/709022967_..._thumb.jpg
                # 目标: https://img2.pixhost.to/images/6769/709022967_... (t->img, thumbs->images, 移除 _thumb)
                if th_url:
                    # 1. 替换路径
                    direct = th_url.replace('/thumbs/', '/images/').replace('_thumb.', '.')
                    # 2. 替换域名头部 (t2 -> img2)
                    direct = direct.replace('://t', '://img', 1)
                    print(f"    - PixHost 上传成功 (直链): {direct}")
                    return direct
                
                return show_url
            else:
                print(f"    × PixHost 上传失败: {res.status_code} - {res.text}")
                return None
        except Exception as e:
            print(f"    × PixHost 异常: {e}")
            return None

    def upload_batch_to_bbcode(self, file_paths):
        """上传多张图片并生成标准的 [img] 直接大图列表"""
        bbcodes = []
        for path in file_paths:
            direct_url = self.upload_image(path)
            if direct_url:
                # 按照用户要求，直接使用 [img]大图链接[/img]
                bbcodes.append(f"[img]{direct_url}[/img]")
        
        if bbcodes:
            return "\n".join(bbcodes)
        return ""
