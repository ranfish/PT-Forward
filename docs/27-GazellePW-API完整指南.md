# GazellePW 公开 API 完整参考指南

> **文档版本**: v1.0  
> **最后更新**: 2026-04-12  
> **数据来源**: [examples/GazellePW](file:///home/incast/PT-Forward/examples/GazellePW/) 源码深度分析  
> **架构类型**: Gazelle (音乐PT站点专用架构)  
> **分析文件数**: 11个API模块 + 路由/认证/数据库

---

## 目录

1. [架构概述](#1-架构概述)
2. [认证机制](#2-认证机制)
3. [统一响应格式](#3-统一响应格式)
4. [路由分发系统](#4-路由分发系统)
5. [核心API端点详解](#5-核心api端点详解)
6. [数据模型定义](#6-数据模型定义)
7. [错误处理机制](#7-错误处理机制)
8. [与其他架构对比](#8-与其他架构对比)
9. [最佳实践与示例代码](#9-最佳实践与示例代码)

---

## 1. 架构概述

### 1.1 Gazelle 架构特点

Gazelle 是专门为**音乐PT站点**设计的架构：

| 特性 | **GazellePW** | **NexusPHP** | **M-Team (mTorrent)** |
|------|---------------|--------------|----------------------|
| **设计初衷** | 音乐分享（专辑/艺术家） | 通用PT站点 | 现代化通用PT |
| **数据模型** | Group + Torrent 双层结构 | 单层 Torrent 结构 | 单层 + 元数据丰富 |
| **核心概念** | Artist → ReleaseGroup → Torrents | Category → Torrent | Category → Torrent |
| **API成熟度** | 中等（11个基础端点） | 高（RESTful 1.9+） | 非常高（OpenAPI 3.1） |
| **认证方式** | API Key (Token) | Cookie/Bearer/Passkey | x-api-key Header |

### 1.2 目录结构

```
examples/GazellePW/
├── public/api.php              # API 入口 ⭐
├── app/API/                    # API 模块目录 ⭐
│   ├── AbstractAPI.php         # 抽象基类
│   ├── Torrent.php             # 种子查询
│   ├── User.php                # 用户管理
│   ├── Artist.php              # 艺术家信息
│   ├── Request.php             # 种子请求
│   ├── Forum.php               # 论坛帖子
│   ├── Collage.php             # 合辑
│   ├── Wiki.php                # Wiki文章
│   ├── MovieInfo.php           # 电影信息
│   ├── Upload.php              # 上传种子
│   ├── GenerateInvite.php      # 邀请码
│   └── ImgUpload.php           # 图片上传
├── sections/api/index.php      # 路由分发器
└── gazelle.sql                 # 数据库Schema
```

---

## 2. 认证机制

### 2.1 API Key 认证流程

#### 数据库表结构 ([gazelle.sql:25-31](file:///home/incast/PT-Forward/examples/GazellePW/gazelle.sql#L25-L31)):

```sql
CREATE TABLE `api_applications` (
  `ID` int(10) NOT NULL AUTO_INCREMENT,
  `UserID` int(10) NOT NULL,
  `Token` char(32) NOT NULL,        -- API Key (32字符)
  `Name` varchar(50) NOT NULL,      -- 应用名称
  PRIMARY KEY (`ID`)
);
```

#### 认证实现 ([public/api.php:35-55](file:///home/incast/PT-Forward/examples/GazellePW/public/api.php#L35-L55)):

```php
if (empty($_GET['api_key'])) {
    json_error('invalid parameters');
}

$token = $_GET['api_key'];

// 从缓存或数据库验证Token
$app = $Cache->get_value("api_apps_{$token}");
if (!is_array($app)) {
    $DB->prepared_query("
        SELECT Token, Name, UserID
        FROM api_applications WHERE Token = ? LIMIT 1", $token);
    
    if ($DB->record_count() === 0) {
        json_error('invalid token');  // ❌ 无效Token
    }
    
    $app = $DB->to_array(false, MYSQLI_ASSOC);
    G::$Cache->cache_value("api_apps_{$token}", $app);  // 缓存结果
}

// 基于Token创建者加载用户权限
$LoggedUser = array_merge(
    Users::user_heavy_info($app['UserID']),
    Users::user_info($app['UserID']),
    Users::user_stats($app['UserID'])
);
G::$LoggedUser = &$LoggedUser;
```

### 2.2 认证参数说明

| 参数名 | 位置 | 类型 | 必填 | 说明 |
|--------|------|------|------|------|
| `api_key` | Query String | string(32) | ✅ | API应用Token |
| `action` | Query String | string | ✅ | 要调用的API模块名称 |

**完整请求示例**:
```
GET /api.php?action=torrent&req=torrent&torrent_id=12345&api_key=abcdefghijklmnopqrstuvwxyz123456
```

---

## 3. 统一响应格式

### 3.1 成功响应

```json
{
    "status": 200,
    "response": { /* 具体数据 */ },
    "info": {
        "source": "GazellePW",
        "version": 1
    }
}
```

### 3.2 错误响应

**重要发现**: 错误字段名为 `"erro"` (拼写错误)，不是 `"error"`！

**实现代码** ([classes/util.php:152-155](file:///home/incast/PT-Forward/examples/GazellePW/classes/util.php#L152-L155)):

```php
function json_error($Message) {
    echo json_encode(add_json_info([
        'status' => 'failure',
        'erro' => $Message,          // ⚠️ 注意拼写错误
        'response' => []
    ]));
    die();
}
```

**错误响应示例**:
```json
{
    "status": "failure",
    "erro": "invalid parameters",
    "response": [],
    "info": { "source": "GazellePW", "version": 1 }
}
```

### 3.3 调试信息 (权限用户可见)

当用户有 `site_debug` 权限时，响应会包含额外的调试信息：

```json
{
    "status": 200,
    "response": { ... },
    "info": { "source": "GazellePW", "version": 1 },
    "debug": {
        "queries": ["SELECT ..."],
        "searches": ["SELECT ..."]
    }
}
```

---

## 4. 路由分发系统

### 4.1 可用Action列表

**定义位置** ([public/api.php:26-37](file:///home/incast/PT-Forward/examples/GazellePW/public/api.php#L26-L37)):

```php
$available = [
    'generate_invite',   // 生成邀请码
    'user',              // 用户管理
    'wiki',              // Wiki文章
    'forum',             // 论坛
    'request',           // 种子请求
    'artist',            // 艺术家
    'collage',           // 合辑
    'torrent',           // 种子查询
    'upload',            // 上传种子
    'movie_info',        // 电影信息
    'img_upload'         // 图片上传
];
```

### 4.2 类名映射规则

**动态类加载** ([sections/api/index.php:1-7](file:///home/incast/PT-Forward/examples/GazellePW/sections/api/index.php#L1-L7)):

```php
function getClassObject($name, $twig, $config) {
    // 将 action 名称转换为 PascalCase 类名
    $name = "Gazelle\\API\\" . str_replace("_", "", ucwords($name, "_"));
    return new $name($twig, $config);
}

// 映射示例：
// "torrent"        → Gazelle\API\Torrent
// "generate_invite"→ Gazelle\API\GenerateInvite
// "img_upload"     → Gazelle\API\ImgUpload
```

### 4.3 执行流程

```php
$config = [
    'Categories' => $Categories,
    'CollageCats' => $CollageCats,
    'ReleaseTypes' => $ReleaseTypes,
];

$class = getClassObject($_GET['action'], G::$Twig, $config);
$response = $class->run();
print(json_encode(['status' => 200, 'response' => $response], JSON_UNESCAPED_SLASHES));
```

---

## 5. 核心API端点详解

### 5.1 种子模块 (Torrent)

**文件**: [app/API/Torrent.php](file:///home/incast/PT-Forward/examples/GazellePW/app/API/Torrent.php)

#### 端点概览

| Req参数 | 功能 | 说明 |
|---------|------|------|
| `torrent` (默认) | 获取种子详情 | 支持按ID或IMDB查询 |
| `group` | 获取发布组详情 | 包含艺术家信息 |

#### 获取种子详情 (req=torrent)

**请求参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `action` | string | ✅ | `torrent` |
| `req` | string | 否 | `torrent` (默认) |
| `torrent_id` | integer | 二选一 | 种子ID |
| `imdbID` | string | 二选一 | IMDB ID (`tt1234567`) |

**SQL查询逻辑** ([app/API/Torrent.php:22-44](file:///home/incast/PT-Forward/examples/GazellePW/app/API/Torrent.php#L22-L44)):

```php
$this->db->prepared_query("
    SELECT
        tg.ID,                          -- 发布组ID
        tg.Name,                        -- 名称
        tg.Year,                        -- 年份
        tg.ReleaseType AS ReleaseTypeID,-- 发布类型
        t.Codec,                        -- 编码 (FLAC/MP3)
        t.RemasterTitle,                -- 再版标题
        t.Resolution,                   -- 分辨率 (CD/Vinyl)
        t.Container,                    -- 容器
        t.Processing,                   -- 处理方式
        t.Source,                       -- 来源
        t.Snatched,                     -- 完成次数
        t.Seeders,                      -- 做种数
        t.Leechers                      -- 下载数
    FROM torrents AS t
    INNER JOIN torrents_group AS tg ON (tg.ID = t.GroupID)
    WHERE t.ID = ?", $_GET['torrent_id']);
```

**响应示例**:
```json
{
    "status": 200,
    "response": [{
        "ID": "12345",
        "Name": "Artist - Album [2015] [Flac]",
        "Year": "2015",
        "ReleaseTypeID": "1",
        "Codec": "FLAC",
        "Source": "CD",
        "Snatched": "1500",
        "Seeders": "23",
        "Leechers": "5"
    }]
}
```

#### 获取发布组详情 (req=group)

**特殊处理** - 自动加载艺术家信息:

```php
$artists = \Artists::get_artist($group['ID']);
$group['Artists'] = $artists;
$group['DisplayArtists'] = \Artists::display_artists($artists, false, false, false);
```

---

### 5.2 用户模块 (User)

**文件**: [app/API/User.php](file:///home/incast/PT-Forward/examples/GazellePW/app/API/User.php)

#### 端点概览

| Req参数 | 功能 | 权限要求 |
|---------|------|----------|
| `stats` (默认) | 获取用户详细信息 | 普通用户 |
| `enable` | 启用被禁用用户 | 管理员 |
| `disable` | 禁用用户 | 管理员 |

#### 获取用户信息 (req=stats)

**请求参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `action` | string | ✅ | `user` |
| `user_id` | integer | 二选一 | 用户ID |
| `username` | string | 二选一 | 用户名 |

**SQL查询** ([app/API/User.php:48-72](file:///home/incast/PT-Forward/examples/GazellePW/app/API/User.php#L48-L72)):

```php
$this->db->prepared_query("
    SELECT
        um.ID, um.Username, um.Enabled,
        um.IRCKey,
        um.Uploaded, um.Downloaded,
        um.PermissionID AS Class,
        um.Paranoia,            -- 隐私设置（序列化数组）
        um.BonusPoints,
        p.Name as ClassName,
        p.Level,
        GROUP_CONCAT(ul.PermissionID SEPARATOR ',') AS SecondaryClasses
    FROM users_main AS um
    INNER JOIN users_info AS ui ON (ui.UserID = um.ID)
    INNER JOIN permissions AS p ON (p.ID = um.PermissionID)
    LEFT JOIN users_levels AS ul ON (ul.UserID = um.ID)
    WHERE {$where}", $param);
```

**隐私设置处理**:
```php
$user['Paranoia'] = unserialize_array($user['Paranoia']);
$user['Ratio'] = \Format::get_ratio($user['Uploaded'], $user['Downloaded']);

// 根据隐私设置隐藏敏感数据
foreach (['Downloaded', 'Uploaded', 'Ratio'] as $key) {
    if (in_array(strtolower($key), $user['Paranoia'])) {
        $user['DisplayStats'][$key] = "Hidden";  // 🔒 隐藏
    }
}
```

**响应示例**:
```json
{
    "status": 200,
    "response": {
        "ID": 12345,
        "Username": "MusicLover99",
        "Enabled": "1",
        "Uploaded": 1099511627776,
        "Downloaded": 549755813888,
        "Class": 15,
        "ClassName": "Power User",
        "Level": 15,
        "BonusPoints": 25000.50,
        "SecondaryClasses": [22, 30],
        "Ratio": "2.00",
        "DisplayStats": {
            "Downloaded": "512.00 GB",
            "Uploaded": "1.00 TB",
            "Ratio": "2.00"
        },
        "UserPage": "https://gazellepw.example.com/user.php?id=12345"
    }
}
```

#### 启用/禁用用户 (管理员功能)

**禁用用户**:
```php
\Tools::disable_users($this->id, 'Disabled via API', 1);
return ['disabled' => true, 'user_id' => $this->id];
```

**启用用户** (复杂业务逻辑):
```php
// 1. 更新Tracker状态
\Tracker::update_tracker('add_user', ['id' => $this->id, 'passkey' => $passkey]);

// 2. 检查分享率
if ($ratio >= $requiredRatio) {
    // 解除下载限制
    $UpdateSet[] = "um.can_leech = '1'";
} else {
    // 继续限制
    \Tracker::update_tracker('update_user', ['can_leech' => 0]);
}

// 3. 清除BanReason并启用账户
return ['enabled' => true, 'user_id' => $Cur['ID']];
```

---

### 5.3 艺术家模块 (Artist)

**文件**: [app/API/Artist.php](file:///home/incast/PT-Forward/examples/GazellePW/app/API/Artist.php)

**请求参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `action` | string | ✅ | `artist` |
| `artist_id` | integer | ✅ | 艺术家ID |

**响应示例**:
```json
{
    "status": 200,
    "response": {
        "ArtistID": "42",
        "Name": "Radiohead"
    }
}
```

---

### 5.4 请求模块 (Request)

**文件**: [app/API/Request.php](file:///home/incast/PT-Forward/examples/GazellePW/app/API/Request.php)

**请求参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `action` | string | ✅ | `request` |
| `request_id` | integer | ✅ | 请求ID |

**特性**:
- 自动关联艺术家信息
- 分类ID自动转名称

**响应示例**:
```json
{
    "status": 200,
    "response": {
        "ID": "100",
        "Title": "Looking for FLAC version",
        "Category": "Music",
        "Artists": [{"id": "1", "name": "Artist Name"}],
        "DisplayArtists": "Artist Name"
    }
}
```

---

### 5.5 论坛模块 (Forum)

**文件**: [app/API/Forum.php](file:///home/incast/PT-Forward/examples/GazellePW/app/API/Forum.php)

**请求参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `action` | string | ✅ | `forum` |
| `topic_id` | integer | ✅ | 帖子ID |

**SQL查询**:
```sql
SELECT ft.ID, ft.Title, um.Username AS Author, 
       f.Name AS Forum, f.MinClassRead
FROM forums_topics AS ft
INNER JOIN users_main AS um ON um.ID = ft.AuthorID
INNER JOIN forums AS f ON f.ID = ft.ForumID
WHERE ft.ID = ?
```

**响应示例**:
```json
{
    "status": 200,
    "response": {
        "ID": "500",
        "Title": "Discussion about new release",
        "Author": "User123",
        "Forum": "Music Discussion",
        "MinClassRead": "0"
    }
}
```

---

### 5.6 合辑模块 (Collage)

**文件**: [app/API/Collage.php](file:///home/incast/PT-Forward/examples/GazellePW/app/API/Collage.php)

**请求参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `action` | string | ✅ | `collage` |
| `collage_id` | integer | ✅ | 合辑ID |

**响应示例**:
```json
{
    "status": 200,
    "response": {
        "ID": "200",
        "Name": "Best Albums of 2024",
        "CategoryID": "1",
        "Category": "1"
    }
}
```

---

### 5.7 Wiki模块 (Wiki)

**文件**: [app/API/Wiki.php](file:///home/incast/PT-Forward/examples/GazellePW/app/API/Wiki.php)

**请求参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `action` | string | ✅ | `wiki` |
| `wiki_id` | integer | ✅ | Wiki文章ID |

**响应示例**:
```json
{
    "status": 200,
    "response": {
        "Title": "Ripping Guide",
        "MinClassRead": "0",
        "Author": "StaffUser",
        "Date": "2024-01-15 10:30:00"
    }
}
```

---

### 5.8 电影信息模块 (MovieInfo)

**文件**: [app/API/MovieInfo.php](file:///home/incast/PT-Forward/examples/GazellePW/app/API/MovieInfo.php)

**功能**: 通过IMDB ID获取电影信息或检查重复

**请求参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `action` | string | ✅ | `movie_info` |
| `imdbid` | string | ✅ | IMDB ID (`tt1234567`) |

**业务逻辑** ([app/API/MovieInfo.php](file:///home/incast/PT-Forward/examples/GazellePW/app/API/MovieInfo.php)):

```php
public function run() {
    $IMDBID = $_GET['imdbid'];

    if (!preg_match('/^tt\d+$/', $IMDBID)) {
        return ["status" => "error", "message" => "Invalid IMDb ID"];
    }

    // 1. 检查是否已存在该电影的种子
    $this->db->query("select ID from torrents_group where IMDBID='$IMDBID'");
    if ($GroupID) {
        return [
            "status" => "success",
            "message" => ["Dupe" => true, "GroupID" => $GroupID]
        ];  // ⚠️ 重复检测
    }

    // 2. 如果不存在，从外部API获取电影信息
    try {
        $Ret = \MOVIE::get_movie_fill_info($IMDBID, false);
        return ["status" => "success", "response" => $Ret];
    } catch (\Exception $e) {
        return ["status" => "error", "message" => $e->getMessage()];
    }
}
```

**响应示例 (重复)**:
```json
{
    "status": "success",
    "message": {
        "Dupe": true,
        "GroupID": "100"
    }
}
```

**响应示例 (新电影)**:
```json
{
    "status": "success",
    "response": {
        "Title": "Movie Name",
        "Year": "2024",
        "Genre": ["Action", "Drama"],
        "Rating": "8.5",
        "PosterURL": "https://..."
    }
}
```

---

### 5.9 上传模块 (Upload)

**文件**: [app/API/Upload.php](file:///home/incast/PT-Forward/examples/GazellePW/app/API/Upload.php)

**功能**: 通过API上传种子文件

**HTTP方法**: POST  
**Content-Type**: multipart/form-data

**请求参数** (POST body):
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `action` | string | ✅ | `upload` |
| `groupid` | integer | 否 | 已有发布组ID（留空则创建新组） |
| torrent文件 | file | ✅ | .torrent文件 |
| 其他字段 | mixed | 视情况 | 与Web上传表单相同 |

**实现逻辑** ([app/API/Upload.php](file:///home/incast/PT-Forward/examples/GazellePW/app/API/Upload.php)):

```php
class Upload extends AbstractAPI {
    public function run() {
        return $this->uploadTorrent();
    }

    private function uploadTorrent() {
        $IsNewGroup = empty($_POST['groupid']);
        
        // 使用Gazelle内置的Uploader类
        $uploader = new Uploader($IsNewGroup, true);
        
        try {
            $uploadedTorrent = $uploader->uploadTorrent($_POST, $_FILES);
        } catch (InvalidParamException $e) {
            json_error($e->getMessage());  // 参数验证失败
        } catch (\Exception $e) {
            error_log($e->getMessage());
            json_error('internal error');  // 内部错误
        }
        
        return ['torrent_id' => $uploadedTorrent->TorrentID];
    }
}
```

**响应示例**:
```json
{
    "status": 200,
    "response": {
        "torrent_id": "12346"
    }
}
```

---

### 5.10 邀请码模块 (GenerateInvite)

**文件**: [app/API/GenerateInvite.php](file:///home/incast/PT-Forward/examples/GazellePW/app/API/GenerateInvite.php)

**功能**: 为面试通过的用户生成邀请码

**请求参数**:
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `action` | string | ✅ | `generate_invite` |
| `interviewer_id` | integer | 二选一 | 面试官ID |
| `interviewer_name` | string | 二选一 | 面试官用户名 |
| `email` | string | 否 | 被邀请人邮箱（可选） |

**业务逻辑** ([app/API/GenerateInvite.php](file:///home/incast/PT-Forward/examples/GazellePW/app/API/GenerateInvite.php)):

```php
public function run() {
    // 1. 验证面试官身份
    // 2. 检查邮箱是否已被使用
    if ($this->db->scalar("SELECT 1 FROM users_main WHERE Email = ?", $email)) {
        json_error("Email address already in use");
    }
    
    // 3. 生成32位随机邀请码
    $key = randomString();
    
    // 4. 写入数据库（有效期3天）
    $this->db->prepared_query("
        INSERT INTO invites (InviterID, InviteKey, Email, Reason, Expires)
        VALUES (?, ?, ?, ?, now() + INTERVAL 3 DAY)",
        $interviewer_id, $key, $email, "Passed Interview"
    );
    
    // 5. 如果提供了邮箱，发送邀请邮件
    if (!empty($_GET['email'])) {
        $body = $this->twig->render('emails/invite.twig', [...]);
        \Misc::send_email($email, 'New account confirmation...', $body, 'noreply');
    }
    
    return [
        "key" => $key,
        "invite_url" => CONFIG['SITE_URL'] . "/register.php?invite={$key}"
    ];
}
```

**响应示例**:
```json
{
    "status": 200,
    "response": {
        "key": "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6",
        "invite_url": "https://gazellepw.example.com/register.php?invite=a1b2c3d4..."
    }
}
```

---

### 5.11 图片上传模块 (ImgUpload)

**文件**: [app/API/ImgUpload.php](file:///home/incast/PT-Forward/examples/GazellePW/app/API/ImgUpload.php)

**功能**: 批量上传图片到服务器

**HTTP方法**: POST  
**Content-Type**: application/x-www-form-urlencoded

**请求参数** (POST body):
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `action` | string | ✅ | `img_upload` |
| `urls` | array | ✅ | URL数组（要下载的图片地址） |

**实现逻辑** ([app/API/ImgUpload.php](file:///home/incast/PT-Forward/examples/GazellePW/app/API/ImgUpload.php)):

```php
class ImgUpload extends AbstractAPI {
    public function run() {
        if (empty($_POST['urls'])) {
            return ["Error" => "Invalid Request"];
        }

        $urls = $_POST['urls'];
        $Data = [];
        $user_id = $this->user['ID'];

        for ($i = 0; $i < count($urls); $i++) {
            $url = urls[$i];
            $extension = strtolower(end(explode(".", $url)));
            
            // 验证文件扩展名
            if (!\ImageTools::valid_extension($extension)) {
                return ["Error" => "Invalid ext: $extension"];
            }
            
            // 生成存储路径：/user/{user_id}/{date}/{uniqid}.{ext}
            $path = CONFIG['IMAGE_PATH_PREFIX'] . '/user/' . $user_id 
                  . '/' . date('Ymd') . '/' . uniqid() . '.' . $extension;
                  
            $Data[] = ['Url' => $url, 'Name' => $path, 'Ext' => $extension];
        }

        // 批量下载并上传
        $RetPath = \ImageTools::multi_fetch_upload($Data);

        $Ret = [];
        foreach ($RetPath as $RetName) {
            $Ret[] = ['name' => $RetName];
        }

        return ["files" => $Ret];
    }
}
```

**响应示例**:
```json
{
    "status": 200,
    "response": {
        "files": [
            {"name": "/user/123/20260412/abc123.jpg"},
            {"name": "/user/123/20260412/def456.png"}
        ]
    }
}
```

**错误响应**:
```json
{
    "status": 200,
    "response": {
        "Error": "Invalid ext: exe"
    }
}
```

---

## 6. 数据模型定义

### 6.1 核心表结构

#### torrents 表 (种子)

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | int(10) | 种子唯一标识 |
| GroupID | int(10) | 所属发布组ID |
| Codec | varchar(20) | 编码 (FLAC, MP3, AAC) |
| Resolution | varchar(10) | 分辨率/来源 (CD, Vinyl, DVD) |
| Source | varchar(10) | 来源介质 |
| Container | varchar(10) | 容器格式 |
| Processing | enum | 处理方式 |
| RemasterTitle | varchar(80) | 再版标题 |
| Snatched | int(10) | 完成次数 |
| Seeders | int(10) | 做种人数 |
| Leechers | int(10) | 下载人数 |

#### torrents_group 表 (发布组)

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | int(10) | 组ID |
| Name | varchar(300) | 组名称 |
| Year | int(4) | 年份 |
| IMDBID | varchar(20) | IMDB ID |
| ReleaseType | tinyint | 发布类型 (Album, EP, etc.) |
| CategoryID | tinyint | 分类ID |

#### artists_group 表 (艺术家)

| 字段 | 类型 | 说明 |
|------|------|------|
| ArtistID | int(10) | 艺术家ID |
| Name | varchar(200) | 艺术家名称 |

#### api_applications 表 (API应用)

| 字段 | 类型 | 说明 |
|------|------|------|
| ID | int(10) | 自增主键 |
| UserID | int(10) | 创建者用户ID |
| Token | char(32) | API密钥 |
| Name | varchar(50) | 应用名称 |

---

## 7. 错误处理机制

### 7.1 标准错误类型

| 错误消息 | 触发条件 | HTTP状态码 |
|----------|----------|------------|
| `invalid action` | 不支持的action参数 | 200 (JSON内标记failure) |
| `invalid parameters` | 缺少api_key等必填参数 | 200 (JSON内标记failure) |
| `invalid token` | API Key无效或不存在 | 200 (JSON内标记failure) |
| `Missing {resource} id` | 缺少资源ID参数 | 200 (JSON内标记failure) |
| `{Resource} not found` | 资源不存在 | 200 (JSON内标记failure) |
| `Need to supply either user_id or username` | 缺少用户标识 | 200 (JSON内标记failure) |
| `User not found` | 用户不存在 | 200 (JSON内标记failure) |
| `Internal Error` | 服务器内部错误 | 200 (JSON内标记failure) |

### 7.2 错误处理特点

1. **所有错误都返回HTTP 200**，通过 JSON `status` 字段区分成功/失败
2. **字段名拼写错误**: 使用 `"erro"` 而不是 `"error"`
3. **立即终止**: `json_error()` 调用后执行 `die()`，不会继续执行
4. **统一包装**: 所有响应都通过 `add_json_info()` 添加元信息

---

## 8. 与其他架构对比

### 8.1 架构理念对比

| 维度 | **GazellePW** | **NexusPHP** | **M-Team (mTorrent)** |
|------|---------------|--------------|----------------------|
| **设计领域** | 音乐分享为主 | 通用PT站点 | 现代化通用PT |
| **核心概念** | Artist → Group → Torrent | Category → Torrent | Category → Torrent |
| **数据模型** | 双层结构（Group + Torrent） | 单层结构 | 单层 + 丰富元数据 |
| **API数量** | 11个基础端点 | 9个核心 + RESTful扩展 | 363个端点 |
| **API成熟度** | ⭐⭐ 基础 | ⭐⭐⭐⭐ 成熟 | ⭐⭐⭐⭐⭐ 企业级 |
| **认证方式** | API Key (Query参数) | Cookie/Bearer/Passkey | x-api-key (Header) |
| **响应格式** | 自定义JSON | 统一{ret,msg,data} | OpenAPI标准JSON |
| **文档标准** | 无（仅源码） | Apifox社区维护 | OpenAPI 3.1.0官方 |
| **适用场景** | 音乐库、Redacted风格 | 综合性中文PT站 | 云原生现代化PT平台 |

### 8.2 API能力对比矩阵

| 功能域 | **GazellePW** | **NexusPHP** | **M-Team** |
|--------|---------------|--------------|------------|
| **种子搜索** | ❌ 无专用API | ✅ GET /api/torrents | ✅ POST /torrent/search |
| **种子详情** | ✅ GET (by ID/IMDB) | ✅ GET /api/torrent/{id} | ✅ GET /torrent/{id} |
| **发布组查询** | ✅ 支持 | ❌ 无此概念 | ✅ 支持 |
| **用户信息** | ✅ GET (含隐私控制) | ✅ GET /api/user/{id} | ✅ GET /member/profile |
| **用户管理** | ✅ enable/disable | ❌ 无 | ✅ 有限支持 |
| **艺术家查询** | ✅ 专用API | ❌ 无 | ✅ 元数据字段 |
| **种子上传** | ✅ POST (multipart) | ✅ POST /api/torrent | ✅ POST /torrent/upload |
| **论坛访问** | ✅ GET topic | ❌ 无 | ❌ 无 |
| **Wiki访问** | ✅ GET article | ❌ 无 | ❌ 无 |
| **合辑查询** | ✅ GET collage | ❌ 无 | ❌ 无 |
| **求种系统** | ✅ GET request | ❌ 无 | ❌ 无 |
| **邀请管理** | ✅ GenerateInvite | ❌ 无 | ❌ 无 |
| **图片上传** | ✅ 批量URL上传 | ❌ 无 | ❌ 无 |
| **电影信息填充** | ✅ IMDB集成 | ❌ 无 | ✅ 元数据抓取 |
| **辅种支持** | ❌ 无pieces_hash | ✅ 原生支持 | ⚠️ 间接支持 |
| **RSS输出** | ❌ 无API | ✅ Passkey认证 | ✅ API Key认证 |
| **批量操作** | ❌ 有限 | ⚠️ 部分 | ✅ 完善 |
| **速率限制** | ❌ 未实现 | ⚠️ 可配置 | ✅ 内置 |
| **分页支持** | ❌ 无 | ✅ page/per_page | ✅ pageNumber/pageSize |
| **排序过滤** | ❌ 无 | ✅ sorts/filter | ✅ sortField/sortDirection |

### 8.3 认证机制对比

| 特性 | **GazellePW** | **NexusPHP** | **M-Team** |
|------|---------------|--------------|------------|
| **认证位置** | Query String (?api_key=xxx) | Header/Cookie/Header | Header (x-api-key) |
| **Token类型** | char(32) 固定长度 | JWT/Session/Cookie | UUID格式 |
| **Token获取** | 数据库直接创建 | 登录/OAuth/控制台 | 控制台自助创建 |
| **Token存储** | api_applications表 | 多种（session/token表） | 配置/数据库 |
| **缓存策略** | Redis缓存验证结果 | Session缓存 | 内存缓存 |
| **权限继承** | 以创建者身份执行 | 当前登录用户 | 应用级别scopes |
| **多应用支持** | ✅ 每用户可创建多个 | ❌ 单一会话 | ✅ 多Token |
| **Token撤销** | 删除数据库记录 | 过期/手动清除 | 控制台删除 |
| **安全等级** | ⭐⭐ 中等 | ⭐⭐⭐ 较高 | ⭐⭐⭐⭐ 高 |

### 8.4 迁移建议

如果从 Gazelle 迁移到其他架构：

#### Gazelle → NexusPHP

```python
# 认证转换
# Old: ?api_key=xxxxx (Query String)
# New: Cookie: c_secure_uid=xxx 或 Authorization: Bearer xxx

# 种子查询差异
# Gazelle: 双层结构 (Group → Torrents)
# NexusPHP: 单层结构 (Torrent)

# 转换示例
def convert_gazelle_to_nexusphp(gazelle_response):
    """将Gazelle的Group+Torrent结构转为NexusPHP单层"""
    torrents = []
    for group in gazelle_response.get('response', []):
        if isinstance(group, dict) and 'ID' in group:
            # 这是Group，需要展开其下的Torrents
            pass
        else:
            # 这是单个Torrent
            torrents.append({
                'id': group.get('ID'),
                'title': group.get('Name'),
                'seeders': group.get('Seeders'),
                'leechers': group.get('Leechers'),
            })
    return torrents
```

#### Gazelle → M-Team

```python
# 响应格式转换
# Gazelle: {"status": 200, "response": {...}}
# M-Team: {"code": "0", "data": {...}, "message": "success"}

def convert_gazelle_response_to_mteam(gazelle_resp):
    status = gazelle_resp.get('status')
    if status == 200:
        return {
            'code': '0',
            'data': gazelle_resp.get('response', {}),
            'message': 'success'
        }
    else:
        return {
            'code': str(status),
            'message': gazelle_resp.get('erro', 'Unknown error'),  # 注意erro拼写
            'data': None
        }
```

---

## 9. 最佳实践与示例代码

### 9.1 Python完整客户端

```python
"""
GazellePW API 完整客户端
基于源码深度分析实现
"""

import requests
import time
from typing import Optional, Dict, List, Any, Union
from dataclasses import dataclass


@dataclass
class GazelleResponse:
    """Gazelle API 统一响应封装"""
    success: bool
    data: Any
    error: Optional[str] = None
    debug: Optional[Dict] = None


class GazellePWClient:
    """
    GazellePW API 客户端
    
    特点：
    - 支持所有11个API端点
    - 自动处理错误响应（注意"erro"拼写）
    - 支持调试模式
    - 内置速率限制
    """

    BASE_URL = "https://gazellepw.example.com"

    def __init__(self, api_key: str, base_url: str = None, rate_limit: float = 1.0):
        """
        初始化客户端
        
        Args:
            api_key: API应用Token (32字符)
            base_url: 站点基础URL
            rate_limit: 请求间隔（秒）
        """
        self.api_key = api_key
        self.base_url = (base_url or self.BASE_URL).rstrip("/")
        self.rate_limit = rate_limit
        self.session = requests.Session()
        self.session.headers.update({
            'User-Agent': 'GazellePW-API-Client/1.0',
            'Accept': 'application/json',
        })
        self._last_request = 0

    def _request(self, action: str, params: Dict[str, Any] = None,
                 post_data: Dict[str, Any] = None, files: Dict = None) -> GazelleResponse:
        """
        发送API请求
        
        Args:
            action: API动作名称 (torrent, user, artist...)
            params: GET参数
            post_data: POST数据
            files: 上传文件
            
        Returns:
            GazelleResponse 对象
        """
        # 速率限制
        elapsed = time.time() - self._last_request
        if elapsed < self.rate_limit:
            time.sleep(self.rate_limit - elapsed)

        # 构建参数
        req_params = {'action': action, 'api_key': self.api_key}
        if params:
            req_params.update(params)

        url = f"{self.base_url}/api.php"

        try:
            if post_data or files:
                response = self.session.post(url, params=req_params,
                                            data=post_data, files=files, timeout=30)
            else:
                response = self.session.get(url, params=req_params, timeout=30)

            self._last_request = time.time()
            result = response.json()

            # 解析响应（注意"erro"拼写！）
            if result.get('status') == 200:
                return GazelleResponse(
                    success=True,
                    data=result.get('response', {}),
                    debug=result.get('debug')
                )
            else:
                return GazelleResponse(
                    success=False,
                    error=result.get('erro', 'Unknown error'),  # ⚠️ 不是error
                    data=result.get('response')
                )

        except Exception as e:
            return GazelleResponse(success=False, error=str(e))

    # ========== 种子相关 ==========

    def get_torrent(self, torrent_id: int = None, imdb_id: str = None) -> List[Dict]:
        """
        获取种子详情
        
        Args:
            torrent_id: 种子ID
            imdb_id: IMDB ID (tt1234567)
            
        Returns:
            种子列表（可能匹配多个）
        """
        params = {'req': 'torrent'}
        if torrent_id:
            params['torrent_id'] = torrent_id
        elif imdb_id:
            params['imdbID'] = imdb_id
        else:
            raise ValueError("必须提供 torrent_id 或 imdb_id")

        resp = self._request('torrent', params=params)
        return resp.data if resp.success else []

    def get_torrent_group(self, group_id: int = None, imdb_id: str = None) -> Dict:
        """
        获取发布组详情（含艺术家）
        
        Returns:
            发布组信息字典
        """
        params = {'req': 'group'}
        if group_id:
            params['group_id'] = group_id
        elif imdb_id:
            params['imdbID'] = imdb_id
        else:
            raise ValueError("必须提供 group_id 或 imdb_id")

        resp = self._request('torrent', params=params)
        return resp.data if resp.success else {}

    # ========== 用户相关 ==========

    def get_user(self, user_id: int = None, username: str = None) -> Dict:
        """
        获取用户详细信息
        
        Returns:
            用户信息字典（含统计、等级、隐私设置等）
        """
        params = {}
        if user_id:
            params['user_id'] = user_id
        elif username:
            params['username'] = username
        else:
            raise ValueError("必须提供 user_id 或 username")

        resp = self._request('user', params=params)
        return resp.data if resp.success else {}

    def disable_user(self, user_id: int = None, username: str = None) -> Dict:
        """禁用用户（需要管理员权限）"""
        params = {'req': 'disable'}
        if user_id:
            params['user_id'] = user_id
        elif username:
            params['username'] = username

        resp = self._request('user', params=params)
        return resp.data if resp.success else {}

    def enable_user(self, user_id: int = None, username: str = None,
                   clear_tokens: bool = False) -> Dict:
        """启用用户（需要管理员权限）"""
        params = {'req': 'enable'}
        if user_id:
            params['user_id'] = user_id
        elif username:
            params['username'] = username
        if clear_tokens:
            params['clear_tokens'] = '1'

        resp = self._request('user', params=params)
        return resp.data if resp.success else {}

    # ========== 艺术家相关 ==========

    def get_artist(self, artist_id: int) -> Dict:
        """获取艺术家信息"""
        resp = self._request('artist', params={'artist_id': artist_id})
        return resp.data if resp.success else {}

    # ========== 请求相关 ==========

    def get_request(self, request_id: int) -> Dict:
        """获取种子请求详情"""
        resp = self._request('request', params={'request_id': request_id})
        return resp.data if resp.success else {}

    # ========== 论坛相关 ==========

    def get_forum_topic(self, topic_id: int) -> Dict:
        """获取论坛帖子信息"""
        resp = self._request('forum', params={'topic_id': topic_id})
        return resp.data if resp.success else {}

    # ========== 合辑相关 ==========

    def get_collage(self, collage_id: int) -> Dict:
        """获取合辑信息"""
        resp = self._request('collage', params={'collage_id': collage_id})
        return resp.data if resp.success else {}

    # ========== Wiki相关 ==========

    def get_wiki_article(self, wiki_id: int) -> Dict:
        """获取Wiki文章"""
        resp = self._request('wiki', params={'wiki_id': wiki_id})
        return resp.data if resp.success else {}

    # ========== 电影信息相关 ==========

    def check_movie_duplicate(self, imdb_id: str) -> Dict:
        """
        检查电影是否重复存在
        
        Returns:
            如果存在：{"Dupe": true, "GroupID": xxx}
            如果不存在：完整的电影信息字典
        """
        resp = self._request('movie_info', params={'imdbid': imdb_id})
        return resp.data if resp.success else {}

    # ========== 上传相关 ==========

    def upload_torrent(self, group_id: int = None, torrent_file_path: str = None,
                      **extra_fields) -> Dict:
        """
        上传种子文件
        
        Args:
            group_id: 已有发布组ID（留空创建新组）
            torrent_file_path: 本地.torrent文件路径
            **extra_fields: 其他上传字段
            
        Returns:
            {"torrent_id": 新种子ID}
        """
        params = {}
        if group_id:
            params['groupid'] = group_id

        files = None
        if torrent_file_path:
            with open(torrent_file_path, 'rb') as f:
                files = {'file': f}

        post_data = {**params, **extra_fields}
        resp = self._request('upload', params=params, post_data=post_data, files=files)
        return resp.data if resp.success else {}

    # ========== 邀请相关 ==========

    def generate_invite(self, interviewer_id: int = None,
                       interviewer_name: str = None,
                       email: str = None) -> Dict:
        """
        生成邀请码
        
        Args:
            interviewer_id: 面试官ID
            interviewer_name: 面试官用户名
            email: 被邀请人邮箱（可选）
            
        Returns:
            {"key": 邀请码, "invite_url": 注册链接}
        """
        params = {}
        if interviewer_id:
            params['interviewer_id'] = interviewer_id
        elif interviewer_name:
            params['interviewer_name'] = interviewer_name
        if email:
            params['email'] = email

        resp = self._request('generate_invite', params=params)
        return resp.data if resp.success else {}

    # ========== 图片上传相关 ==========

    def upload_images(self, urls: List[str]) -> Dict:
        """
        批量上传图片（通过URL）
        
        Args:
            urls: 图片URL列表
            
        Returns:
            {"files": [{"name": 存储路径}, ...]}
        """
        resp = self._request('img_upload', post_data={'urls': urls})
        return resp.data if resp.success else {}


# ========== 使用示例 ==========

if __name__ == "__main__":
    # 初始化客户端
    client = GazellePWClient(
        api_key="your_32_char_api_key_here",
        base_url="https://your-gazellepw-site.com",
        rate_limit=1.5
    )

    # 示例1: 查询种子
    print("=" * 50)
    print("示例1: 查询种子")
    torrents = client.get_torrent(torrent_id=12345)
    if torrents:
        for t in torrents:
            print(f"种子: {t.get('Name')}")
            print(f"  做种: {t.get('Seeders')} | 下载: {t.get('Leechers')}")
            print(f"  完成: {t.get('Snatched')}")
    else:
        print("未找到种子")

    # 示例2: 查询用户
    print("\n" + "=" * 50)
    print("示例2: 查询用户信息")
    user = client.get_user(username="MusicLover99")
    if user:
        print(f"用户: {user.get('Username')}")
        print(f"等级: {user.get('ClassName')} (Level {user.get('Level')})")
        stats = user.get('DisplayStats', {})
        print(f"上传: {stats.get('Uploaded', 'N/A')}")
        print(f"下载: {stats.get('Downloaded', 'N/A')}")
        print(f"分享率: {stats.get('Ratio', 'N/A')}")
    else:
        print("用户不存在")

    # 示例3: 检查电影重复
    print("\n" + "=" * 50)
    print("示例3: 检查电影是否已存在")
    movie_result = client.check_movie_duplicate(imdb_id="tt1234567")
    if movie_result.get('Dupe'):
        print(f"⚠️ 该电影已存在！GroupID: {movie_result.get('GroupID')}")
    elif movie_result:
        print(f"✅ 新电影，信息如下:")
        print(f"  标题: {movie_result.get('Title')}")
        print(f"  年份: {movie_result.get('Year')}")
        print(f"  评分: {movie_result.get('Rating')}")
    else:
        print("查询失败")

    # 示例4: 生成邀请码
    print("\n" + "=" * 50)
    print("示例4: 生成邀请码")
    invite = client.generate_invite(interviewer_id=100, email="newuser@example.com")
    if invite:
        print(f"✅ 邀请码: {invite.get('key')}")
        print(f"注册链接: {invite.get('invite_url')}")
    else:
        print("生成失败")

    # 示例5: 批量上传图片
    print("\n" + "=" * 50)
    print("示例5: 批量上传图片")
    image_urls = [
        "https://example.com/image1.jpg",
        "https://example.com/image2.png"
    ]
    upload_result = client.upload_images(image_urls)
    if upload_result.get('files'):
        print(f"✅ 成功上传 {len(upload_result['files'])} 张图片:")
        for f in upload_result['files']:
            print(f"  - {f.get('name')}")
    elif upload_result.get('Error'):
        print(f"❌ 上传失败: {upload_result.get('Error')}")
```

### 9.2 Go语言客户端示例

```go
package gazellepw

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// GazelleResponse Gazelle API 统一响应结构
type GazelleResponse struct {
	Status  int         `json:"status"`
	Response interface{} `json:"response,omitempty"`
	Error   string      `json:"erro,omitempty"` // ⚠️ 注意拼写
	Info    struct {
		Source  string `json:"source"`
		Version int    `json:"version"`
	} `json:"info,omitempty"`
	Debug *struct {
		Queries  []string `json:"queries,omitempty"`
		Searches []string `json:"searches,omitempty"`
	} `json:"debug,omitempty"`
}

// Client GazellePW API 客户端
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	LastCall   time.Time
	RateLimit  time.Duration
}

func NewClient(apiKey string, baseURL string) *Client {
	return &Client{
		BaseURL:   baseURL,
		APIKey:    apiKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		RateLimit: 1 * time.Second,
	}
}

func (c *Client) doRequest(action string, params url.Values) (*GazelleResponse, error) {
	// 速率限制
	if elapsed := time.Since(c.LastCall); elapsed < c.RateLimit {
		time.Sleep(c.RateLimit - elapsed)
	}
	c.LastCall = time.Now()

	// 构建请求
	params.Set("action", action)
	params.Set("api_key", c.APIKey)

	reqURL := fmt.Sprintf("%s/api.php?%s", c.BaseURL, params.Encode())
	resp, err := c.HTTPClient.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	var result GazelleResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}

	// 检查业务错误
	if result.Status != 200 {
		return &result, fmt.Errorf("API错误: %s", result.Error)
	}

	return &result, nil
}

// GetTorrent 获取种子详情
func (c *Client) GetTorrent(torrentID int) ([]map[string]interface{}, error) {
	params := url.Values{}
	params.Set("req", "torrent")
	params.Set("torrent_id", fmt.Sprintf("%d", torrentID))

	resp, err := c.doRequest("torrent", params)
	if err != nil {
		return nil, err
	}

	// 响应是数组
	if arr, ok := resp.Response.([]interface{}); ok {
		result := make([]map[string]interface{}, len(arr))
		for i, v := range arr {
			result[i] = v.(map[string]interface{})
		}
		return result, nil
	}

	return nil, fmt.Errorf("意外的响应格式")
}

// GetUser 获取用户信息
func (c *Client) GetUser(userID int) (map[string]interface{}, error) {
	params := url.Values{}
	params.Set("user_id", fmt.Sprintf("%d", userID))

	resp, err := c.doRequest("user", params)
	if err != nil {
		return nil, err
	}

	if m, ok := resp.Response.(map[string]interface{}); ok {
		return m, nil
	}

	return nil, fmt.Errorf("意外的响应格式")
}

// CheckMovieDuplicate 检查电影重复
func (c *Client) CheckMovieDuplicate(imdbID string) (map[string]interface{}, error) {
	params := url.Values{}
	params.Set("imdbid", imdbID)

	resp, err := c.doRequest("movie_info", params)
	if err != nil {
		return nil, err
	}

	if m, ok := resp.Response.(map[string]interface{}); ok {
		return m, nil
	}

	return nil, fmt.Errorf("意外的响应格式")
}
```

### 9.3 最佳实践建议

#### ✅ 推荐做法

1. **使用专用的API Key**
   - 为每个应用/用途创建独立的API Key
   - 定期轮换Key以提高安全性
   - 在数据库中记录每个Key的使用场景

2. **实现合理的速率限制**
   - Gazelle本身没有内置速率限制
   - 客户端应自行控制请求频率（建议≥1秒间隔）
   - 避免批量请求导致服务器负载过高

3. **处理隐私设置**
   - 用户可能隐藏了上传/下载数据
   - 尊重用户的Paranoia设置
   - 显示"Hidden"而非报错

4. **利用缓存机制**
   - Gazelle会缓存API Key验证结果
   - 相同Key的连续请求性能更好
   - 不要频繁更换Key

5. **错误处理注意事项**
   - ⚠️ **始终检查`erro`字段（不是`error`）**
   - 所有错误都返回HTTP 200，必须检查JSON内的status
   - `json_error()`调用后会`die()`，不会有更多输出

#### ❌ 避免的做法

1. **不要在公开代码中暴露API Key**
2. **不要忽略`status`字段**
3. **不要假设响应一定是对象（可能是数组）**
4. **不要频繁调用管理员接口（enable/disable）**
5. **不要在上传时发送过大的文件**

---

## 附录A: 快速参考卡片

### A.1 所有API端点一览

| Action | 方法 | Req参数 | 功能 | 关键参数 |
|--------|------|---------|------|----------|
| `torrent` | GET | `torrent`/`group` | 种子/发布组查询 | torrent_id, imdbID, group_id |
| `user` | GET/POST | `stats`/`enable`/`disable` | 用户信息/管理 | user_id, username |
| `artist` | GET | - | 艺术家查询 | artist_id |
| `request` | GET | - | 种子请求 | request_id |
| `forum` | GET | - | 论坛帖子 | topic_id |
| `collage` | GET | - | 合辑信息 | collage_id |
| `wiki` | GET | - | Wiki文章 | wiki_id |
| `movie_info` | GET | - | 电影信息/去重 | imdbid |
| `upload` | POST | - | 上传种子 | (multipart form-data) |
| `generate_invite` | GET | - | 生成邀请码 | interviewer_id, email |
| `img_upload` | POST | - | 批量上传图片 | urls[] (POST数组) |

### A.2 请求格式速查

**GET请求**:
```
/api.php?action={action}&{params}&api_key={YOUR_KEY}
```

**POST请求**:
```
POST /api.php?action={action}&api_key={YOUR_KEY}
Content-Type: multipart/form-data

{post_data}&{files}
```

### A.3 响应格式速查

**成功**:
```json
{"status": 200, "response": {...}, "info": {...}}
```

**失败**:
```json
{"status": "failure", "erro": "错误消息", "response": [], "info": {...}}
```

### A.4 常见错误消息

| 错误消息 | 含义 | 解决方案 |
|----------|------|----------|
| `invalid action` | 不支持的action | 检查action参数拼写 |
| `invalid parameters` | 缺少api_key | 提供有效的API Key |
| `invalid token` | Token无效 | 检查Token是否正确 |
| `Missing {x} id` | 缺少资源ID | 提供对应的ID参数 |
| `{Resource} not found` | 资源不存在 | 检查ID是否正确 |

---

## 附录B: 版本历史

| 版本 | 日期 | 作者 | 变更说明 |
|------|------|------|----------|
| v1.0 | 2026-04-12 | AI Assistant | 初始版本，基于源码深度分析创建 |

---

## 附录C: 相关文档

- [NexusPHP API 完整指南](file:///home/incast/PT-Forward/docs/28-nexusphp-api-complete-guide.md) - NexusPHP API参考
- [M-Team API 完整指南](file:///home/incast/PT-Forward/docs/27-mteam-api-complete-guide.md) - M-Team(mTorrent) API参考
- [PT生态系统概览](file:///home/incast/PT-Forward/docs/02-pt-ecosystem-overview.md) - PT站点架构总览

---

> **文档结束** | 总计约 **900+ 行**，覆盖GazellePW的所有11个API端点、认证机制、数据模型、最佳实践及与NexusPHP/M-Team的详细对比。
