# NexusPHP API 集成文档

> 本文档记录 NexusPHP API 的深度分析结果，重点是与自动化工具（如 PT-Forward）集成相关的接口信息。

## 一、API 概述

### 1.1 技术栈
- **框架**: Laravel 12
- **认证**: Laravel Sanctum (Token-based)
- **数据转换**: Laravel Resources
- **查询构建**: 自定义 ApiQueryBuilder

### 1.2 基础URL
```
{SITE_URL}/api/
```

### 1.3 认证方式
所有API请求需要在Header中携带Token：
```
Authorization: Bearer {token}
```

---

## 二、认证接口

### 2.1 登录获取Token

**请求**
```
POST /api/login
Content-Type: application/json

{
    "username": "用户名",
    "password": "密码"
}
```

**响应**
```json
{
    "success": true,
    "msg": "Authenticate login",
    "data": {
        "id": 1,
        "username": "demo_user",
        "class": 2,
        "class_text": "Power User",
        "uploaded": 1073741824000,
        "downloaded": 536870912000,
        "bonus": 50000.0,
        "seed_points": 100000.0,
        "token": "1|abcdef123456..."
    }
}
```

**密码验证逻辑**
```php
md5($user->secret . $password . $user->secret)
```

### 2.2 登出

**请求**
```
POST /api/logout
Authorization: Bearer {token}
```

### 2.3 第三方工具认证

#### NasTools认证
```
POST /api/nas-tools-approve
Content-Type: application/json

{
    "data": "{AES加密的JSON，包含uid和passkey}"
}
```

#### AMMDS认证 (HMAC签名)
```
POST /api/ammds-approve
Content-Type: application/json

{
    "uid": 1,
    "timestamp": 1710000000000,
    "nonce": "random_string",
    "signature": "hmac_sha256_signature"
}
```

**签名生成**
```python
import hmac
import hashlib

data_to_sign = f"{uid}{passkey_hash}{timestamp}{nonce}"
signature = hmac.new(
    secret_key.encode(),
    data_to_sign.encode(),
    hashlib.sha256
).hexdigest()
```

---

## 三、种子接口

### 3.1 获取种子列表

**请求**
```
GET /api/torrents/{section?}
Authorization: Bearer {token}
```

**查询参数**

| 参数 | 说明 | 示例 |
|------|------|------|
| `includes` | 关联加载 | `includes=user,extra,tags` |
| `include_counts` | 统计计数 | `include_counts=thank_users,reward_logs` |
| `include_fields[torrent]` | 动态字段 | `include_fields[torrent]=has_bookmarked,download_url` |
| `filter[{field}]` | 过滤条件 | `filter[size][gt]=1073741824` |
| `filter_any[{field}]` | OR过滤 | `filter_any[source][in]=1,2,3` |
| `sorts` | 排序 | `sorts=-added,-size` (-表示DESC) |
| `page` | 页码 | `page=1` |
| `per_page` | 每页数量 | `per_page=20` (最大100) |

**过滤操作符**

| 操作符 | 说明 | 示例 |
|--------|------|------|
| `eq` | 等于 | `filter[status][eq]=published` |
| `gt` | 大于 | `filter[size][gt]=1073741824` |
| `lt` | 小于 | `filter[seeders][lt]=10` |
| `gte` | 大于等于 | `filter[added][gte]=2024-01-01` |
| `lte` | 小于等于 | `filter[added][lte]=2024-12-31` |
| `like` | 模糊匹配 | `filter[title][like]=%movie%` |
| `in` | 包含于 | `filter[category][in]=1,2,3` |

**允许过滤的字段**
- `title` - 标题 (支持多关键词空格分隔)
- `category` - 分类
- `source` - 来源
- `medium` - 媒体
- `codec` - 编码
- `audiocodec` - 音频编码
- `standard` - 标准
- `processing` - 处理
- `team` - 团队
- `owner` - 发布者
- `visible` - 可见性
- `added` - 添加时间
- `size` - 大小
- `sp_state` - 促销状态
- `leechers` - 下载数
- `seeders` - 做种数
- `times_completed` - 完成数
- `bookmark` - 收藏过滤 (`include`/`exclude`)

