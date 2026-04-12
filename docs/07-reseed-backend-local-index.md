# Reseed-backend 项目深度分析报告

## 项目概述

Reseed 是一个易于使用的跨站辅种工具，通过对本地磁盘进行索引，搜索全网可辅种种子并提供下载链接。该项目解决了 PT 站点用户入站后辅种繁琐的痛点，帮助用户快速找到可以辅种的资源。

**技术栈**: Python + Flask + MySQL + Redis + WebSocket (Socket.IO)  
**前端**: Vue.js (独立项目)  
**核心功能**: 磁盘索引、种子匹配、跨站搜索、辅种链接生成

---

## 一、项目架构与技术栈

### 1.1 整体架构

Reseed-backend 采用典型的前后端分离架构：

```
Reseed-backend
├── app.py                    # Flask 应用入口
├── config/
│   └── config.py            # 配置文件
├── models/
│   └── user.py              # 用户模型
├── views/                    # 路由和控制器
│   ├── __init__.py          # 应用工厂
│   ├── user.py              # 用户相关路由
│   ├── reseed.py            # 辅种核心路由
│   └── plugin.py            # 插件路由
├── utils/                    # 工具模块
│   ├── database.py          # 数据库操作
│   ├── torrent_compare.py   # 种子比较算法
│   └── sites/               # 站点适配器
│       ├── tjupt.py
│       ├── ourbits.py
│       └── hdchina.py
├── scripts/                  # 脚本工具
│   ├── reseed.py            # 磁盘索引工具
│   └── 6v.py                # 6v 站点种子下载
├── _db/
│   └── reseed.sql           # 数据库结构
└── requirements.txt          # Python 依赖
```

### 1.2 技术栈详解

#### 1.2.1 后端框架

**Flask 1.1.1** - 轻量级 Web 框架
- 路由和视图处理
- 请求和响应管理
- 扩展插件支持

#### 1.2.2 数据库

**MySQL** - 关系型数据库
- 存储用户信息
- 存储种子索引
- 存储历史记录

**Redis** - 内存缓存
- 种子信息缓存
- API Token 缓存
- 查询结果缓存

#### 1.2.3 实时通信

**Flask-SocketIO 4.2.1** - WebSocket 支持
- 实时种子匹配
- 进度推送
- 双向通信

#### 1.2.4 其他核心依赖

```python
Flask-Login==0.4.1        # 用户认证
Flask-Limiter==1.1.0      # 速率限制
Flask-Cors==3.0.9         # 跨域支持
Flask-MySQL==1.4.0        # MySQL 集成
flask-redis==0.4.0        # Redis 集成
bcrypt==3.1.7             # 密码哈希
requests==2.22.0          # HTTP 客户端
beautifulsoup4==4.8.1     # HTML 解析
bencoder==0.2.0           # Torrent 文件解析
```

---

## 二、Flask 应用结构与路由设计

### 2.1 应用工厂模式

