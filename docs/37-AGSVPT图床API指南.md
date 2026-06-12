# AGSVPT 公开图床 API 指南

> 来源：https://img.seedvault.cn/api
> 基础 URL：`https://img.seedvault.cn/api/v1`

## 服务概况

AGSVPT 公开免费图床（域名 `img.seedvault.cn`），国内 CDN 加速，可供其他站点调用。

| 项目 | 说明 |
|------|------|
| 费用 | 免费 |
| 注册要求 | 注册后使用，游客上传压缩到 10%，登录后画质完全保留 |
| 单次上传限制 | 最高 30MB |
| 流量限制 | 每账户每月 100GB |
| 容量限制 | 488.28 GB |
| 内容限制 | 严禁 R18 或政治相关内容，违禁 3 次删除该账号所有图片 |
| 封禁申诉 | admin@agsv.date |

## 认证方式

### Bearer Token

所有接口使用 Bearer Token 认证。在请求 Header 中携带：

```
Authorization: Bearer {token}
Accept: application/json
```

**未设置 Authorization 时视为游客上传（画质压缩至 10%）。**

### 获取 Token

```
POST /tokens
Content-Type: application/json
```

请求参数：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| email | String | 是 | 注册邮箱 |
| password | String | 是 | 密码 |

响应示例：

```json
{
  "status": true,
  "message": "...",
  "data": {
    "token": "1|1bJbwlqBfnggmOMEZqXT5XusaIwqiZjCDs7r1Ob5"
  }
}
```

### 清空 Token

```
DELETE /tokens
```

使当前 Token 失效。

### 用户资料

```
GET /profile
```

响应 `data` 字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| name | String | 用户名 |
| avatar | String | 头像地址 |
| email | String | 邮箱地址 |
| capacity | Float | 总容量（Byte） |
| used_capacity | Float | 已使用容量（Byte） |
| url | String | 个人主页地址 |
| image_num | Integer | 图片数量 |
| album_num | Integer | 相册数量 |
| registered_ip | String | 注册 IP |

## 策略相关

### 策略列表

```
GET /strategies
```

Query 参数：

| 字段 | 类型 | 说明 |
|------|------|------|
| keyword | String | 筛选关键字 |

响应 `data.strategies[]`：

| 字段 | 类型 | 说明 |
|------|------|------|
| id | Integer | 策略 ID |
| name | String | 策略名称 |

当前已知策略：`id=1`「国内高速图床」

## 图片操作

### 上传图片

```
POST /upload
Content-Type: multipart/form-data
```

请求参数：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file | File | 是 | 图片文件（最大 30MB） |
| strategy_id | Integer | 否 | 储存策略 ID（默认 1 = 国内高速图床） |

响应 `data` 字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| key | String | 图片唯一密钥（用于删除等操作） |
| name | String | 图片名称 |
| pathname | String | 图片路径名 |
| origin_name | String | 图片原始名 |
| size | Float | 图片大小（KB） |
| mimetype | String | MIME 类型 |
| extension | String | 文件扩展名 |
| md5 | String | MD5 值 |
| sha1 | String | SHA1 值 |
| links.url | String | 图片访问 URL |
| links.html | String | HTML 标签 |
| links.bbcode | String | BBCode 标签 |
| links.markdown | String | Markdown 标签 |
| links.markdown_with_link | String | Markdown 带链接 |
| links.thumbnail_url | String | 缩略图 URL |

### 图片列表

```
GET /images
```

Query 参数：

| 字段 | 类型 | 说明 |
|------|------|------|
| page | Integer | 页码 |
| order | String | newest / earliest / utmost（最大）/ least（最小） |
| permission | String | public / private |
| album_id | Integer | 相册 ID |
| keyword | String | 筛选关键字 |

响应 `data` 分页结构：

| 字段 | 类型 | 说明 |
|------|------|------|
| current_page | Integer | 当前页码 |
| last_page | Integer | 最后一页 |
| per_page | Integer | 每页数量 |
| total | Integer | 总数量 |
| data[] | Object[] | 图片列表 |

每条图片包含：`key`, `name`, `origin_name`, `pathname`, `size`, `width`, `height`, `md5`, `sha1`, `human_date`, `date`, `links`

### 删除图片

```
DELETE /images/:key
```

Path 参数：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| key | String | 是 | 图片密钥（上传时返回的 key） |

## 相册操作

### 相册列表

```
GET /albums
```

Query 参数：

| 字段 | 类型 | 说明 |
|------|------|------|
| page | Integer | 页码 |
| order | String | newest / earliest / most（图片最多）/ least（图片最少） |
| keyword | String | 筛选关键字 |

响应 `data.data[]`：

| 字段 | 类型 | 说明 |
|------|------|------|
| id | Integer | 相册自增 ID |
| name | String | 相册名称 |
| intro | String | 相册简介 |
| image_num | Integer | 图片数量 |

### 删除相册

```
DELETE /albums/:id
```

Path 参数：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | String | 是 | 相册自增 ID |

## 公共响应格式

所有接口统一返回：

```json
{
  "status": true|false,
  "message": "描述信息",
  "data": { ... }
}
```

## 限流说明

响应 Header 携带限流信息：

| Header | 类型 | 说明 |
|--------|------|------|
| X-RateLimit-Limit | Integer | 每分钟请求配额 |
| X-RateLimit-Remaining | Integer | 剩余请求配额 |

## HTTP 状态码

| 状态码 | 说明 |
|--------|------|
| 401 | 未登录或授权失败 |
| 403 | 管理员关闭了接口功能 |
| 429 | 超出请求配额 |
| 500 | 服务端异常 |

## 典型使用流程

```bash
# 1. 获取 Token
curl --noproxy '*' -s -X POST https://img.seedvault.cn/api/v1/tokens \
  -H "Accept: application/json" \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"xxx"}'

# 2. 上传图片
curl --noproxy '*' -s -X POST https://img.seedvault.cn/api/v1/upload \
  -H "Accept: application/json" \
  -H "Authorization: Bearer {token}" \
  -F "file=@screenshot.png"

# 3. 返回中获取 links.url / links.bbcode / links.markdown 等格式链接
```