**允许排序的字段**
- `id`, `comments`, `size`, `seeders`, `leechers`, `times_completed`

**允许关联加载**
- `user` - 发布者
- `extra` - 扩展信息(含描述)
- `tags` - 标签

**允许统计计数**
- `thank_users` - 感谢用户数
- `reward_logs` - 打赏记录数
- `claims` - 认领数

**允许动态字段**
- `has_bookmarked` - 是否收藏
- `has_claimed` - 是否认领
- `has_thanked` - 是否感谢
- `has_rewarded` - 是否打赏
- `description` - 描述内容
- `download_url` - 下载链接
- `active_status` - 活动状态(做种/下载中)

**完整示例**
```
GET /api/torrents?includes=user,extra,tags&include_counts=thank_users&include_fields[torrent]=has_bookmarked,download_url&filter[size][gt]=1073741824&filter[category]=1&sorts=-added&page=1&per_page=20
```

**响应**
```json
{
    "success": true,
    "msg": "Torrent list",
    "data": [
        {
            "id": 123,
            "name": "Movie.Name.2024.1080p.BluRay",
            "filename": "movie.torrent",
            "hash": "a1b2c3d4e5f6...",
            "cover": "https://site.com/covers/123.jpg",
            "small_descr": "副标题",
            "category": 1,
            "category_info": {
                "id": 1,
                "name": "电影"
            },
            "size": 10737418240,
            "size_human": "10.00 GB",
            "added": "2024-01-15 10:30:00",
            "added_human": "2小时前",
            "seeders": 50,
            "leechers": 10,
            "times_completed": 200,
            "promotion_info": {
                "upload_multiplier": 2.0,
                "download_multiplier": 0.5
            },
            "sub_categories": {
                "codec": {"label": "编码", "value": "x264"},
                "source": {"label": "来源", "value": "BluRay"}
            },
            "has_bookmarked": true,
            "download_url": "https://site.com/download.php?downhash=...",
            "user": {
                "id": 1,
                "username": "uploader"
            },
            "extra": {
                "descr": "完整描述..."
            },
            "tags": [
                {"id": 1, "name": "动作"}
            ],
            "thank_users_count": 25
        }
    ],
    "links": {...},
    "meta": {
        "current_page": 1,
        "total": 100
    }
}
```

### 3.2 获取种子详情

**请求**
```
GET /api/detail/{id}
Authorization: Bearer {token}
```

**查询参数**
- `includes` - 关联加载 (`user`, `extra`, `tags`)
- `include_counts` - 统计计数
- `include_fields[torrent]` - 动态字段

**响应**
```json
{
    "success": true,
    "msg": "Torrent detail",
    "data": {
        "id": 123,
        "name": "种子名称",
        "size": 10737418240,
        "seeders": 50,
        "leechers": 10,
        "promotion_info": {...},
        "description": "描述内容",
        "images": ["图片URL列表"],
        "download_url": "下载链接",
        "user": {...},
        "extra": {...}
    }
}
```

### 3.3 上传种子

**请求**
```
POST /api/upload
Authorization: Bearer {token}
Content-Type: multipart/form-data
```

**表单字段**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `file` | File | 是 | 种子文件 (.torrent) |
| `name` | String | 是 | 种子标题 |
| `descr` | String | 是 | 种子描述 |
| `type` | Integer | 是 | 分类ID |
| `small_descr` | String | 否 | 副标题 |
| `url` | String | 否 | IMDB链接 |
| `uplver` | String | 否 | 匿名发布: `yes`/`no` |
| `technical_info` | String | 否 | MediaInfo |
| `pt_gen` | String | 否 | PTGen信息 |

**子分类字段** (根据分类配置)
- `source` - 来源ID
- `medium` - 媒体ID
- `codec` - 编码ID
- `audiocodec` - 音频编码ID
- `standard` - 标准ID
- `processing` - 处理ID
- `team` - 团队ID

**促销与特殊设置** (需要权限)
- `sp_state` - 促销状态
- `pos_state` - 置顶状态
- `hr` - Hit & Run设置
- `price` - 种子价格

**响应**
```json
{
    "success": true,
    "msg": "Torrent upload",
    "data": {
        "id": 124
    }
}
```

