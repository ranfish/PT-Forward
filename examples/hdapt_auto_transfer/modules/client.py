import qbittorrentapi
import time
import os
import re
import shutil

class QBManager:
    def __init__(self, config):
        self.host = config['host']
        self.username = config['username']
        self.password = config['password']
        self.save_path = config.get('save_path', '/root/ardtu/pt')
        self.max_global_upload_speed_mb = config.get('max_global_upload_speed_mb', 90)
        self.max_torrent_upload_speed_mb = config.get('max_torrent_upload_speed_mb', 90)
        self.use_super_seeding = config.get('use_super_seeding', True)
        self.max_active_uploads = config.get('max_active_uploads', 100)
        self.max_active_downloads = config.get('max_active_downloads', 5)
        
        self.client = qbittorrentapi.Client(
            host=self.host,
            username=self.username,
            password=self.password
        )

    def apply_limits(self):
        """应用全局限速设置，并同步最大活跃做种数/下载数"""
        try:
            self.login()
            limit_bytes = int(self.max_global_upload_speed_mb * 1024 * 1024)
            # 设置全局上传限速
            self.client.transfer_set_upload_limit(limit=limit_bytes)
            print(f"QB: 已应用全局上传限速: {self.max_global_upload_speed_mb} MB/s", flush=True)
            # 设置最大活跃做种数和最大同时下载数 (通过 QB 偏好 API 下发，此前只存入 YAML 从不生效)
            self.client.app_set_preferences(prefs={
                'max_active_uploads': int(self.max_active_uploads),
                'max_active_downloads': int(self.max_active_downloads),
            })
            print(f"QB: 已应用最大活跃做种数: {self.max_active_uploads}, 最大同时下载数: {self.max_active_downloads}", flush=True)
        except Exception as e:
            print(f"QB: 应用限速失败: {e}", flush=True)

    def set_torrent_limit(self, torrent_hash):
        """为单个种子设置限速"""
        try:
            self.login()
            limit_bytes = int(self.max_torrent_upload_speed_mb * 1024 * 1024)
            self.client.torrents_set_upload_limit(limit=limit_bytes, torrent_hashes=torrent_hash)
            # print(f"QB: 已为种子 {torrent_hash[:8]} 开启限速 ({self.max_torrent_upload_speed_mb}MB/s)", flush=True)
            return True
        except Exception as e:
            print(f"QB: 设置单种限速失败: {e}", flush=True)
            return False

    def set_super_seeding(self, torrent_hash, value=True):
        """开启/关闭超级做种模式"""
        try:
            self.login()
            self.client.torrents_set_super_seeding(torrent_hashes=torrent_hash, value=value)
            return True
        except Exception as e:
            print(f"QB: 设置超级做种失败: {e}", flush=True)
            return False

    def reannounce_seeding_torrents(self):
        """强制重新向 Tracker 汇报所有做种中的 PT_Repost 种子。
        
        原因: HDA Tracker 的自然汇报间隔通常为 30~60 分钟。
        在此期间新加入的下载者无法发现本节点，导致上传速度持续归零。
        每轮循环（约 2 分钟）主动重新汇报，保证 Peer 列表始终新鲜。
        """
        try:
            self.login()
            torrents = self.client.torrents_info()
            hashes = []
            for t in torrents:
                cat = getattr(t, 'category', '')
                tags = getattr(t, 'tags', '')
                is_repost = cat == 'PT_Repost' or any(
                    tg.strip().startswith('REPOST_') for tg in tags.split(',')
                )
                if is_repost and t.progress >= 1.0:
                    hashes.append(t.hash)

            if hashes:
                self.client.torrents_reannounce(torrent_hashes=hashes)
                print(f"QB: 已强制重新汇报 {len(hashes)} 个做种任务 (确保 Tracker 持续分配新 Peer)", flush=True)
        except Exception as e:
            print(f"QB: 重新汇报失败: {e}", flush=True)

    def get_free_space_gb(self, path):
        """获取指定路径的可用空间 (GB)"""
        try:
            # 如果路径不存在，检查其父目录
            check_path = path
            while not os.path.exists(check_path) and os.path.dirname(check_path) != check_path:
                check_path = os.path.dirname(check_path)
            
            usage = shutil.disk_usage(check_path)
            free_gb = usage.free / (1024**3)
            return free_gb
        except Exception as e:
            print(f"QB: 检查磁盘空间失败 ({path}): {e}", flush=True)
            return 999  # 出错时返回较大值避免误停

    def pause_all_downloads(self):
        """暂停所有正在下载的任务"""
        try:
            self.login()
            # 获取所有正在下载状态的种子
            torrents = self.client.torrents_info(status_filter='downloading')
            hashes = [t.hash for t in torrents]
            if hashes:
                self.client.torrents_pause(torrent_hashes=hashes)
                print(f"QB: 已暂停 {len(hashes)} 个下载任务 (原因: 磁盘空间不足)", flush=True)
        except Exception as e:
            print(f"QB: 暂停任务失败: {e}", flush=True)

    def resume_all_downloads(self):
        """恢复所有已暂停的下载任务"""
        try:
            self.login()
            # 获取所有已暂停状态的种子 (可能会恢复非本程序添加的任务，但既然空间够了就全开)
            torrents = self.client.torrents_info(status_filter='paused')
            hashes = [t.hash for t in torrents]
            if hashes:
                self.client.torrents_resume(torrent_hashes=hashes)
                print(f"QB: 已恢复 {len(hashes)} 个下载任务 (原因: 磁盘空间已恢复)", flush=True)
        except Exception as e:
            print(f"QB: 恢复任务失败: {e}", flush=True)

    def login(self):
        try:
            self.client.auth_log_in()
            return True
        except Exception as e:  # Bug7修复: 记录登录失败日志，方便排查
            print(f"QB: 登录失败 ({self.host}): {e}", flush=True)
            return False

    def get_stats(self):
        """获取当前下载和上传中的种子数量"""
        try:
            self.login()
            torrents = self.client.torrents_info()
            downloading = len([t for t in torrents if t.state in ['downloading', 'stalledDL', 'metaDL']])
            uploading = len([t for t in torrents if t.state in ['uploading', 'stalledUP', 'seeding']])
            return {'downloading': downloading, 'uploading': uploading}
        except:
            return {'downloading': 0, 'uploading': 0}

    def add_torrent(self, torrent_path, title=None, tag=None):
        try:
            self.login()
            # 1. 获取添加前的快照
            before = {t.hash for t in self.client.torrents_info()}
            
            # 2. 尝试添加并携带标签 (只有在文件存在时，比如 Docker 初次运行时才需要上传)
            tags = [tag] if tag else ["PT_AUTO_TRANSFER"]
            import os
            if os.path.exists(torrent_path):
                with open(torrent_path, 'rb') as f:
                    self.client.torrents_add(
                        torrent_files=[f],
                        save_path=self.save_path,
                        is_paused=False,
                        tags=tags
                    )
                
                # 3. Bug3修复: 用独立标志位确保只识别第一个匹配种子，不误匹配其他人同时添加的种子
                found_hash = None
                for _ in range(10):
                    time.sleep(1)
                    after = self.client.torrents_info()
                    for t in after:
                        if tag and tag in t.tags:
                            found_hash = t.hash
                            break
                        if t.hash not in before:
                            found_hash = t.hash
                            break
                    if found_hash:
                        break
                
                if found_hash:
                    limit_bytes = int(self.max_torrent_upload_speed_mb * 1024 * 1024)
                    self.client.torrents_set_upload_limit(limit=limit_bytes, torrent_hashes=found_hash)
                    if self.use_super_seeding:
                        self.set_super_seeding(found_hash, True)
                    return True, {'hash': found_hash}
            else:
                print(f"QB Add: 忽略物理文件上传，因为它已被 Docker 重启清理 ({torrent_path})", flush=True)
            
            # 4. 如果没找见新的，尝试通过标题 (Title) 匹配并“补打”标签
            if title:
                import difflib
                raw_title = title.split('/')[0].replace('.torrent', '')
                # 去除非字母数字符号以及中文字符，只保留纯粹的核心英文及数字串
                clean_title = re.sub(r'[\(\)\[\]\s\.\_\-]', '', raw_title)
                clean_title = re.sub(r'[\u4e00-\u9fa5]+', '', clean_title).lower()
                for t in self.client.torrents_info():
                    clean_t_name = re.sub(r'[\(\)\[\]\s\.\_\-]', '', t.name).lower()
                    ratio = difflib.SequenceMatcher(None, clean_title, clean_t_name).ratio()
                    if clean_title in clean_t_name or clean_t_name in clean_title or ratio > 0.85:
                        # 补打标签，确保下次能直接通过标签找到
                        if tag and tag not in t.tags:
                            self.client.torrents_add_tags(tags=tag, torrent_hashes=t.hash)
                        # 补打限速
                        limit_bytes = int(self.max_torrent_upload_speed_mb * 1024 * 1024)
                        self.client.torrents_set_upload_limit(limit=limit_bytes, torrent_hashes=t.hash)
                        if self.use_super_seeding:
                            self.set_super_seeding(t.hash, True)
                        return True, {'hash': t.hash}
            
            # 5. 最后保底：通过种子文件名匹配
            base_name = os.path.basename(torrent_path).replace('.torrent', '').split('_')[0]
            for t in self.client.torrents_info():
                if base_name in t.name:
                    if tag and tag not in t.tags:
                        self.client.torrents_add_tags(tags=tag, torrent_hashes=t.hash)
                    return True, {'hash': t.hash}
            
            return True, None
        except Exception as e:
            print(f"QB Add Error: {e}")
            return False, None

    def check_and_cleanup(self, rules):
        try:
            self.login()
            torrents = self.client.torrents_info()
            
            to_delete_reposts = []
            for t in torrents:
                # 核心防误删：只针对已经成功搬运到了 HDArea 并打上 PT_Repost 标签或分类的种子进行计算！
                # 注意：我们在 main.py 中添加辅种时实际用的是 category='PT_Repost'
                cat = getattr(t, 'category', '')
                tags = getattr(t, 'tags', '')
                if cat != 'PT_Repost' and 'PT_Repost' not in tags:
                    continue
                if t.progress < 1.0: continue
                
                # 记录上传速度用于计算时间窗口内的平均速度
                if not hasattr(self, 'speed_history'):
                    self.speed_history = {}
                if t.hash not in self.speed_history:
                    self.speed_history[t.hash] = []
                
                # 瞬时上传速度 (单位: Byte/s)
                self.speed_history[t.hash].append(getattr(t, 'upspeed', 0))
                
                # 根据主引擎检查间隔和设定时间，计算需要保留多少个历史快照
                check_interval = rules.get('check_interval', 120)
                low_speed_time_minutes = rules.get('low_speed_time_minutes', 10)
                max_history_len = max(1, int((low_speed_time_minutes * 60) / check_interval))
                
                if len(self.speed_history[t.hash]) > max_history_len:
                    self.speed_history[t.hash].pop(0)

                # 规则1: 超过做种时间
                seed_time_h = t.seeding_time / 3600
                if seed_time_h > rules.get('max_seed_time_hours', 48):
                    print(f"Cleanup: {t.name} (Time Limit reached: {seed_time_h:.1f}h)")
                    to_delete_reposts.append(t)
                    continue
                
                # 规则2: 出种人数达标 (Seeders)
                # PT站有时不返回其他做种者，导致 num_seeds 为 0。此时尽量以 qB 内显示的全局种子数为准
                swarm_seeders = getattr(t, 'num_seeds', 0)
                if swarm_seeders >= rules.get('min_seeders_for_deletion', 5):
                    print(f"Cleanup: {t.name} (Seeders Limit reached: {swarm_seeders})")
                    to_delete_reposts.append(t)
                    continue

                # 规则3: 分享率达标且长时间平均上传速度低于阈值
                min_ratio = rules.get('min_ratio_for_deletion', 1.1)
                low_speed_threshold_bytes = rules.get('low_speed_threshold_kb', 20) * 1024
                
                if getattr(t, 'ratio', 0) > min_ratio:
                    history = self.speed_history[t.hash]
                    # 只有积攒了足够的时间窗口才进行末位淘汰评判
                    if len(history) >= max_history_len:
                        avg_speed = sum(history) / len(history)
                        if avg_speed < low_speed_threshold_bytes:
                            print(f"Cleanup: {t.name} (Ratio > {min_ratio} and {low_speed_time_minutes}-min Avg Speed: {avg_speed/1024:.1f} KB/s < {low_speed_threshold_bytes/1024:.0f} KB/s)")
                            to_delete_reposts.append(t)
                            continue

                    
            # 统一执行删除，并连带删除原站(TTG/MT)的种子
            for t in to_delete_reposts:
                related_hashes = [t.hash]
                
                # 提取此 HDArea 种子绑定的专属原站标签 (如 REPOST_MT_1158526)
                t_tags = getattr(t, 'tags', '').split(',')
                rep_tags = [tg.strip() for tg in t_tags if tg.strip().startswith('REPOST_')]

                # 寻找对应的原站源头种子
                for other in torrents:
                    if other.hash == t.hash: continue
                    
                    other_tags = [tg.strip() for tg in getattr(other, 'tags', '').split(',')]
                    other_cats = getattr(other, 'category', '')
                    
                    # 匹配原则1: 如果带有同一个 REPOST_xxx 标签或分类，说明是它的老祖宗，绝对要一起删！
                    if any(rt in other_tags or rt == other_cats for rt in rep_tags if rt):
                        related_hashes.append(other.hash)
                    # 匹配原则2: 退化处理，如果刚好装在同一个路径下 (适用于老版本残留辅种)
                    elif other.content_path and other.content_path == getattr(t, 'content_path', ''):
                        related_hashes.append(other.hash)
                
                # 双重删除并连带物理文件连根拔起
                self.client.torrents_delete(delete_files=True, torrent_hashes=related_hashes)
                
                # 释放速度记录内存
                for h in related_hashes:
                    if hasattr(self, 'speed_history') and h in self.speed_history:
                        del self.speed_history[h]
                        
                
        except Exception as e:
            print(f"Cleanup error: {e}")
