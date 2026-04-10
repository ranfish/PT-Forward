import os
import yaml
import time
import re
import threading
import requests
import json
from datetime import datetime, timezone, timedelta
from modules.crawler import TTGCrawler
from modules.mteam import MTeamCrawler
from modules.client import QBManager
from modules.processor import MediaProcessor
from modules.imghost import PixHostUploader
from modules.metadata import MetadataEngine
from modules.uploader import HDUploader

def print_now(msg):
    # 强制打印到控制台，避免 Docker 缓冲
    print(f"[{datetime.now().strftime('%H:%M:%S')}] {msg}", flush=True)

class TransferEngine:
    def __init__(self):
        # 允许通过环境变量或默认路径选择配置文件
        config_path = os.environ.get('CONFIG_PATH', 'config.yaml')
        self.config = yaml.safe_load(open(config_path, 'r', encoding='utf-8'))
        
        # 状态储存路径 (持久化)
        self.state_file = os.path.join(
            self.config['settings'].get('path_mapping', {}).get('/downloads/disk1', '/app/data'), 
            'state.json'
        )
        if not os.path.exists(os.path.dirname(self.state_file)):
            # 兼容非 Docker 环境
            self.state_file = 'state.json'
            
        self.state = self._load_state()
        
        # 初始化各个模块
        self.qb = QBManager(self.config['qbittorrent'])
        # QBManager 接收的是 qbittorrent 子块，concurrency 的参数需要手动注入
        self.qb.max_active_uploads = self.config.get('concurrency', {}).get('max_active_uploads', 100)
        self.qb.max_active_downloads = self.config.get('concurrency', {}).get('max_active_downloads', 5)
        self.processor = MediaProcessor(self.config['settings'].get('temp_dir', '/app/data/.pt_transfer'))
        self.imghost = PixHostUploader()
        self.meta_engine = MetadataEngine(self.config)
        self.hd = HDUploader(self.config['sites']['hdarea'], self.config.get('hdarea_mapping', {}))
        
        self.crawlers = {
            'TTG': TTGCrawler(self.config['sites']['ttg']),
            'MT': MTeamCrawler(self.config['sites']['mteam'])
        }
        
        # 初始应用限速
        self.qb.apply_limits()
        self.low_space = False

    def _task_key(self, site_name, torrent_id):
        return f"{site_name}:{torrent_id}"

    def _task_temp_name(self, site_name, torrent_id, suffix=".torrent"):
        safe_key = self._task_key(site_name, torrent_id).replace(":", "_")
        return f"{safe_key}{suffix}"

    def _normalize_state(self, raw_state):
        normalized = {}
        for raw_key, data in raw_state.items():
            if not isinstance(data, dict):
                continue

            site_name = data.get('site')
            source_id = data.get('source_id') or raw_key
            task_key = raw_key
            if ':' not in str(raw_key) and site_name and source_id:
                task_key = self._task_key(site_name, source_id)

            if 'source_details_url' not in data and data.get('source_url'):
                data['source_details_url'] = data['source_url']
            if 'source_download_url' not in data and data.get('download_url'):
                data['source_download_url'] = data['download_url']

            normalized[task_key] = data
        return normalized

    def _load_state(self):
        if os.path.exists(self.state_file):
            try:
                with open(self.state_file, 'r', encoding='utf-8') as f:
                    data = self._normalize_state(json.load(f))
                    print_now(f"√ 已加载持久化映射记录: {len(data)} 条")
                    return data
            except Exception as e:
                print_now(f"! 加载状态文件失败: {e}")
        return {}

    def _save_state(self):
        try:
            # 仅保留最近 24 小时的记录，防止 state.json 无限增大
            cutoff = time.time() - (36 * 3600)
            cleaned_state = {
                k: v for k, v in self.state.items() 
                if v.get('status') != 'completed' or v.get('processed_time', time.time()) > cutoff
            }
            self.state = cleaned_state
            with open(self.state_file, 'w', encoding='utf-8') as f:
                json.dump(cleaned_state, f, ensure_ascii=False, indent=2)
        except Exception as e:
            print_now(f"! 保存状态文件失败: {e}")

    def reload_config(self):
        """重新读取配置文件方案并应用"""
        try:
            config_path = os.environ.get('CONFIG_PATH', 'config.yaml')
            with open(config_path, 'r', encoding='utf-8') as f:
                new_config = yaml.safe_load(f)

                # 检测限速值是否实际发生了变化（避免每轮都打断正在进行的 TCP 上传流）
                old_torrent_limit = self.qb.max_torrent_upload_speed_mb

                self.config = new_config

                # 同步关键限速参数到 QB 管理器
                self.qb.max_global_upload_speed_mb = new_config['qbittorrent'].get('max_global_upload_speed_mb', 90)
                self.qb.max_torrent_upload_speed_mb = new_config['qbittorrent'].get('max_torrent_upload_speed_mb', 90)
                self.qb.use_super_seeding = new_config['qbittorrent'].get('use_super_seeding', False)
                self.qb.max_active_uploads = new_config.get('concurrency', {}).get('max_active_uploads', 100)
                self.qb.max_active_downloads = new_config.get('concurrency', {}).get('max_active_downloads', 5)

                # 同步爬虫链接和 Cookie
                if 'TTG' in self.crawlers and 'ttg' in new_config.get('sites', {}):
                    self.crawlers['TTG'].monitor_urls = [u.strip() for u in new_config['sites']['ttg'].get('monitor_urls', []) if u.strip()]
                    self.crawlers['TTG'].cookie = new_config['sites']['ttg'].get('cookie', '')
                    self.crawlers['TTG'].headers['Cookie'] = self.crawlers['TTG'].cookie

                if 'MT' in self.crawlers and 'mteam' in new_config.get('sites', {}):
                    self.crawlers['MT'].monitor_urls = [u.strip() for u in new_config['sites']['mteam'].get('monitor_urls', []) if u.strip()]
                    self.crawlers['MT'].api_key = new_config['sites']['mteam'].get('api_key', '')
                    self.crawlers['MT'].free_only = new_config['sites']['mteam'].get('free_only', True)
                    self.crawlers['MT'].headers['x-api-key'] = self.crawlers['MT'].api_key

                # 重新下发全局限速
                self.qb.apply_limits()

                # Bug2修复: 先登录再获取种子列表，防止 Session 过期导致崩溃
                self.qb.login()

                # 只在单种限速值实际变更时才遍历重设，避免每 120s 打断正在进行的 TCP 上传流
                torrent_limit_changed = (old_torrent_limit != self.qb.max_torrent_upload_speed_mb)
                if torrent_limit_changed:
                    print_now(f"QB: 单种限速变更 ({old_torrent_limit} → {self.qb.max_torrent_upload_speed_mb} MB/s)，正在逐一应用...")
                    torrents = self.qb.client.torrents_info()
                    for t in torrents:
                        if getattr(t, 'category', '') == 'PT_Repost' or any(tg.startswith('REPOST_') for tg in getattr(t, 'tags', '').split(',')):
                            self.qb.set_torrent_limit(t.hash)
        except Exception as e:
            print_now(f"! 配置文件重载失败: {e}")

    def check_disk_space(self):
        min_space = self.config.get('settings', {}).get('min_free_space_gb', 30)
        # 探测实际物理路径
        path_mapping = self.config['settings'].get('path_mapping', {})
        # 默认检查 save_path 对应的物理路径
        remote_path = self.config['qbittorrent'].get('save_path', '/downloads/disk1')
        local_path = remote_path
        for r, l in path_mapping.items():
            if remote_path.startswith(r):
                local_path = remote_path.replace(r, l, 1)
                break
        
        free_gb = self.qb.get_free_space_gb(local_path)
        print_now(f"--- 磁盘空间检查: {free_gb:.1f} GB 可用 (阈值: {min_space} GB) ---")
        
        if free_gb < min_space:
            if not self.low_space:
                print_now("! 警告: 磁盘空间不足，进入下载保护模式 (暂停所有下载)")
                self.qb.pause_all_downloads()
                self.low_space = True
        else:
            if self.low_space:
                print_now("√ 恢复: 磁盘空间充足，退出保护模式")
                self.qb.resume_all_downloads()
                self.low_space = False
                # 尝试处理等待中的任务
                self.handle_pending_space_tasks()

    def handle_pending_space_tasks(self):
        pending_tasks = {task_key: data for task_key, data in self.state.items() if data.get('status') == 'pending_space'}
        if not pending_tasks: return
        
        print_now(f"▶ 正在恢复处理 {len(pending_tasks)} 个等待空间的任务...")
        for task_key, data in pending_tasks.items():
            t_path = data.get('source_torrent_path')
            if t_path and os.path.exists(t_path):
                added, info = self.qb.add_torrent(t_path, title=data['title'], tag=data['tag'])
                if added:
                    print_now(f"  √ 已重新推送下载: {data['title']}")
                    self.state[task_key]['status'] = 'downloading'
                    if info: self.state[task_key]['hash'] = info.get('hash', '')
                    self._save_state()

    def run_one_cycle(self):
        # 检查是否收到清空缓存的指令 (来自 Web UI)
        if os.path.exists(".clear_cache_flag"):
            try:
                os.remove(".clear_cache_flag")
                self.state = {}
                self._save_state()
                print_now("!!! 已清空所有缓存状态，系统将开始重新抓取历史资源 (如果需要) !!!")
            except Exception as e:
                print_now(f"! 清空缓存失败: {e}")

        # 0. 热重载配置与应用限速
        self.reload_config()

        # 0.5 磁盘空间检查
        self.check_disk_space()
        
        # 1. 各站扫描 (Phase A)
        self.scan_sources()
        
        # 2. 追踪下载进度 (Phase B)
        self.watch_qb_progress()
        
        # 3. 后处理与发布 (Phase C)
        self.process_and_upload()
        
        # 4. 做种及清理 (Phase D)
        print_now("--- 阶段 D: 做种清理 ---")
        cleanup_conf = dict(self.config.get('cleanup_rules', {}))
        cleanup_conf['check_interval'] = self.config['settings'].get('check_interval', 120)
        self.qb.check_and_cleanup(cleanup_conf)

        # 5. 强制重新汇报 Tracker (Phase E)
        # HDA Tracker 汇报间隔通常 30~60 分钟，每 2 分钟主动汇报确保 Peer 列表新鲜
        self.qb.reannounce_seeding_torrents()

    def scan_sources(self):
        max_ttl = self.config['settings'].get('max_ttl_hours', 1)
        temp_dir = self.config['settings'].get('temp_dir', '/app/data/.pt_transfer')
        os.makedirs(temp_dir, exist_ok=True)

        for site_name, crawler in self.crawlers.items():
            print_now(f"--- 阶段 A: {site_name} 扫描 (TTL:{max_ttl}h) ---")
            try:
                torrents = crawler.fetch_all_torrents()
                max_size_gb = self.config['settings'].get('max_torrent_size_gb', 0)
                
                for t in torrents:
                    tid = t['id']
                    # 避免重复下载
                    task_key = self._task_key(site_name, tid)
                    if task_key in self.state: continue
                    
                    # 时间过滤 (优先屏蔽旧资源，避免干扰日志)
                    if t['ttl_hours'] > max_ttl:
                        continue
                        
                    # 体积过滤
                    t_size = t.get('size_gb', 0)
                    if max_size_gb > 0 and t_size > max_size_gb:
                        print_now(f"  ! 跳过大体积种子 ({t_size:.2f}GB > {max_size_gb}GB): {t['title']}")
                        continue

                    if True:
                        # ...之前的逻辑...
                        t_path = os.path.join(temp_dir, self._task_temp_name(site_name, tid))
                        success, original_filename = crawler.download_torrent(t['download_url'], t_path)
                        
                        if success:
                            # 提前确定标签和类型
                            hda_type_key = t.get('hda_type_key') or "Movies 1080p"
                            q_tag = f"REPOST_{site_name}_{tid}"
                            
                            # 预存记录
                            self.state[task_key] = {
                                'site': site_name,
                                'title': t['title'],
                                'status': 'pending_space' if self.low_space else 'downloading',
                                'source_id': tid,
                                'source_url': t['details_url'],
                                'source_details_url': t['details_url'],
                                'source_download_url': t['download_url'],
                                'source_torrent_path': t_path,
                                'original_filename': original_filename,
                                'hda_type_key': hda_type_key,
                                'tag': q_tag,
                                'imdb_id': t.get('imdb_id', ''),
                                'douban_id': t.get('douban_id', ''),
                                'subtitle': t.get('subtitle', ''),
                                'attrs': t.get('attrs')
                            }
                            
                            if not self.low_space:
                                added, info = self.qb.add_torrent(t_path, title=t['title'], tag=q_tag)
                                if added:
                                    print_now(f"  √ 已添加下载并打标 ({q_tag}): {t['title']}")
                                    if info: self.state[task_key]['hash'] = info.get('hash', '')
                                else:
                                    print_now(f"  × 添加下载失败: {t['title']}")
                            else:
                                print_now(f"  ! 磁盘空间不足，任务已排队 (等待空间充足后推送): {t['title']}")
                            
                            self._save_state()
                self._save_state()
            except Exception as e:
                print_now(f"{site_name} Scanner Exception: {e}")

    def watch_qb_progress(self):
        active_tasks = {task_key: data for task_key, data in self.state.items() if data.get('status') == 'downloading'}
        if not active_tasks: return
        
        print_now("--- 阶段 B: 下载进度追踪 ---")
        try:
            qb_torrents = self.qb.client.torrents_info()
            qb_map = {t.hash.lower(): t for t in qb_torrents}
            
            for task_key, data in active_tasks.items():
                q_hash = str(data.get('hash', '')).lower()
                
                if not q_hash:
                    # 补救逻辑
                    q_tag = data.get('tag') or f"REPOST_{data.get('site')}_{data.get('source_id')}"
                    for qt in qb_torrents:
                        if q_tag in qt.tags:
                            q_hash = qt.hash.lower()
                            self.state[task_key]['hash'] = q_hash
                            break

                if q_hash and q_hash in qb_map:
                    q_torrent = qb_map[q_hash]
                    print_now(f"    - 正在追踪: {data['title'][:40]}... (进度: {q_torrent.progress*100:.1f}%)")
                    if q_torrent.progress >= 1.0:
                        print_now(f"      √ 下载完成! 准备进入后处理阶段")
                        self.state[task_key]['status'] = 'ready_to_process'
                        self.state[task_key]['local_path'] = q_torrent.content_path
                        self.state[task_key]['save_path'] = q_torrent.save_path
                        self._save_state()
                else:
                    if qb_torrents: # 检测到 qB 正常返回了列表，但确实找不到这个任务，说明被手工删除了
                        print_now(f"    × [警告] 任务在 QB 中已丢失或被手动删除，系统将停止追踪并标记弃置 (ID: {task_key})")
                        self.state[task_key]['status'] = 'abandoned'
                        self._save_state()
                    else:
                        print_now(f"    × [警告] 未在 QB 中找到任务 (ID: {task_key}, Hash: {q_hash})")
        except Exception as e: print_now(f"Watcher Exception: {e}")

    def process_and_upload(self):
        ready_tasks = {task_key: data for task_key, data in self.state.items() if data.get('status') == 'ready_to_process'}
        if not ready_tasks: return

        print_now("--- 阶段 C: 后处理与发布 ---")
        temp_dir = self.config['settings'].get('temp_dir', '/app/data/.pt_transfer')

        for task_key, data in ready_tasks.items():
            try:
                print_now(f"  ▶ 正在处理: {data['title']}")
                path = data['local_path']
                site_name = data.get('site', 'TTG')
                source_id = data.get('source_id')
                crawler = self.crawlers.get(site_name)
                
                if not crawler:
                    print_now(f"    × 无法加载站点 '{site_name}' 的爬虫实例")
                    continue

                # 种子文件检查与补全
                t_path = data.get('source_torrent_path')
                if not t_path or not os.path.exists(t_path):
                    print_now(f"    - 发现种子文件丢失，正在尝试重新获取...")
                    download_ref = data.get('source_download_url')
                    if not download_ref and site_name == 'TTG':
                        details_url = data.get('source_details_url') or data.get('source_url')
                        if details_url:
                            download_ref = crawler.get_download_url_from_details(details_url)
                    if not download_ref:
                        download_ref = source_id
                    t_path = os.path.join(temp_dir, self._task_temp_name(site_name, source_id))
                    success_dl, _ = crawler.download_torrent(download_ref, t_path)
                    if not success_dl:
                        print_now(f"    × 重新下载种子失败，跳过。")
                        continue
                    self.state[task_key]['source_torrent_path'] = t_path
                
                # 路径映射
                path_mapping = self.config['settings'].get('path_mapping', {})
                for remote_root, local_root in path_mapping.items():
                    if path.startswith(remote_root):
                        path = path.replace(remote_root, local_root, 1)
                        print_now(f"    - 路径映射匹配: {path}")
                        break
                
                main_file = self.processor.find_main_video(path)
                if not main_file:
                    print_now(f"    × 无法定位视频主文件: {path}")
                    continue
                
                print_now(f"    - 正在分析 MediaInfo 和截图...")
                mi_bbcode = self.processor.get_full_mediainfo(main_file)
                media_props = self.processor.parse_media_attributes(main_file)
                screens = self.processor.take_screenshots(main_file, count=4)
                pix_bbcode = self.imghost.upload_batch_to_bbcode(screens)
                
                imdb_id = data.get('imdb_id')
                douban_id = data.get('douban_id')  # 保留源站爬回的豆瓣 ID
                bbcode_intro = ""
                imdb_url = ""

                if imdb_id:
                    print_now(f"    - 识别到 IMDb ({imdb_id})，正在获取简介...")
                    bbcode_intro = self.meta_engine.get_bbcode_intro(imdb_id)
                    # 仅在源站没有豆瓣 ID 时，才尝试从 BBCode 简介里反向提取
                    # 避免覆盖 MT 源站已经爬回的正确豆瓣 ID
                    if not douban_id:
                        douban_id = self.meta_engine.extract_douban_id(bbcode_intro)
                    imdb_url = f"https://www.imdb.com/title/{imdb_id}/"
                elif douban_id:
                    print_now(f"    - 识别到豆瓣 ID ({douban_id})，正在获取简介...")
                    bbcode_intro = self.meta_engine.get_bbcode_intro_by_douban(douban_id)
                    # 按照用户要求，如果源站只有豆瓣信息，就不再反向提取和强填 IMDb 字段
                
                hda_descr = f"{bbcode_intro}\n\n[center]\n{pix_bbcode}\n[/center]\n\n{mi_bbcode}"
                
                ttg_title = data['title']
                raw_subtitle = data.get('subtitle', '')
                
                # 预判：如果 crawler 已经分离好了副标题，且标题里没有 CJK，则直接信任
                subtitle_has_chinese = raw_subtitle and bool(re.search(r'[\u4e00-\u9fff\u3400-\u4dbf\uf900-\ufaff]', raw_subtitle))
                hda_small_descr = raw_subtitle if subtitle_has_chinese else ''
                
                # 从标题中尝试分离英文名和中文。如果爬虫已经把标题处理成了纯英文，这里会自动跳过。
                cjk_match = re.search(r'[\u4e00-\u9fff\u3400-\u4dbf\uf900-\ufaff]', ttg_title)
                if cjk_match:
                    cjk_pos = cjk_match.start()
                    hda_name = ttg_title[:cjk_pos].strip()
                    chinese_part = ttg_title[cjk_pos:].strip()
                else:
                    hda_name = ttg_title.strip()
                    chinese_part = ''
                
                # 整合副标题：如果原始副标题为空，但标题尾部含有中文，则搬运过来
                if not hda_small_descr and chinese_part:
                    hda_small_descr = chinese_part
                
                # 安全兜底：如果副标题（hda_small_descr）包含了重复的标题内容（如 MT 有时会填入种子名），
                # 且标题（hda_name）本身就是那个种子名，则清空副标题以保持简洁。
                if hda_small_descr.lower().replace(' ', '.') == hda_name.lower().replace(' ', '.'):
                    hda_small_descr = ''

                metadata = {
                    'title': hda_name,
                    'subtitle': hda_small_descr,
                    'imdb_url': imdb_url,
                    'douban_id': douban_id,
                    'description_bbcode': hda_descr,
                    'original_filename': data.get('original_filename')
                }
                
                # 优先使用爬虫阶段已经精准辨析过的属性 (包含 Category 强制媒介逻辑)
                parsed_attrs = data.get('attrs')
                if not parsed_attrs:
                    parsed_attrs = crawler.parse_title_attributes(data['title'])
                
                # 创建一个副本进行合并
                final_attrs = dict(parsed_attrs) if parsed_attrs else {}
                
                # 使用 MediaInfo 测得的硬件参数覆盖标题正则分析的结果
                if media_props:
                    # 用户要求：分辨率以标题标注为准，因为有些压制组切掉黑边后物理分辨率不再是标准数值
                    # if media_props.get('resolution'):
                    #     final_attrs['resolution'] = media_props['resolution']
                    #     print_now(f"    - [自动纠正] 分辨率被 MediaInfo 识别为: {media_props['resolution']}")
                    
                    if media_props.get('codec'):
                        final_attrs['codec'] = media_props['codec']
                        print_now(f"    - [自动纠正] 视频编码识别为: {media_props['codec']}")
                    if media_props.get('audio'):
                        final_attrs['audio'] = media_props['audio']
                        print_now(f"    - [自动纠正] 音频编码识别为: {media_props['audio']}")
                
                final_attrs['type_key'] = data['hda_type_key']
                
                hda_id = self.hd.upload(self.state[task_key]['source_torrent_path'], metadata, final_attrs)
                
                if hda_id:
                    hda_torrent_path = os.path.join(temp_dir, self._task_temp_name(site_name, source_id, suffix="_hda.torrent"))
                    if self.hd.download_result_torrent(hda_id, hda_torrent_path):
                        with open(hda_torrent_path, 'rb') as hda_f:  # Bug1修复: 使用 with 确保文件句柄正确关闭
                            self.qb.client.torrents_add(
                                torrent_files=hda_f,
                                save_path=data['save_path'],
                                is_paused=False,
                                is_skip_checking=True,  # 修复: 文件已完整存在，跳过哈希校验直接做种，避免校验期间 0 上传速度
                                category='PT_Repost',
                                tags=data['tag']
                            )
                        
                        # 确保 HDA 辅种种子也应用单种限速
                        print_now(f"    - 已添加 HDA 辅种，正在应用限速...")
                        time.sleep(5) # 等待 qB 完成识别 (从 2s 延长至 5s，确保超级做种设置生效)
                        hda_tasks = self.qb.client.torrents_info(tag=data['tag'])
                        for ht in hda_tasks:
                            self.qb.set_torrent_limit(ht.hash)
                            if self.qb.use_super_seeding:
                                self.qb.set_super_seeding(ht.hash, True)
                            
                        print_now(f"    √ 发布并辅种成功!")
                        self.state[task_key]['status'] = 'completed'
                        self.state[task_key]['processed_time'] = time.time()
                        self._save_state()
                    else:
                        print_now(f"    ! 发布成功但 HDA 种子下载失败")
                        self.state[task_key]['status'] = 'completed'
                        self.state[task_key]['processed_time'] = time.time()
                        self._save_state()
                else:
                    print_now(f"    × 发布失败，可能已存在或站点响应错误")
                    self.state[task_key]['status'] = 'error'
                    self._save_state()
                    
            except Exception as e:
                print_now(f"    × 处理环节异常: {e}")

from web_server import app

def run_flask():
    config_path = os.environ.get('CONFIG_PATH', 'config.yaml')  # Bug5修复: 使用 CONFIG_PATH 环境变量
    config = yaml.safe_load(open(config_path, 'r', encoding='utf-8'))
    port = config.get('web_ui', {}).get('port', 8888)
    app.run(host='0.0.0.0', port=port, use_reloader=False)

def main():
    print_now("=== PT 流水线引擎启动成功 ===")
    flask_thread = threading.Thread(target=run_flask, daemon=True)
    flask_thread.start()
    
    engine = TransferEngine()
    while True:
        try:
            engine.run_one_cycle()
        except Exception as e:
            print_now(f"主引擎发生严重错误阻止了本次循环: {e}")
            
        try:
            config = yaml.safe_load(open(os.environ.get('CONFIG_PATH', 'config.yaml'), 'r', encoding='utf-8'))
            interval = config.get('settings', {}).get('check_interval', 120)
        except Exception:
            interval = 120
            
        print_now(f"--- 循环结束，等待 {interval} 秒后进行下一轮扫描 ---")
        time.sleep(interval)

if __name__ == "__main__":
    main()