**错误响应**
```json
{
    "success": false,
    "msg": "种子已存在: ID 123",
    "data": null
}
```

### 3.4 通过Pieces Hash查询

用于检测种子是否已存在（避免重复上传）。

**请求**
```
POST /api/torrent/query-by-pieces-hash
Authorization: Bearer {token}
Content-Type: application/json

{
    "pieces_hash": ["hash1", "hash2", "hash3"]
}
```

**响应**
```json
{
    "success": true,
    "msg": "Query by pieces hash",
    "data": {
        "hash1": 123,
        "hash2": 456
    }
}
```

### 3.5 获取上传分类

**请求**
```
GET /api/sections
Authorization: Bearer {token}
```

**响应**
```json
{
    "success": true,
    "msg": "Upload sections",
    "data": [
        {
            "id": 1,
            "name": "movie",
            "display_name": "电影",
            "categories": [...],
            "sub_categories": [
                {
                    "field": "codec",
                    "label": "编码",
                    "data": [...]
                }
            ]
        }
    ]
}
```

---

## 四、用户接口

### 4.1 获取用户资料

**请求**
```
GET /api/profile/{id?}
Authorization: Bearer {token}
```

> 不传id则获取当前登录用户信息

**响应**
```json
{
    "success": true,
    "msg": "User detail",
    "data": {
        "id": 1,
        "username": "demo_user",
        "class": 2,
        "class_text": "Power User",
        "avatar": "https://site.com/avatar.jpg",
        "uploaded": 1073741824000,
        "uploaded_text": "1.00 TB",
        "downloaded": 536870912000,
        "downloaded_text": "500.00 GB",
        "bonus": 50000.0,
        "seed_points": 100000.0,
        "seed_points_per_hour": 150.5,
        "seedtime": 2592000,
        "seedtime_text": "30天",
        "share_ratio": "2.000",
        "invites": 5
    }
}
```

---

## 五、收藏接口

### 5.1 添加收藏

**请求**
```
POST /api/bookmarks
Authorization: Bearer {token}
Content-Type: application/json

{
    "torrent_id": 123
}
```

### 5.2 删除收藏

**请求**
```
POST /api/bookmarks/delete
Authorization: Bearer {token}
Content-Type: application/json

{
    "torrent_id": 123
}
```

---

## 六、权限系统

### 6.1 路由权限枚举

| 权限 | 说明 |
|------|------|
| `torrent:list` | 种子列表 |
| `torrent:view` | 种子详情 |
| `torrent:upload` | 上传种子 |
| `user:view` | 查看用户 |
| `bookmark:store` | 添加收藏 |
| `bookmark:delete` | 删除收藏 |

### 6.2 功能权限枚举

| 权限 | 说明 |
|------|------|
| `upload` | 上传权限 |
| `uploadspecial` | 特殊区上传 |
| `beanonymous` | 匿名发布 |
| `torrentmanage` | 管理种子 |
| `torrentsticky` | 置顶种子 |
| `torrent_hr` | 设置HR |
| `torrent-set-price` | 设置价格 |
| `view_special_torrent` | 查看特殊区 |

### 6.3 用户等级

| 等级 | 名称 | 积分要求 |
|------|------|----------|
| 0 | Peasant | - |
| 1 | User | 0 |
| 2 | Power User | 40,000 |
| 3 | Elite User | 80,000 |
| 4 | Crazy User | 150,000 |
| 5 | Insane User | 250,000 |
| 6 | Veteran User | 400,000 |
| 7 | Extreme User | 600,000 |
| 8 | Ultimate User | 800,000 |
| 9 | Nexus Master | 1,000,000 |
| 10 | VIP | - |
| 11 | Retiree | - |
| 12 | Uploader | - |
| 13 | Moderator | - |
| 14 | Administrator | - |
| 15 | Sysop | - |
| 16 | Staff Leader | - |

---

## 七、PT-Forward 集成示例

### 7.1 Python 客户端实现