[views/__init__.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/__init__.py#L16-L40) 实现了应用工厂模式：

```python
def create_app(debug=False):
    """Create an application."""
    app = Flask(__name__, instance_relative_config=True)
    app.config.from_pyfile('config.py')
    app.debug = debug

    from .reseed import reseed
    from .plugin import plugin
    from .user import us
    app.register_blueprint(us)
    app.register_blueprint(reseed)
    app.register_blueprint(plugin)

    # 初始化扩展
    cors.init_app(app)
    socketio.init_app(app, cors_allowed_origins='*')
    limiter.init_app(app)
    login_manager.init_app(app)
    mysql.init_app(app)
    redis.init_app(app)

    return app
```

**设计优势**:
- 便于测试
- 支持多实例
- 配置灵活
- 扩展易于管理

### 2.2 Blueprint 路由组织

项目使用 Flask Blueprint 组织路由：

#### 2.2.1 用户路由 Blueprint

[views/user.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/user.py#L12-L14):

```python
us = Blueprint('user', __name__)

@us.route('/signup', methods=['POST'])
def sign_up():
    # 注册逻辑

@us.route('/login', methods=['POST'])
def log_in():
    # 登录逻辑
```

#### 2.2.2 辅种路由 Blueprint

[views/reseed.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/reseed.py#L16-L18):

```python
reseed = Blueprint('reseed', __name__)

@reseed.route('/upload_json', methods=['POST'])
@login_required
@limiter.limit('10/day;5/hour')
def upload_file():
    # 上传索引文件

@reseed.route('/sites_info')
@login_required
def sites_info():
    # 获取站点信息
```

#### 2.2.3 插件路由 Blueprint

[views/plugin.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/plugin.py#L12-L14):

```python
plugin = Blueprint('plugin', __name__)

@plugin.route('/hdchina_token')
@login_required
def get_hdchina_token():
    # 获取 HDChina Token
```

### 2.3 路由装饰器

#### 2.3.1 认证装饰器

[views/reseed.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/reseed.py#L20-L28) 定义了 WebSocket 认证装饰器：

```python
def authenticated_only(f):
    @functools.wraps(f)
    def wrapped(*args, **kwargs):
        if not current_user.is_authenticated:
            disconnect()
        else:
            return f(*args, **kwargs)
    return wrapped
```

#### 2.3.2 限流装饰器

[views/reseed.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/reseed.py#L30-L31) 使用 Flask-Limiter：

```python
@reseed.route('/upload_json', methods=['POST'])
@login_required
@limiter.limit('10/day;5/hour')
def upload_file():
    # 每天 10 次，每小时 5 次
```

### 2.4 错误处理

[app.py](file:///home/incast/PT-Forward/examples/Reseed-backend/app.py#L13-L16) 定义了速率限制错误处理：

```python
@app.errorhandler(429)
def ratelimit_handler(e):
    return jsonify({'success': False, 'msg': "Rate limit exceeded: %s" % e.description}), 429
```

---

## 三、数据库模型与设计

### 3.1 数据库表结构

#### 3.1.1 用户表 (users)

存储用户账户信息和站点绑定：

```sql
CREATE TABLE `users` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `username` varchar(50) NOT NULL,
  `passhash` varchar(60) NOT NULL,
  `tjupt_id` int(11) DEFAULT NULL,
  `ourbits_id` int(11) DEFAULT NULL,
  `enable` tinyint(1) NOT NULL DEFAULT '1',
  PRIMARY KEY (`id`),
  UNIQUE KEY `username` (`username`)
);
```

**字段说明**:
- `id`: 用户 ID（主键）
- `username`: 用户名（唯一）
- `passhash`: 密码哈希（bcrypt）
- `tjupt_id`: TJUPT 站点 ID
- `ourbits_id`: OurBits 站点 ID
- `enable`: 账户是否启用

#### 3.1.2 种子表 (torrents)

存储种子索引信息：

```sql
CREATE TABLE `torrents` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL,
  `files` longtext,
  `length` bigint(20) NOT NULL,
  `sites_existed` text CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci NOT NULL,
  `added_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `name` (`name`)
) ENGINE=MyISAM DEFAULT CHARSET=utf8mb4;
```

**字段说明**:
- `id`: 种子 ID（主键）
- `name`: 种子名称
- `files`: 文件列表（JSON 格式）
- `length`: 总大小
- `sites_existed`: 存在的站点
- `added_at`: 添加时间

#### 3.1.3 种子记录表 (torrent_records)

存储种子在各站点的记录：

```sql
CREATE TABLE `torrent_records` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `tid` int(11) NOT NULL,
  `sid` int(11) NOT NULL,
  `site` varchar(15) NOT NULL,
  `added_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `tid` (`tid`),
  KEY `site` (`site`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**字段说明**:
- `id`: 记录 ID（主键）
- `tid`: 种子 ID（关联 torrents 表）
- `sid`: 站点种子 ID
- `site`: 站点名称
- `added_at`: 添加时间

#### 3.1.4 站点表 (sites)

存储支持的站点信息：

```sql
CREATE TABLE `sites` (
  `site` varchar(10) NOT NULL,
  `base_url` varchar(30) NOT NULL,
  `download_page` varchar(50) NOT NULL DEFAULT 'download.php?id={}',
  `rss_page` varchar(255) DEFAULT 'torrentrss.php?rows=50&passkey={}&linktype=dl',
  `torrents_page` varchar(50) NOT NULL DEFAULT 'torrents.php?incldead=0&page={}',
  `enabled` tinyint(1) NOT NULL DEFAULT '1',
  `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `passkey` varchar(32) NOT NULL,
  `cookies` mediumtext NOT NULL,
  `skip_page` int(11) NOT NULL DEFAULT '0',
  `show` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`site`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

#### 3.1.5 历史记录表 (historys)

存储用户查询历史和缓存：

```sql
CREATE TABLE `historys` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `uid` int(11) NOT NULL,
  `hash` char(32) NOT NULL,
  `result` json NOT NULL,
  `ip` varchar(40) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `hash` (`hash`),
  KEY `uid` (`uid`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**字段说明**:
- `hash`: 查询内容的 MD5 哈希（用于缓存）
- `result`: 查询结果（JSON 格式）
- `time`: 查询时间
- `uid`: 用户 ID
- `ip`: 用户 IP

#### 3.1.6 错误种子表 (error_torrents)

记录错误的种子：

```sql
CREATE TABLE `error_torrents` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `tid` int(11) NOT NULL,
  `site` varchar(20) NOT NULL,
  `reason` varchar(100) NOT NULL,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 3.2 数据库操作类

[utils/database.py](file:///home/incast/PT-Forward/examples/Reseed-backend/utils/database.py#L5-L70) 封装了数据库操作：

```python
class Database(MySQL):
    def exec(self, sql: str, args=None, cursor: object = pymysql.cursors.DictCursor):
        db = self.get_db()
        cursor = db.cursor(cursor)
        cursor.execute(sql, args)
        data = cursor.fetchall()
        return data

    def get_sites_info(self):
        return self.exec("SELECT `site`, `base_url` FROM `sites` WHERE `show` = 1")

    def select_torrent(self, name):
        return self.exec("SELECT `id`, `name`, `files`, `length` FROM `torrents` WHERE `name` = %s",
                         (str(name),))

    def find_torrents_by_id(self, tid):
        return self.exec("SELECT site, sid FROM torrent_records WHERE tid = %s", (tid,))

    def find_tid_by_hash(self, hex_info_hash):
        tid = self.exec("SELECT tid FROM torrent_records WHERE hex_info_hash = %s", (hex_info_hash,))
        return tid[0]['tid'] if tid else -1
```

**设计特点**:
- 继承 Flask-MySQL
- 默认返回字典格式
- 统一的执行接口
- 自动资源管理

---

## 四、站点适配器与爬虫实现

### 4.1 站点认证机制

项目支持多个 PT 站点的认证，每个站点有独立的认证实现。

#### 4.1.1 TJUPT 站点认证

[utils/sites/tjupt.py](file:///home/incast/PT-Forward/examples/Reseed-backend/utils/sites/tjupt.py#L7-L25) 实现了 TJUPT 的 ID 和 Passkey 验证：

```python
def check_id_passkey_tjupt(tjupt_id, tjupt_passkey):
    api_type = 'verify_id_passkey'
    sign = hashlib.md5((current_app.config.get('TJUPT_TOKEN') + api_type + tjupt_id + tjupt_passkey +
                        current_app.config.get('TJUPT_SECRET')).encode('utf-8')).hexdigest()
    try:
        resp = requests.get('https://tjupt.org/api_username.php', params={
            'token': current_app.config.get('TJUPT_TOKEN'),
            'id': tjupt_id,
            'passkey': tjupt_passkey,
            'type': api_type,
            'sign': sign
        }, timeout=30)
        if resp.status_code == 200:
            data = resp.json()
            if data['status'] == 0:
                return ''
            else:
                return 'Auth failed! Please check your ID and passkey.'
        else:
            return 'Network error! Please try it later...'
    except requests.RequestException:
        return 'Network error! Please try it later...'
```

**认证流程**:
1. 构造签名：`MD5(TOKEN + api_type + id + passkey + SECRET)`
2. 发送 GET 请求到 API
3. 验证响应状态码和返回数据

#### 4.1.2 OurBits 站点认证

[utils/sites/ourbits.py](file:///home/incast/PT-Forward/examples/Reseed-backend/utils/sites/ourbits.py#L7-L25) 实现了 OurBits 的认证：

```python
def check_id_passkey_ourbits(ob_id, ob_passkey):
    verity = hashlib.md5(('{}{}{}{}'.format(current_app.config.get('OURBITS_TOKEN'), ob_id, ob_passkey,
                                            current_app.config.get('OURBITS_SECRET'))).encode('utf-8')).hexdigest()
    try:
        resp = requests.get('https://www.ourbits.club/api_reseed.php', params={
            'token': current_app.config.get('OURBITS_TOKEN'),
            'id': ob_id,
            'verity': verity
        }, timeout=30)
        if resp.status_code == 200:
            data = resp.json()
            if data['success']:
                return ''
            else:
                return 'Auth failed! Please check your ID and passkey.'
        else:
            return 'Network error! Please try it later...'
    except requests.RequestException:
        return 'Network error! Please try it later...'
```

#### 4.1.3 HDChina 站点认证

[utils/sites/hdchina.py](file:///home/incast/PT-Forward/examples/Reseed-backend/utils/sites/hdchina.py#L7-L20) 实现了 HDChina 的认证：

```python
def check_id_passkey_hdchina(hdchina_username, hdchina_userkey):
    try:
        resp = requests.post('https://api.hdchina.org/v1/3rd/reseed/checkUserToken', data={
            'userKey': hdchina_userkey
        }, headers={'Authorization': "Bearer {}".format(current_app.config.get('HDCHINA_APIKEY'))}, timeout=30)
        if resp.status_code == 200:
            data = resp.json()
            if data['status'] == 'success':
                if str.lower(hdchina_username) == str.lower(data['data']['username']):
                    return ''
                return 'Auth failed! Username mismatch, please contact administrator.'
            else:
                return 'Auth failed! Please check your ID and passkey.'
        else:
            return 'Network error! Please try it later...'
    except requests.RequestException:
        return 'Network error! Please try it later...'
```

### 4.2 HDChina Token 管理

[views/plugin.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/plugin.py#L12-L46) 实现了 HDChina Token 的获取和刷新：

```python
@plugin.route('/hdchina_token')
@login_required
def get_hdchina_token():
    token = redis.get('_hdchina_token')
    if token:
        return jsonify({'success': True, 'token': str(token, encoding='utf-8')})
    else:
        result = refresh_hdchina_token().json()
        if result['success']:
            token = redis.get('_hdchina_token')
            return jsonify({'success': True, 'token': str(token, encoding='utf-8')})
        else:
            return jsonify(result), 500

def refresh_hdchina_token():
    token = redis.get('_hdchina_token')
    if token:
        return jsonify({'success': True})
    try:
        resp = requests.get('https://api.hdchina.org/v1/3rd/reseed/requestToken',
                            headers={'Authorization': "Bearer {}".format(current_app.config.get('HDCHINA_APIKEY'))},
                            timeout=30)
        if resp.status_code == 200:
            data = resp.json()
            if data['status'] == 'success':
                token = data['data']['token']
                redis.set('_hdchina_token', token, int(data['data']['expire']) - int(time.time()) - 10)
                return jsonify({'success': True})
            else:
                return jsonify({'success': False, 'msg': 'Reseed auth failed! Please report to @tongyifan'})
        else:
            return jsonify({'success': False, 'msg': 'Network error! Please try it later...'})
    except requests.RequestException:
        return jsonify({'success': False, 'msg': 'Network error! Please try it later...'})
```

**Token 管理**:
- Redis 缓存 Token
- 自动刷新过期 Token
- Bearer Token 认证

---

## 五、种子比较与匹配算法

### 5.1 种子比较核心算法

[utils/torrent_compare.py](file:///home/incast/PT-Forward/examples/Reseed-backend/utils/torrent_compare.py#L10-L49) 实现了种子比较的核心逻辑：

```python
def compare_torrents(name, files):
    from views import redis, mysql

    # 先从 Redis 缓存获取
    torrents = redis.get(name)
    if not torrents:
        torrents = mysql.select_torrent(name)
    else:
        torrents = json.loads(str(torrents, encoding='utf-8'))

    cmp_success = []
    cmp_warning = []
    for t in torrents:
        success_count = failure_count = 0
        torrent_files = eval(t['files'])

        if len(torrent_files):
            # 多文件种子
            if type(files) is int:
                continue

            # Windows 路径处理
            keys = list(files.keys())
            for key in keys:
                files[key.replace('\\', '/')] = files.pop(key)

            # 文件级比较
            for k, v in torrent_files.items():
                if v * 0.95 < files.get(k, -1) < v * 1.05:
                    success_count += 1
                else:
                    failure_count += 1
            
            # 判断匹配结果
            if failure_count:
                if success_count > failure_count:
                    cmp_warning.append({'id': t['id']})
            else:
                cmp_success.append({'id': t['id']})
        else:
            # 单文件种子
            if type(files) is not int:
                continue
            if t['length'] * 0.95 < files < t['length'] * 1.05:
                cmp_success.append({'id': t['id']})
    
    return {'name': name, 'cmp_success': cmp_success, 'cmp_warning': cmp_warning}
```

### 5.2 匹配策略

#### 5.2.1 文件大小匹配

使用 **±5%** 的容差范围：

```python
if v * 0.95 < files.get(k, -1) < v * 1.05:
    success_count += 1
else:
    failure_count += 1
```

**原因**:
- 文件大小可能有微小差异
- 不同编码的文件大小可能不同
- 允许一定的误差范围

#### 5.2.2 匹配结果分类

1. **完全匹配 (cmp_success)**:
   - 单文件：总大小在 ±5% 范围内
   - 多文件：所有文件都匹配

2. **部分匹配 (cmp_warning)**:
   - 多文件：成功匹配的文件数 > 失败的文件数
   - 可能有文件重命名或文件结构差异

3. **不匹配**:
   - 超出容差范围
   - 成功匹配数 ≤ 失败匹配数

### 5.3 路径处理

[utils/torrent_compare.py](file:///home/incast/PT-Forward/examples/Reseed-backend/utils/torrent_compare.py#L32-L34) 处理 Windows 路径：

```python
keys = list(files.keys())
for key in keys:
    files[key.replace('\\', '/')] = files.pop(key)
```

**目的**: 统一路径分隔符，确保跨平台兼容

---

## 六、用户认证与权限管理

### 6.1 用户模型

[models/user.py](file:///home/incast/PT-Forward/examples/Reseed-backend/models/user.py#L7-L24) 定义了用户模型：

```python
class User(UserMixin):
    def __init__(self, user):
        self.user = user
        self.id = user['id']

    def is_active(self):
        return self.user['enable']

    def get_auth_token(self):
        key = self.user['passhash']
        ts_str = str(time.time() + current_app.config.get('REMEMBER_COOKIE_DURATION'))
        ts_byte = ts_str.encode("utf-8")
        sha1_tshexstr = hmac.new(key.encode("utf-8"), ts_byte, 'sha1').hexdigest()
        token = self.user['username'] + ':' + ts_str + ':' + sha1_tshexstr
        b64_token = base64.urlsafe_b64encode(token.encode("utf-8"))
        return b64_token.decode("utf-8")
```

### 6.2 Token 生成机制

**Token 格式**: `base64(username:timestamp:sha1)`

**生成步骤**:
1. 获取用户密码哈希作为密钥
2. 计算过期时间戳
3. 使用 HMAC-SHA1 签名：`HMAC(passhash, timestamp)`
4. 拼接：`username:timestamp:signature`
5. Base64 编码

**示例**:
```python
key = "hashed_password"
ts_str = "1234567890"
sha1 = hmac.new(key.encode("utf-8"), ts_str.encode("utf-8"), 'sha1').hexdigest()
token = "admin:1234567890:" + sha1
b64_token = base64.urlsafe_b64encode(token.encode("utf-8"))
```

### 6.3 Token 验证机制

[views/__init__.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/__init__.py#L56-L75) 实现了 Token 验证：

```python
@login_manager.request_loader
def load_user_from_request(request):
    def verify_token(token):
        try:
            token_str = base64.urlsafe_b64decode(token).decode('utf-8')
            token_list = token_str.split(':')
            if len(token_list) != 3:
                return False
            
            username = token_list[0]
            user = mysql.get_user(username)
            if not user or not user['enable']:
                return False
            
            key = user['passhash']
            ts_str = token_list[1]
            
            # 检查过期时间
            if float(ts_str) < time.time():
                return False
            
            # 验证签名
            known_sha1_tsstr = token_list[2]
            sha1 = hmac.new(key.encode("utf-8"), ts_str.encode('utf-8'), 'sha1')
            calc_sha1_tsstr = sha1.hexdigest()
            
            return User(user) if calc_sha1_tsstr == known_sha1_tsstr else None
        except Exception:
            return None

    token = request.headers.get('Authorization')
    if token:
        token = token.replace('Bearar ', '', 1)
        return verify_token(token)
    else:
        return None
```

**验证步骤**:
1. Base64 解码 Token
2. 分割获取 `username`, `timestamp`, `signature`
3. 查询用户信息
4. 检查用户是否启用
5. 检查 Token 是否过期
6. 重新计算签名并验证

### 6.4 用户注册

[views/user.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/user.py#L14-L38) 实现了用户注册：

```python
@us.route('/signup', methods=['POST'])
def sign_up():
    username = request.form['username']
    password = request.form['password']

    site = request.form.get('site', 'tjupt')
    user_id = request.form['id']
    user_passkey = request.form['passkey']

    # 检查站点 ID 是否已被注册
    if not mysql.check_site_id_registered(site, user_id):
        return jsonify({'success': False, 'msg': 'This ID has been used in Site: {}.'.format(site)}), 403

    # 验证站点账号
    msg = check_id_passkey(site, user_id, user_passkey)
    if msg:
        return jsonify({'success': False, 'msg': msg}), 403

    # 创建用户
    if not mysql.get_user(username):
        salt = bcrypt.gensalt()
        passhash = bcrypt.hashpw(password.encode('utf-8'), salt)

        mysql.signup(username, passhash.decode('utf-8'), site, user_id)
        return jsonify({'success': True, 'msg': 'Registration success!'}), 201
    else:
        return jsonify({'success': False, 'msg': 'Username existed!'}), 403
```

**注册流程**:
1. 检查站点 ID 是否已被注册
2. 验证站点账号和密钥
3. 检查用户名是否已存在
4. 使用 bcrypt 哈希密码
5. 创建用户记录

### 6.5 用户登录

[views/user.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/user.py#L41-L67) 实现了用户登录：

```python
@us.route('/login', methods=['POST'])
def log_in():
    username = request.form['username']
    password = request.form['password']

    user = mysql.get_user(username)
    if user:
        if not user['enable']:
            return jsonify({'success': False, 'msg': 'User has been banned! Please contact administrator.'}), 403

        # 检查 TJUPT 账号状态
        if user['tjupt_id']:
            user_active = check_id_tjupt(user['tjupt_id'])
            if not user_active:
                mysql.ban_user(user['id'])
                return jsonify({'success': False, 'msg': 'User has been banned! Please contact administrator.'}), 403

        # 验证密码
        if bcrypt.checkpw(password.encode('utf-8'), user['passhash'].encode('utf-8')):
            return jsonify({'success': True, 'msg': 'Success~', 'token': User(user).get_auth_token()})
        else:
            return jsonify({'success': False, 'msg': 'Invalid username or password!'}), 403
    else:
        return jsonify({'success': False, 'msg': 'Invalid username or password!'}), 403
```

**登录流程**:
1. 查询用户信息
2. 检查用户是否被封禁
3. 检查站点账号状态
4. 验证密码
5. 生成并返回 Token

### 6.6 账号封禁同步

[views/user.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/user.py#L54-L58) 实现了账号封禁同步：

```python
# 如果用户的TJUPT账户被封禁，则将Reseed账户同时封禁
if user['tjupt_id']:
    user_active = check_id_tjupt(user['tjupt_id'])
    if not user_active:
        mysql.ban_user(user['id'])
        return jsonify({'success': False, 'msg': 'User has been banned! Please contact administrator.'}), 403
```

**功能**: 每次登录时检查站点账号状态，如果被封禁则同步封禁 Reseed 账号

---

## 七、WebSocket 实时通信

### 7.1 Socket.IO 集成

[views/__init__.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/__init__.py#L9) 初始化 Socket.IO：

```python
socketio = SocketIO(cors_allowed_origins='*')
```

[views/__init__.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/__init__.py#L35) 在应用工厂中初始化：

```python
socketio.init_app(app, cors_allowed_origins='*')
```

### 7.2 WebSocket 事件处理

#### 7.2.1 文件匹配事件

[views/reseed.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/reseed.py#L42-L61) 处理文件匹配请求：

```python
@socketio.on('file')
@authenticated_only
def find_torrents_by_file_socket(files: dict):
    def send_result(torrent):
        for t in chain(torrent['cmp_warning'], torrent['cmp_success']):
            torrents = find_torrents_by_id(t['id'])
            t['sites'] = ",".join(["{}-{}".format(t['site'], t['sid']) for t in torrents])
        torrent['cmp_success'] = list(filter(lambda k: k['sites'] != '', torrent['cmp_success']))
        torrent['cmp_warning'] = list(filter(lambda k: k['sites'] != '', torrent['cmp_warning']))
        emit('reseed result', torrent, json=True)

    file_hash = hashlib.md5(json.dumps(files).encode('utf-8')).hexdigest()
    cache = mysql.get_result_cache(file_hash)
    if cache is not None:
        result = json.loads(cache)
        for torrent in result:
            send_result(torrent)
    else:
        result = []
        for name, file in files.items():
            torrent = compare_torrents(name, file)
            result.append(torrent)
            send_result(torrent)
        mysql.record_upload_data(current_user.id, file_hash, json.dumps(result),
                                 socketio.server.environ[request.sid]['HTTP_X_REAL_IP'])
```

**处理流程**:
1. 接收文件索引数据
2. 计算文件哈希（用于缓存）
3. 检查缓存
4. 逐个比较种子
5. 实时推送匹配结果

#### 7.2.2 ID 查询事件

[views/reseed.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/reseed.py#L63-L66) 处理种子 ID 查询：

```python
@socketio.on('tid')
@authenticated_only
def find_torrents_by_id_socket(tid):
    emit('reseed result', find_torrents_by_id(tid), json=True)
```

#### 7.2.3 Hash 查询事件

[views/reseed.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/reseed.py#L68-L73) 处理种子 Hash 查询：

```python
@socketio.on('hash')
@authenticated_only
def find_torrents_by_hash_socket(hex_info_hash):
    emit('reseed result', find_torrents_by_hash(hex_info_hash), json=True)
```

### 7.3 实时推送优势

1. **即时反馈**: 匹配结果实时推送到前端
2. **减少轮询**: 避免前端频繁查询
3. **提升体验**: 用户可以看到匹配进度
4. **降低负载**: 减少 HTTP 请求次数

---

## 八、限流与缓存机制

### 8.1 速率限制

[views/__init__.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/__init__.py#L10) 初始化限流器：

```python
limiter = Limiter(key_func=lambda: current_user.id)
```

[views/__init__.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/__init__.py#L36) 在应用工厂中初始化：

```python
limiter.init_app(app)
```

#### 8.1.1 限流规则

[views/reseed.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/reseed.py#L30-L31) 定义限流规则：

```python
@reseed.route('/upload_json', methods=['POST'])
@login_required
@limiter.limit('10/day;5/hour')
def upload_file():
    # 每天 10 次，每小时 5 次
```

**限流策略**:
- 基于用户 ID 限流
- 多级限流：每天 10 次，每小时 5 次
- 超出限制返回 429 状态码

### 8.2 缓存机制

#### 8.2.1 Redis 缓存

**配置**:
```python
REDIS_URL = "redis://localhost:6379/1"
REDIS_TTL = 2 * 24 * 60 * 60  # 2 days
```

#### 8.2.2 种子信息缓存

[utils/torrent_compare.py](file:///home/incast/PT-Forward/examples/Reseed-backend/utils/torrent_compare.py#L13-L16) 缓存种子信息：

```python
torrents = redis.get(name)
if not torrents:
    torrents = mysql.select_torrent(name)
else:
    torrents = json.loads(str(torrents, encoding='utf-8'))
```

#### 8.2.3 查询结果缓存

[views/reseed.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/reseed.py#L33-L37) 缓存查询结果：

```python
file_hash = hashlib.md5(json.dumps(t_json['result']).encode('utf-8')).hexdigest()
cache = mysql.get_result_cache(file_hash)
if cache is not None:
    result = json.loads(cache)
else:
    # 执行查询
    mysql.record_upload_data(current_user.id, file_hash, json.dumps(result), request.remote_addr)
```

[utils/database.py](file:///home/incast/PT-Forward/examples/Reseed-backend/utils/database.py#L26-L29) 获取缓存：

```python
def get_result_cache(self, hash):
    cache = self.exec(
        '''SELECT `result` FROM `historys` WHERE `hash` = %s AND TIMESTAMPDIFF(HOUR, `time`, NOW()) < 24 ORDER BY `time` DESC''',
        (hash,))
    return cache[0]['result'] if cache else None
```

**缓存策略**:
- 基于 MD5 哈希的缓存键
- 24 小时有效期
- MySQL 存储
- 支持重复查询

#### 8.2.4 站点记录缓存

[views/reseed.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/reseed.py#L75-L80) 缓存站点记录：

```python
def find_torrents_by_id(tid):
    cache = redis.get(tid)
    if cache:
        result = json.loads(str(cache, encoding='utf-8'))
    else:
        result = mysql.find_torrents_by_id(tid)
        redis.set(tid, json.dumps(result), current_app.config.get('REDIS_TTL', 2 * 24 * 60 * 60))
    return result
```

#### 8.2.5 Hash 到 ID 映射缓存

[views/reseed.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/reseed.py#L82-L88) 缓存 Hash 映射：

```python
def find_torrents_by_hash(hex_info_hash):
    cache = redis.get('torrent_hash_{}'.format(hex_info_hash))
    if cache:
        tid = cache
    else:
        tid = mysql.find_tid_by_hash(hex_info_hash)
        redis.set('torrent_hash_{}'.format(hex_info_hash), tid,
                  current_app.config.get('REDIS_TTL', 2 * 24 * 60 * 60))
    return find_torrents_by_id(tid)
```

### 8.3 缓存层次

```
第一层：Redis 缓存
  ├─ 种子信息（按名称）
  ├─ 站点记录（按 ID）
  └─ Hash 映射（按 Hash）

第二层：MySQL 缓存
  └─ 查询历史（按 Hash）
```

---

## 九、脚本工具与自动化功能

### 9.1 磁盘索引工具

[scripts/reseed.py](file:///home/incast/PT-Forward/examples/Reseed-backend/scripts/reseed.py#L8-L31) 实现了磁盘索引功能：

```python
def walk_dir(root_path):
    result = {}
    for file in os.listdir(root_path):
        print("正在索引：{}".format(file))
        path = os.sep.join([root_path, file])
        if os.path.isfile(path):
            try:
                result[file] = os.path.getsize(path)
            except FileNotFoundError:
                print("文件[{}]存在但读取时出现错误，本次索引可能不完整，请检查此文件详情后重试~".format(path))
                continue
        else:
            per_torrent = {}
            for root, dirs, file_list in os.walk(path):
                for filename in file_list:
                    apath = os.sep.join([root, filename])
                    try:
                        alength = os.path.getsize(apath)
                    except (FileNotFoundError, OSError):
                        print("文件[{}]存在但读取时出现错误，本次索引可能不完整，请检查此文件详情后重试~".format(apath))
                        continue
                    per_torrent[apath.replace(path + os.sep, '')] = alength
            result[file] = per_torrent
    return {'base_dir': root_path, 'result': result}
```

**索引策略**:
1. 遍历根目录
2. 区分文件和目录
3. 文件：记录大小
4. 目录：递归遍历，记录相对路径和大小
5. 错误处理：跳过无法访问的文件

**输出格式**:
```json
{
  "base_dir": "/path/to/downloads",
  "result": {
    "single_file.torrent": 123456789,
    "multi_file_dir": {
      "file1.mkv": 123456789,
      "file2.mkv": 987654321
    }
  }
}
```

#### 9.1.1 命令行接口

[scripts/reseed.py](file:///home/incast/PT-Forward/examples/Reseed-backend/scripts/reseed.py#L34-L61) 提供了命令行接口：

```python
@click.command()
@click.argument('path')
@click.option('--save-dir', default=os.getcwd(), help='索引文件保存的路径，默认为当前文件夹')
def main(path, save_dir):
    """
    PATH: 需要索引的路径，举例：/home/xxx/downloads/, D:\\\\Downloads
    """
    if not os.access(save_dir, os.W_OK):
        print("保存路径[{}]不可写入，请提升权限或更改目录！".format(save_dir))
        return
    elif not os.access(path, os.R_OK):
        print("索引路径[{}]不可读取，请检查路径是否正确！".format(path))
        return
    else:
        print("开始索引，时间根据路径下文件零散程度不等，请耐心等待...")

    result = walk_dir(path)

    rstr = r"[\/\\\:\*\?\"\<\>\|]"
    path = re.sub(rstr, "_", path)
    result_file = '{}.json'.format(os.path.join(save_dir, path))
    with open(result_file, 'w')as f:
        f.write(json.dumps(result))
    print("成功！保存路径：{}".format(result_file))
```

**使用方法**:
```bash
python scripts/reseed.py /path/to/downloads --save-dir ./output
```

### 9.2 6v 站点种子下载工具

[scripts/6v.py](file:///home/incast/PT-Forward/examples/Reseed-backend/scripts/6v.py#L22-L91) 实现了 6v 站点的种子下载：

```python
def download_torrent(sid):
    if os.path.exists(DOWNLOAD_DIR + '/{}.torrent'.format(sid)):
        return "存在种子文件，跳过：{}".format(sid)

    try:
        torrent_page = requests.get(BASE_URL + 'thread-{}-1-1.html'.format(sid),
                                    cookies=parse_cookies(COOKIES), headers=HEADERS, timeout=30)
    except requests.RequestException:
        return "获取详情页错误，种子ID：{}".format(sid)
    if 'member.php' in torrent_page.url:
        return "Cookies过期，请手动更换Cookies", -1

    torrent_page.encoding = "gbk"
    torrent_page_bs = BeautifulSoup(torrent_page.text, "lxml")

    title = torrent_page_bs.title.get_text()

    if not re.search("(提示信息|我关注的)", title):
        if torrent_page_bs.find("img", src="static/image/filetype/torrent.gif"):
            download_link = ""
            for attnm in torrent_page_bs.find_all('p', {"class": "attnm"}):
                if '.torrent' in attnm.find('a').text:
                    download_link = BASE_URL + attnm.find('a').get('href')
            if not download_link:
                for attachment in torrent_page_bs.find_all(
                        'a', {'href': re.compile('^forum.php\\?mod=attachment.*')}):
                    if '.torrent' in attachment.text:
                        download_link = BASE_URL + attachment.get('href')
            resp = requests.get(download_link, cookies=parse_cookies(COOKIES), timeout=60, headers=HEADERS)
            if resp.headers.get('Content-Type') != 'application/x-bittorrent':
                _bs = BeautifulSoup(resp.text, 'lxml')
                dl_redirect_link = _bs.find("p", {"class": "alert_btnleft"}).find('a').get('href')
                resp = requests.get(BASE_URL + dl_redirect_link, cookies=parse_cookies(COOKIES),
                                    timeout=60, headers=HEADERS)

            with open(DOWNLOAD_DIR + '/{}.torrent'.format(sid), 'wb') as fp:
                fp.write(resp.content)
            return "下载成功：{}".format(sid)

        else:
            return "非种子/试种区/无权限：{}".format(sid)
    else:
        return "种子不存在：{}".format(sid)
```

**下载流程**:
1. 检查文件是否已存在
2. 获取种子详情页
3. 检查 Cookies 是否过期
4. 解析页面获取下载链接
5. 处理重定向
6. 下载种子文件

---

## 十、配置管理

### 10.1 配置文件结构

[config/config.py](file:///home/incast/PT-Forward/examples/Reseed-backend/config/config.py#L1-L26) 定义了配置项：

```python
# Database
MYSQL_DATABASE_HOST = 'localhost'
MYSQL_DATABASE_PORT = 3306
MYSQL_DATABASE_USER = ''
MYSQL_DATABASE_PASSWORD = ''
MYSQL_DATABASE_DB = 'reseed'
MYSQL_DATABASE_CHARSET = "utf8"
MYSQL_USE_UNICODE = True

# Redis
REDIS_URL = "redis://localhost:6379/1"
REDIS_TTL = 2 * 24 * 60 * 60  # 2 days

# Flask
SECRET_KEY = b''
REMEMBER_COOKIE_NAME = 'reseed'
REMEMBER_COOKIE_DURATION = 86400

# Site
TJUPT_TOKEN = ""
TJUPT_SECRET = ""

OURBITS_TOKEN = ""
OURBITS_SECRET = ""

HDCHINA_APIKEY = ""
```

### 10.2 配置加载

[views/__init__.py](file:///home/incast/PT-Forward/examples/Reseed-backend/views/__init__.py#L20) 从配置文件加载：

```python
app.config.from_pyfile('config.py')
```

---

## 十一、项目启动流程

### 11.1 应用启动

[app.py](file:///home/incast/PT-Forward/examples/Reseed-backend/app.py#L18-L21) 启动应用：

```python
if __name__ == '__main__':
    socketio.run(app)
```

### 11.2 启动流程

```
app.py
  ↓
create_app()
  ↓
加载配置 (config.py)
  ↓
注册 Blueprint
  ├─ user
  ├─ reseed
  └─ plugin
  ↓
初始化扩展
  ├─ CORS
  ├─ SocketIO
  ├─ Limiter
  ├─ LoginManager
  ├─ MySQL
  └─ Redis
  ↓
socketio.run(app)
  ↓
启动 WebSocket 服务器
```

---

## 十二、安全分析

### 12.1 认证安全

**优点**:
- 使用 bcrypt 哈希密码
- Token 包含签名验证
- Token 有过期时间
- 站点账号状态同步

**不足**:
- Token 在 URL 中传输（可能被日志记录）
- 没有 Token 刷新机制
- 没有 IP 限制
- 没有 CSRF 保护

### 12.2 密码安全

**优点**:
- 使用 bcrypt 哈希
- 自动生成盐值

**不足**:
- 没有密码强度要求
- 没有密码重置功能

### 12.3 API 安全

**优点**:
- 站点认证使用签名
- 速率限制防止滥用
- 错误信息不泄露敏感信息

**不足**:
- 没有 HTTPS 强制
- 没有请求签名
- 没有重放攻击防护

### 12.4 数据安全

**优点**:
- 用户数据隔离
- 历史记录包含 IP

**不足**:
- 没有数据加密
- 没有数据备份
- 没有审计日志

---

## 十三、性能优化

### 13.1 缓存策略

1. **Redis 缓存**:
   - 种子信息缓存
   - 站点记录缓存
   - Hash 映射缓存
   - Token 缓存

2. **MySQL 缓存**:
   - 查询历史缓存
   - 24 小时有效期

### 13.2 数据库优化

1. **索引**:
   - `torrents.name`: 种子名称索引
   - `torrent_records.tid`: 种子 ID 索引
   - `torrent_records.site`: 站点索引
   - `historys.hash`: 哈希索引
   - `historys.uid`: 用户 ID 索引

2. **存储引擎**:
   - `torrents`: MyISAM（读多写少）
   - 其他表: InnoDB（事务支持）

### 13.3 网络优化

1. **连接池**: Flask-MySQL 自动管理
2. **超时设置**: 所有 HTTP 请求都有超时
3. **批量处理**: 支持批量上传索引

---

## 十四、项目优缺点分析

### 14.1 优点

#### 14.1.1 架构设计
- ✅ 前后端分离
- ✅ Blueprint 模块化
- ✅ 应用工厂模式
- ✅ 清晰的分层架构

#### 14.1.2 功能实现
- ✅ 多站点支持
- ✅ 实时匹配推送
- ✅ 智能缓存机制
- ✅ 用户认证系统
- ✅ 速率限制

#### 14.1.3 性能优化
- ✅ Redis 缓存
- ✅ 数据库索引
- ✅ 查询结果缓存
- ✅ 连接池管理

#### 14.1.4 代码质量
- ✅ 错误处理完善
- ✅ 日志记录
- ✅ 注释清晰
- ✅ 代码结构清晰

### 14.2 缺点

#### 14.2.1 安全问题
- ❌ Token 在 URL 中传输
- ❌ 没有 HTTPS 强制
- ❌ 没有 CSRF 保护
- ❌ 没有 IP 限制
- ❌ 没有请求签名

#### 14.2.2 功能缺失
- ❌ 没有密码重置
- ❌ 没有邮件验证
- ❌ 没有管理后台
- ❌ 没有统计功能
- ❌ 没有日志查看

#### 14.2.3 可靠性问题
- ❌ 没有健康检查
- ❌ 没有监控告警
- ❌ 没有数据备份
- ❌ 没有故障恢复

#### 14.2.4 用户体验
- ❌ 错误提示不够友好
- ❌ 没有进度显示（部分）
- ❌ 没有批量操作
- ❌ 没有导出功能

---

## 十五、改进建议

### 15.1 短期改进

#### 15.1.1 增强 Token 安全
```python
# 使用 JWT 替代自定义 Token
from flask_jwt_extended import JWTManager, create_access_token

jwt = JWTManager(app)

# 生成 JWT Token
access_token = create_access_token(identity=user['username'])

# 验证 JWT Token
@jwt_required()
def protected_route():
    current_user = get_jwt_identity()
```

#### 15.1.2 添加 HTTPS 强制
```python
from flask_talisman import Talisman

Talisman(app, force_https=True)
```

#### 15.1.3 添加 CSRF 保护
```python
from flask_wtf.csrf import CSRFProtect

csrf = CSRFProtect(app)
```

### 15.2 中期改进

#### 15.2.1 添加管理后台
- 用户管理
- 站点管理
- 统计分析
- 日志查看

#### 15.2.2 添加监控告警
- Prometheus 指标
- 健康检查端点
- 错误率监控
- 性能监控

#### 15.2.3 添加数据备份
- 定期数据库备份
- Redis 持久化
- 备份验证

### 15.3 长期改进

#### 15.3.1 分布式部署
- 负载均衡
- Redis 集群
- 数据库主从
- 容器化部署

#### 15.3.2 智能推荐
- 基于历史数据推荐
- 个性化排序
- 智能过滤

---

## 十六、总结

Reseed-backend 是一个功能完善、设计良好的跨站辅种工具后端。项目采用了合理的架构设计和现代化的技术栈，具有良好的可扩展性。

### 16.1 核心优势
1. **架构清晰**: 前后端分离，模块化设计
2. **实时通信**: WebSocket 实时推送匹配结果
3. **缓存优化**: Redis + MySQL 多层缓存
4. **多站点支持**: 支持多个 PT 站点
5. **智能匹配**: 文件级匹配算法

### 16.2 主要不足
1. **安全加固**: Token 机制需要改进
2. **功能完善**: 缺少管理后台和统计功能
3. **可靠性**: 缺少监控和告警
4. **用户体验**: 部分功能可以优化

### 16.3 适用场景
- PT 站点用户跨站辅种
- 种子资源管理
- 文件索引和搜索

### 16.4 学习价值
Reseed-backend 项目是一个很好的 Python Web 开发学习案例，涵盖了：
- Flask 框架应用
- WebSocket 实时通信
- 数据库设计和优化
- 缓存策略
- 用户认证和授权
- 站点 API 集成
- 种子匹配算法

通过分析这个项目，可以学到如何设计一个功能完整、性能优化的 Web 后端应用。

---

## 附录

### A. 文件结构

```
Reseed-backend/
├── app.py                          # 应用入口
├── config/
│   └── config.py                  # 配置文件
├── models/
│   └── user.py                    # 用户模型
├── views/
│   ├── __init__.py                # 应用工厂
│   ├── user.py                    # 用户路由
│   ├── reseed.py                  # 辅种路由
│   └── plugin.py                  # 插件路由
├── utils/
│   ├── __init__.py                # 工具包
│   ├── database.py                # 数据库操作
│   ├── torrent_compare.py         # 种子比较
│   └── sites/
│       ├── __init__.py
│       ├── tjupt.py               # TJUPT 站点
│       ├── ourbits.py             # OurBits 站点
│       └── hdchina.py             # HDChina 站点
├── scripts/
│   ├── reseed.py                  # 磁盘索引工具
│   └── 6v.py                      # 6v 站点下载工具
├── _db/
│   └── reseed.sql                 # 数据库结构
├── uwsgi/
│   └── uwsgi.example.ini          # uWSGI 配置
├── requirements.txt                # Python 依赖
├── README.md                       # 英文说明
├── README.zh.md                    # 中文说明
└── LICENSE                         # 许可证
```

### B. API 接口规范

#### B.1 用户注册

**请求**:
```http
POST /signup
Content-Type: application/x-www-form-urlencoded

username=test&password=123456&site=tjupt&id=12345&passkey=abcdef
```

**响应**:
```json
{
  "success": true,
  "msg": "Registration success!"
}
```

#### B.2 用户登录

**请求**:
```http
POST /login
Content-Type: application/x-www-form-urlencoded

username=test&password=123456
```

**响应**:
```json
{
  "success": true,
  "msg": "Success~",
  "token": "base64_encoded_token"
}
```

#### B.3 上传索引文件

**请求**:
```http
POST /upload_json
Authorization: Bearar base64_encoded_token
Content-Type: multipart/form-data

file=@index.json&sites=tjupt,ourbits
```

**响应**:
```json
{
  "success": true,
  "base_dir": "/path/to/downloads",
  "result": [
    {
      "name": "torrent_name",
      "cmp_success": [
        {
          "id": 123,
          "sites": "tjupt-456,ourbits-789"
        }
      ],
      "cmp_warning": []
    }
  ]
}
```

#### B.4 获取站点信息

**请求**:
```http
GET /sites_info
Authorization: Bearar base64_encoded_token
```

**响应**:
```json
[
  {
    "name": "tjupt",
    "base_url": "https://tjupt.org",
    "_enable": false,
    "passkey": ""
  }
]
```

### C. WebSocket 事件

#### C.1 文件匹配

**发送**:
```json
{
  "event": "file",
  "data": {
    "torrent1": 123456789,
    "torrent2": {
      "file1.mkv": 123456789,
      "file2.mkv": 987654321
    }
  }
}
```

**接收**:
```json
{
  "event": "reseed result",
  "data": {
    "name": "torrent1",
    "cmp_success": [
      {
        "id": 123,
        "sites": "tjupt-456,ourbits-789"
      }
    ],
    "cmp_warning": []
  }
}
```

#### C.2 ID 查询

**发送**:
```json
{
  "event": "tid",
  "data": 123
}
```

**接收**:
```json
{
  "event": "reseed result",
  "data": [
    {
      "site": "tjupt",
      "sid": 456
    }
  ]
}
```

### D. 数据库表关系

```
users (用户表)
  ├─ tjupt_id → sites (站点表)
  └─ ourbits_id → sites (站点表)

torrents (种子表)
  └─ id → torrent_records.tid (种子记录表)

torrent_records (种子记录表)
  ├─ tid → torrents.id (种子表)
  └─ site → sites.site (站点表)

historys (历史记录表)
  └─ uid → users.id (用户表)
```

### E. 相关资源

- **GitHub**: https://github.com/tongyifan/Reseed-backend
- **Wiki**: https://github.com/tongyifan/Reseed-backend/wiki
- **作者**: tongyifan
- **Telegram**: @tongyifan

---

**分析完成时间**: 2026-04-11  
**分析工具**: Trae IDE  
**分析深度**: 全面深度分析