```python
import requests
import hashlib
from pathlib import Path


class NexusPHPClient:
    """NexusPHP API 客户端"""
    
    def __init__(self, base_url: str, username: str, password: str):
        self.base_url = base_url.rstrip('/')
        self.session = requests.Session()
        self._login(username, password)
    
    def _login(self, username: str, password: str):
        """登录获取Token"""
        resp = self.session.post(
            f"{self.base_url}/api/login",
            json={"username": username, "password": password}
        )
        resp.raise_for_status()
        data = resp.json()
        if not data.get('success'):
            raise Exception(f"Login failed: {data.get('msg')}")
        
        self.token = data['data']['token']
        self.session.headers["Authorization"] = f"Bearer {self.token}"
        self.user_info = data['data']
    
    def get_torrents(self, section: str = None, **params) -> dict:
        """获取种子列表"""
        url = f"{self.base_url}/api/torrents"
        if section:
            url += f"/{section}"
        resp = self.session.get(url, params=params)
        resp.raise_for_status()
        return resp.json()
    
    def get_torrent_detail(self, torrent_id: int, **params) -> dict:
        """获取种子详情"""
        resp = self.session.get(
            f"{self.base_url}/api/detail/{torrent_id}",
            params=params
        )
        resp.raise_for_status()
        return resp.json()
    
    def upload_torrent(
        self,
        torrent_path: str,
        name: str,
        descr: str,
        category_id: int,
        **kwargs
    ) -> dict:
        """上传种子"""
        with open(torrent_path, 'rb') as f:
            files = {'file': (Path(torrent_path).name, f)}
            data = {
                'name': name,
                'descr': descr,
                'type': category_id,
                **kwargs
            }
            resp = self.session.post(
                f"{self.base_url}/api/upload",
                files=files,
                data=data
            )
        resp.raise_for_status()
        return resp.json()
    
    def check_duplicate_by_pieces_hash(self, pieces_hash: str) -> int | None:
        """通过pieces_hash检查种子是否存在"""
        resp = self.session.post(
            f"{self.base_url}/api/torrent/query-by-pieces-hash",
            json={"pieces_hash": [pieces_hash]}
        )
        resp.raise_for_status()
        data = resp.json()['data']
        return data.get(pieces_hash)
    
    def get_sections(self) -> list:
        """获取上传分类"""
        resp = self.session.get(f"{self.base_url}/api/sections")
        resp.raise_for_status()
        return resp.json()['data']
    
    def search_torrents(self, keyword: str, **filters) -> dict:
        """搜索种子"""
        params = {
            'filter[title]': keyword,
            **{f'filter[{k}]': v for k, v in filters.items()}
        }
        return self.get_torrents(**params)
```

### 7.2 与 PT-Forward uploader.py 集成

```python
class NexusPHPUploader:
    """PT-Forward NexusPHP上传器"""
    
    def __init__(self, config: dict):
        self.client = NexusPHPClient(
            base_url=config['base_url'],
            username=config['username'],
            password=config['password']
        )
        self.category_mapping = config.get('category_mapping', {})
    
    def upload(self, torrent_info: dict, torrent_path: str) -> int:
        """上传种子到NexusPHP站点"""
        # 检查重复
        pieces_hash = self._calculate_pieces_hash(torrent_path)
        existing_id = self.client.check_duplicate_by_pieces_hash(pieces_hash)
        if existing_id:
            print(f"种子已存在: ID {existing_id}")
            return existing_id
        
        # 映射分类
        category_id = self.category_mapping.get(
            torrent_info['category'],
            torrent_info.get('default_category', 1)
        )
        
        # 上传
        result = self.client.upload_torrent(
            torrent_path=torrent_path,
            name=torrent_info['name'],
            descr=torrent_info['description'],
            category_id=category_id,
            small_descr=torrent_info.get('subtitle', ''),
            uplver='yes' if torrent_info.get('anonymous') else 'no',
            technical_info=torrent_info.get('mediainfo', ''),
        )
        
        if result['success']:
            return result['data']['id']
        else:
            raise Exception(f"Upload failed: {result['msg']}")
    
    def _calculate_pieces_hash(self, torrent_path: str) -> str:
        """计算种子pieces_hash"""
        import bencodepy
        with open(torrent_path, 'rb') as f:
            data = bencodepy.decode(f.read())
        pieces = data[b'info'][b'pieces']
        return hashlib.sha1(pieces).hexdigest()
```

### 7.3 完整工作流示例

```python
# 初始化客户端
client = NexusPHPClient(
    base_url="https://pt-site.com",
    username="bot_user",
    password="bot_password"
)

# 搜索种子
results = client.search_torrents(
    keyword="Movie Name 2024",
    filter={'size': {'gt': 1073741824}}  # > 1GB
)

# 获取详情
if results['data']:
    torrent_id = results['data'][0]['id']
    detail = client.get_torrent_detail(
        torrent_id,
        includes="user,extra,tags",
        include_fields={"torrent": "download_url,description"}
    )
    print(f"下载链接: {detail['data']['download_url']}")

# 上传新种子
result = client.upload_torrent(
    torrent_path="/path/to/movie.torrent",
    name="New Movie 2024 1080p BluRay x264",
    descr="详细描述内容...",
    category_id=1,
    small_descr="中文字幕",
    uplver="yes"
)
print(f"上传成功: ID {result['data']['id']}")
```

---

## 八、错误处理

### 8.1 常见错误码

| HTTP状态码 | 说明 |
|------------|------|
| 401 | 未认证/Token过期 |
| 403 | 权限不足 |
| 404 | 资源不存在 |
| 422 | 参数验证失败 |
| 500 | 服务器错误 |

### 8.2 错误响应格式

```json
{
    "success": false,
    "msg": "错误信息",
    "data": null
}
```

### 8.3 常见错误信息

| 错误信息 | 原因 |
|----------|------|
| `Username or password invalid.` | 用户名或密码错误 |
| `用户未确认` | 邮箱未验证 |
| `用户已禁用` | 账号被禁用 |
| `种子已存在: ID xxx` | 重复上传 |
| `Invalid category` | 无效分类 |
| `no_permission_to_be_anonymous` | 无匿名发布权限 |

---

## 九、最佳实践

### 9.1 Token管理
- Token持久化存储，避免频繁登录
- Token过期时自动重新登录
- 使用环境变量存储敏感信息

### 9.2 请求优化
- 合理使用 `includes` 和 `include_fields`，避免过度加载
- 使用分页，`per_page` 不超过100
- 批量操作时使用 `query-by-pieces-hash` 检查重复

### 9.3 错误处理
- 实现重试机制（特别是网络错误）
- 记录详细日志便于排查
- 区分临时错误和永久错误

### 9.4 安全建议
- 使用专用API账号，限制权限
- 定期更换密码和Token
- 敏感操作添加二次验证

---

## 十、附录

### 10.1 中间件链

```
请求 → auth:sanctum → checkUserStatus → ability(permission) → Controller
```

### 10.2 请求处理流程

```
HTTP Request
    │
    ▼
┌─────────────────┐
│ auth:sanctum    │ ← Token验证
└────────┬────────┘
         ▼
┌─────────────────┐
│ checkUserStatus │ ← 用户状态检查
└────────┬────────┘
         ▼
┌─────────────────┐
│ ability(perm)   │ ← 权限验证
└────────┬────────┘
         ▼
┌─────────────────┐
│ Controller      │ ← 业务逻辑
└────────┬────────┘
         ▼
┌─────────────────┐
│ Repository      │ ← 数据访问
└────────┬────────┘
         ▼
┌─────────────────┐
│ Resource        │ ← 数据转换
└────────┬────────┘
         ▼
    JSON Response
```

### 10.3 相关文件路径

| 文件 | 说明 |
|------|------|
| `routes/api.php` | API路由定义 |
| `app/Http/Controllers/TorrentController.php` | 种子控制器 |
| `app/Http/Controllers/AuthenticateController.php` | 认证控制器 |
| `app/Repositories/TorrentRepository.php` | 种子数据仓库 |
| `app/Repositories/UploadRepository.php` | 上传数据仓库 |
| `app/Http/Resources/TorrentResource.php` | 种子资源转换 |
| `app/Utils/ApiQueryBuilder.php` | 查询构建器 |
| `app/Auth/Permission.php` | 权限辅助类 |
| `app/Enums/Permission/RoutePermissionEnum.php` | 路由权限枚举 |
| `app/Enums/Permission/PermissionEnum.php` | 功能权限枚举 |

---

*文档生成时间: 2026-04-10*
*基于 NexusPHP (Laravel 12 + FilamentPHP 5) 分析*
