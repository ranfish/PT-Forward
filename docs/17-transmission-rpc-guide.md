# Transmission 深度技术研究报告

> **项目位置**: [examples/transmission](../examples/transmission)
>
> **研究日期**: 2026-04-12
>
> **研究范围**: RPC架构、libtransmission库、会话管理、配置系统、安全机制

---

## 目录

1. [项目概览](#1-项目概览)
2. [技术架构](#2-技术架构)
3. [核心模块分析](#3-核心模块分析)
4. [RPC 接口系统](#4-rpc-接口系统)
5. [会话管理](#5-会话管理)
6. [配置系统](#6-配置系统)
7. [安全机制](#7-安全机制)
8. [性能特性](#8-性能特性)
9. [PT 场景应用](#9-pt-场景应用)
10. [与 qBittorrent 对比](#10-与-qbittorrent-对比)
11. [部署建议](#11-部署建议)

---

## 1. 项目概览

### 1.1 基本信息

| 属性 | 值 |
|------|-----|
| **项目名称** | Transmission |
| **技术栈** | C++17 / libevent / CMake |
| **许可证** | GPL-2.0+/GPL-3.0+ (双许可) |
| **代码规模** | ~80,000+ 行 C++代码 |
| **主要用途** | 轻量级 BT 下载客户端 |
| **RPC端口** | 默认 9091 |
| **API风格** | JSON-RPC over HTTP |

### 1.2 在 PT 生态系统中的地位

```
PT 生态系统
├── 下载器客户端
│   ├── qBittorrent ⭐ 功能丰富首选
│   └── transmission ⭐ 轻量高效备选
├── 使用场景
│   ├── NAS/嵌入式设备 → Transmission (资源占用低)
│   ├── Docker容器 → Transmission (镜像小)
│   └── 服务器后台 → Transmission (守护进程模式)
└── 特点
    ├── 内存占用: ~50MB (qBittorrent的1/4)
    ├── 启动速度快 (<1秒)
    └── 稳定性极高 (长期运行无内存泄漏)
```

**为什么选择 Transmission？**
- ✅ 极低的资源占用（内存/CPU）
- ✅ 守护进程模式，适合服务器部署
- ✅ WebUI简洁易用
- ✅ 配置简单，开箱即用
- ✅ 长期稳定运行（适合7x24小时做种）

---

## 2. 技术架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────┐
│                    Transmission 架构                         │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │
│  │  WebUI 层    │  │  CLI 层     │  │  Daemon 层   │      │
│  │  (HTTP/RPC)  │  │ (transmission-cli)│ (transmission-daemon)│
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘      │
│         │                 │                 │               │
│         └─────────────────┼─────────────────┘               │
│                           ▼                                 │
│  ┌──────────────────────────────────────────────────┐        │
│  │              libtransmission (核心库)              │        │
│  │                                                  │        │
│  │  ┌────────────┐ ┌────────────┐ ┌────────────┐   │        │
│  │  │ tr_session │ │tr_torrent  │ │ rpc-server  │   │        │
│  │  │ (会话管理) │ │ (种子对象)  │ │ (RPC服务)   │   │        │
│  │  └────────────┘ └────────────┘ └────────────┘   │        │
│  │  ┌────────────┐ ┌────────────┐ ┌────────────┐   │        │
│  │  │ announcer  │ │  peer-mgr  │ │   web       │   │        │
│  │  │ (Tracker)  │ │ (Peer管理) │ │ (HTTP客户端)│   │        │
│  │  └────────────┘ └────────────┘ └────────────┘   │        │
│  └─────────────────────┬────────────────────────────┘        │
│                        ▼                                      │
│  ┌──────────────────────────────────────────────────┐        │
│  │              libevent (事件驱动框架)              │        │
│  └──────────────────────────────────────────────────┘        │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 核心目录结构

```
transmission/
├── libtransmission/          # 核心库 ⭐最重要
│   ├── transmission.h        # 公共API头文件
│   ├── session.h             # 会话结构定义
│   ├── session-settings.h    # 会话配置(60+项)
│   ├── session-id.h          # Session ID管理
│   ├── session-thread.h      # 会话线程
│   ├── rpc-server.h/cc       # RPC服务器
│   ├── rpcimpl.h/cc          # RPC实现
│   ├── quark.h               # 字符串→整数映射
│   ├── announce-list.h       # Tracker列表
│   ├── announcer.h           # Announce逻辑
│   ├── bandwidth.h           # 带宽控制
│   ├── bitfield.h            # 位域操作
│   ├── blocklist.h           # IP黑名单
│   ├── cache.h               # 磁盘缓存
│   ├── clients.h             # Peer客户端识别
│   ├── completion.h          # 完成度追踪
│   ├── crypto-utils.h        # 加密工具
│   ├── error.h               # 错误处理
│   ├── file.h                # 文件操作
│   ├── file-piece-map.h      # 文件-Piece映射
│   ├── handshake.h           # BT握手
│   ├── history.h             # 统计历史
│   ├── inout.h               # 数据读写
│   ├── log.h                 # 日志系统
│   ├── magnet-metainfo.h     # 磁力链接解析
│   ├── makemeta.h            # 种子创建
│   ├── mime-types.h          # MIME类型
│   ├── net.h                 # 网络抽象
│   ├── open-files.h          # 文件句柄管理
│   ├── peer-common.h         # Peer通用定义
│   ├── peer-io.h             # Peer I/O
│   ├── peer-mgr.h            # Peer管理器
│   ├── peer-msgs.h           # Peer消息
│   ├── platform.h            # 平台抽象
│   ├── port-forwarding.h     # 端口转发(UPnP/NAT-PMP)
│   ├── quark.h               # Quark系统
│   ├── resume.h              # 断点续传
│   ├── session.h             # 会话定义
│   ├── session-alt-speeds.h  # 替代速度计划
│   ├── settings.h            # 设置辅助
│   ├── stats.h               # 统计数据
│   ├── torrent.h             # 种子对象
│   ├── torrent-metainfo.h    # 种子元信息
│   ├── tr-dht.h              # DHT支持
│   ├── tr-lpd.h              # LPD支持
│   ├── tr-udp.h              # UDP Tracker
│   ├── tr-utp.h              # UTP协议
│   ├── utils-ev.h            # libevent封装
│   ├── verify.h              # 校验逻辑
│   └── web.h                 # HTTP客户端
│
├── daemon/                   # 守护进程
│   ├── daemon.cc             # 主入口
│   ├── daemon-posix.cc       # Unix实现
│   ├── daemon-win32.cc       # Windows实现
│   └── transmission-daemon.service  # systemd服务文件
│
├── cli/                      # 命令行工具
│   └── transmission-cli.cc   # CLI客户端
│
├── gtk/                      # GTK界面(Linux)
├── macosx/                   # macOS界面(Cocoa)
├── docs/                     # 文档
│   └── rpc-spec.md           # RPC规范⭐重要
└── dist/                     # 分发包
```

### 2.3 设计哲学

| 设计原则 | 实现 | 说明 |
|----------|------|------|
| **轻量级** | 无GUI依赖(core) | 适合嵌入式/NAS |
| **事件驱动** | libevent | 高性能异步I/O |
| **模块化** | 清晰的层次分离 | 易于维护和扩展 |
| **跨平台** | POSIX/Win32抽象 | 一套代码多平台 |
| **配置优先** | JSON格式 | 人机可读可编辑 |

---

## 3. 核心模块分析

### 3.1 libtransmission 库

**头文件**: [transmission.h](../examples/transmission/libtransmission/transmission.h)

**核心类型定义：**

```c
// 基础类型
using tr_file_index_t = size_t;        // 文件索引
using tr_piece_index_t = uint32_t;      // Piece索引
using tr_block_index_t = uint32_t;      // Block索引
using tr_byte_index_t = uint64_t;       // 字节索引
using tr_torrent_id_t = int;            // 种子ID
using tr_bytes_per_second_t = size_t;   // 速度单位
using tr_port = uint16_t;               // 端口号
using tr_mode_t = uint16_t;             // 文件权限

// 常量定义
#define TR_DEFAULT_RPC_PORT 9091       // RPC默认端口
#define TR_DEFAULT_PEER_PORT 51413     // Peer默认端口
#define TR_DEFAULT_PEER_LIMIT_GLOBAL 200  // 全局Peer限制
#define TR_DEFAULT_PEER_LIMIT_TORRENT 50  // 每种子Peer限制
#define TR_DEFAULT_RPC_WHITELIST "127.0.0.1,::1"  // RPC白名单默认值
```

**枚举类型：**

```c
// 校验模式
enum tr_verify_added_mode {
    TR_VERIFY_ADDED_FAST = 0,   // 快速模式（延迟校验）⭐推荐
    TR_VERIFY_ADDED_FULL = 1    // 完整校验
};

// 预分配模式
enum tr_preallocation_mode {
    TR_PREALLOCATE_NONE = 0,    // 不预分配
    TR_PREALLOCATE_SPARSE = 1,  // 稀疏预分配⭐推荐
    TR_PREALLOCATE_FULL = 2     // 完全预分配
};

// 加密模式
enum tr_encryption_mode {
    TR_CLEAR_PREFERRED,         // 明文优先
    TR_ENCRYPTION_PREFERRED,    // 加密优先 ⭐推荐
    TR_ENCRYPTION_REQUIRED      // 强制加密
};
```

### 3.2 Session（会话）管理

**文件**: [session.h](../examples/transmission/libtransmission/session.h)

```cpp
struct tr_session {
private:
    // ====== 内部组件 ======
    class BoundSocket {
        // 监听Socket封装
        using IncomingCallback = void (*)(tr_socket_t, void*);
        BoundSocket(struct event_base* base, ...);
    };

    class AltSpeedMediator final : public tr_session_alt_speeds::Mediator {
        // 替代速度调度中介
    };

    class AnnouncerUdpMediator final : public tr_announcer_udp::Mediator {
        // UDP Announce中介
    };

public:
    // ====== 网络组件 ======
    std::unique_ptr<tr_lpd> lpd_;                          // Local Peer Discovery
    std::unique_ptr<struct utp_context> utp_;              // UTP上下文
    std::unique_ptr<tr_port_forwarding> port_forwarding_;  // 端口转发
    std::unique_ptr<tr_rpc_server> rpc_server_;            // RPC服务器 ⭐
    std::unique_ptr<tr_web> web_;                          // HTTP客户端
    std::unique_ptr<libtransmission::Dns> dns_;            // DNS解析

    // ====== 子系统 ======
    std::unique_ptr<libtransmission::Announcer> announcer_;      // Tracker通信
    std::unique_ptr<libtransmission::PeerMgr> peer_mgr_;        // Peer管理
    std::unique_ptr<libtransmission::Cache> cache_;             // 磁盘缓存
    std::unique_ptr<Blocklist> blocklist_;                      // IP黑名单

    // ====== 设置 ======
    tr_session_settings settings_;                              // 会话设置(60+项)

    // ====== 统计 ======
    struct tr_stats stats_;                                     // 统计数据
    struct tr_session_thread* session_thread_;                   // 工作线程
};
```

**生命周期管理：**

```c
// 初始化
tr_session* session = tr_sessionInit(config_dir, true, &settings);

// 更新设置
tr_sessionSet(session, &new_settings);

// 关闭（等待15秒让Tracker announce完成）
tr_sessionClose(session, 15);
```

### 3.3 Quark 系统（字符串优化）

**文件**: [quark.h](../examples/transmission/libtransmission/quark.h)

**设计目标：**
- 高效的字符串→整数映射
- 用于RPC字段名、配置项名
- O(1)查找性能

**工作原理：**

```cpp
// 定义Quark枚举
enum {
    TR_KEY_NONE,
    TR_KEY_activeTorrentCount,
    TR_KEY_activityDate,
    TR_KEY_addedDate,
    TR_KEY_downloadDir,
    TR_KEY_uploadLimit,
    // ... 数百个预定义键
};

// 使用方式
tr_quark q = tr_quark_new("downloadDir");  // 字符串→整数
const char* s = tr_quark_get_string(q);   // 整数→字符串
```

**优势：**
- ✅ 减少字符串比较开销
- ✅ 节省内存（只存一份字符串）
- ✅ 提升序列化/反序列化性能

---

## 4. RPC 接口系统

### 4.1 RPC 架构

```
HTTP Request (POST /transmission/rpc)
     │
     ▼
┌─────────────┐
│ RPC Server   │  ← HTTP监听 (libevent)
│ (rpc-server) │
└──────┬──────┘
       │
       ├─ 1. Session ID验证
       ├─ 2. 认证检查
       ├─ 3. 反暴力破解
       └─ 4. 白名单检查
              │
              ▼
     ┌─────────────┐
     │ RPC Impl     │  ← 请求分发执行
     │ (rpcimpl)    │
     └──────┬──────┘
            │
            ▼
     ┌─────────────┐
     │ JSON Response│
     └─────────────┘
```

### 4.2 RPC Server 配置

**文件**: [rpc-server.h](../examples/transmission/libtransmission/rpc-server.h)

```cpp
#define RPC_SETTINGS_FIELDS(V) \
    V(TR_KEY_anti_brute_force_enabled, is_anti_brute_force_enabled_, bool, false, "") \
    V(TR_KEY_anti_brute_force_threshold, anti_brute_force_limit_, size_t, 100U, "") \
    V(TR_KEY_rpc_authentication_required, authentication_required_, bool, false, "") \
    V(TR_KEY_rpc_bind_address, bind_address_str_, std::string, "0.0.0.0", "") \
    V(TR_KEY_rpc_enabled, is_enabled_, bool, false, "") \
    V(TR_KEY_rpc_host_whitelist, host_whitelist_str_, std::string, "", "") \
    V(TR_KEY_rpc_host_whitelist_enabled, is_host_whitelist_enabled_, bool, true, "") \
    V(TR_KEY_rpc_port, port_, tr_port, tr_port::fromHost(TR_DEFAULT_RPC_PORT), "") \
    V(TR_KEY_rpc_password, salted_password_, std::string, "", "") \
    V(TR_KEY_rpc_socket_mode, socket_mode_, tr_mode_t, 0750, "") \
    V(TR_KEY_rpc_url, url_, std::string, TR_DEFAULT_RPC_URL_STR, "") \
    V(TR_KEY_rpc_username, username_, std::string, "", "") \
    V(TR_KEY_rpc_whitelist, whitelist_str_, std::string, TR_DEFAULT_RPC_WHITELIST, "") \
    V(TR_KEY_rpc_whitelist_enabled, is_whitelist_enabled_, bool, true, "")
```

**配置说明：**

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `rpc-enabled` | bool | false | 是否启用RPC |
| `rpc-port` | port | 9091 | 监听端口 |
| `rpc-url` | string | "/transmission/" | URL路径 |
| `rpc-bind-address` | string | "0.0.0.0" | 绑定地址 |
| `rpc-authentication-required` | bool | false | 是否需要认证 |
| `rpc-username` | string | "" | 用户名 |
| `rpc-password` | string | "" | 密码（加盐存储） |
| `rpc-whitelist-enabled` | bool | true | 启用白名单 |
| `rpc-whitelist` | string | "127.0.0.1,::1" | 白名单列表 |
| `rpc-host-whitelist-enabled` | bool | true | 主机名白名单 |
| `rpc-socket-mode` | mode | 0750 | Unix socket权限 |
| `anti-brute-force-enabled` | bool | false | 反暴力破解 |
| `anti-brute-force-threshold` | size_t | 100 | 失败阈值 |

### 4.3 Session ID 机制

**文件**: [session-id.h](../examples/transmission/libtransmission/session-id.h)

```cpp
class tr_session_id {
public:
    using current_time_func_t = time_t (*)();

    explicit tr_session_id(current_time_func_t get_current_time);

    // 检查是否为本地会话
    [[nodiscard]] static bool isLocal(std::string_view) noexcept;

    // 当前Session ID
    [[nodiscard]] std::string_view sv() const noexcept;
    [[nodiscard]] char const* c_str() const noexcept;

private:
    static auto constexpr SessionIdSize = size_t{ 48 };           // ID长度: 48字符
    static auto constexpr SessionIdDurationSec = time_t{ 60 * 60 }; // 有效期: 1小时

    using session_id_t = std::array<char, SessionIdSize + 1>;
    static session_id_t make_session_id();

    current_time_func_t const get_current_time_;

    mutable session_id_t current_value_ = {};
    mutable session_id_t previous_value_ = {};     // 上一个ID（兼容过渡期）
    mutable tr_sys_file_t current_lock_file_;
    mutable tr_sys_file_t previous_lock_file_;
    mutable time_t expires_at_ = 0;               // 过期时间戳
};
```

**工作机制：**

```
Client                              Server
  │                                   │
  │ POST /transmission/rpc             │
  │ (无Session-ID header)              │
  │                                   │
  │←── 409 Conflict                    │
  │    X-Transmission-Session-Id: xxx │
  │                                   │
  │ POST /transmission/rpc             │
  │ Header: X-Transmission-Session-Id: xxx │
  │ { "method": "torrent-get", ... }   │
  │                                   │
  │←── 200 OK                          │
  │    { "result": "success", ... }    │
  │                                   │
  │ (1小时内复用同一个Session-ID)       │
```

**为什么需要Session-ID？**
1. **CSRF防护**：防止跨站请求伪造
2. **状态跟踪**：标识客户端会话
3. **安全性**：即使密码泄露，没有Session-ID也无法操作

### 4.4 核心 RPC 方法

**参考文档**: [docs/rpc-spec.md](../examples/transmission/docs/rpc-spec.md)

#### 4.4.1 种子管理

```json
// 添加种子
{
    "method": "torrent-add",
    "arguments": {
        "filename": "/path/to/file.torrent",
        "metainfo": "<base64-encoded-torrent>",
        "download-dir": "/downloads/",
        "paused": false,
        "priority-normal": [],
        "priority-high": [],
        "priority-low": [],
        "cookies": "name=value; name2=value2",
        "bandwidthPriority": 1
    }
}

// 查询种子
{
    "method": "torrent-get",
    "arguments": {
        "ids": [1, 2, 3],  // 或 "recently-active"
        "fields": ["id", "name", "status", "progress", "rateDownload"]
    }
}

// 删除种子
{
    "method": "torrent-remove",
    "arguments": {
        "ids": [1],
        "delete-local-data": false
    }
}
```

#### 4.4.2 会话统计

```json
// 获取全局统计
{
    "method": "session-stats",
    "arguments": {}
}
// Response:
{
    "arguments": {
        "activeTorrentCount": 5,
        "downloadSpeed": 1024000,
        "uploadSpeed": 512000,
        "cumulative-stats": {
            "uploadedBytes": 1099511627776,
            "downloadedBytes": 549755813888,
            "filesAdded": 1000,
            "sessionCount": 50,
            "secondsActive": 17280000
        },
        "current-stats": {
            "uploadedBytes": 10737418240,
            "downloadBytes": 5368709120
        }
    }
}
```

#### 4.4.3 会话设置

```json
// 获取设置
{ "method": "session-get" }

// 修改设置
{
    "method": "session-set",
    "arguments": {
        "speed-limit-down": 500,
        "speed-limit-down-enabled": true,
        "speed-limit-up": 200,
        "speed-limit-up-enabled": true,
        "alt-speed-enabled": false,
        "download-dir": "/downloads/",
        "incomplete-dir-enabled": true,
        "incomplete-dir": "/downloads/incomplete/"
    }
}
```

---

## 5. 会话管理

### 5.1 Session Settings（60+配置项）

**文件**: [session-settings.h](../examples/transmission/libtransmission/session-settings.h)

**完整配置清单：**

```cpp
#define SESSION_SETTINGS_FIELDS(V) \
    // ====== 网络配置 ======
    V(TR_KEY_bind_address_ipv4, bind_address_ipv4, std::string, "0.0.0.0", "") \
    V(TR_KEY_bind_address_ipv6, bind_address_ipv6, std::string, "::", "") \
    V(TR_KEY_peer_port, peer_port, tr_port, 51413, "Peer端口") \
    V(TR_KEY_peer_port_random_on_start, peer_port_random_on_start, bool, false, "") \
    V(TR_KEY_peer_port_random_low, peer_port_random_low, tr_port, 49152, "") \
    V(TR_KEY_peer_port_random_high, peer_port_random_high, tr_port, 65535, "") \
    V(TR_KEY_tcp_enabled, tcp_enabled, bool, true, "") \
    V(TR_KEY_utp_enabled, utp_enabled, bool, true, "") \

    // ====== 连接限制 ======
    V(TR_KEY_peer_limit_global, peer_limit_global, size_t, 200, "全局Peer限制") \
    V(TR_KEY_peer_limit_per_torrent, peer_limit_per_torrent, size_t, 50, "每种子Peer限制") \
    V(TR_KEY_upload_slots_per_torrent, upload_slots_per_torrent, size_t, 8, "上传槽位") \

    // ====== 下载目录 ======
    V(TR_KEY_download_dir, download_dir, std::string, "", "下载目录") \
    V(TR_KEY_incomplete_dir, incomplete_dir, std::string, "", "未完成目录") \
    V(TR_KEY_incomplete_dir_enabled, incomplete_dir_enabled, bool, false, "") \
    V(TR_KEY_rename_partial_files, is_incomplete_file_naming_enabled, bool, false, ".part后缀") \

    // ====== 预分配与缓存 ======
    V(TR_KEY_preallocation, preallocation_mode, tr_preallocation_mode, TR_PREALLOCATE_SPARSE, "") \
    V(TR_KEY_cache_size_mb, cache_size_mb, size_t, 4, "磁盘缓存(MB)") \
    V(TR_KEY_prefetch_enabled, is_prefetch_enabled, bool, true, "预读取") \

    // ====== 队列管理 ======
    V(TR_KEY_download_queue_enabled, download_queue_enabled, bool, true, "") \
    V(TR_KEY_download_queue_size, download_queue_size, size_t, 5, "下载队列大小") \
    V(TR_KEY_seed_queue_enabled, seed_queue_enabled, bool, false, "") \
    V(TR_KEY_seed_queue_size, seed_queue_size, size_t, 10, "做种队列大小") \
    V(TR_KEY_queue_stalled_enabled, queue_stalled_enabled, bool, true, "") \
    V(TR_KEY_queue_stalled_minutes, queue_stalled_minutes, size_t, 30, "停滞超时(分钟)") \

    // ====== 速度限制 ======
    V(TR_KEY_speed_limit_down, speed_limit_down, size_t, 100, "下载限速(KB/s)") \
    V(TR_KEY_speed_limit_down_enabled, speed_limit_down_enabled, bool, false, "") \
    V(TR_KEY_speed_limit_up, speed_limit_up, size_t, 100, "上传限速(KB/s)") \
    V(TR_KEY_speed_limit_up_enabled, speed_limit_up_enabled, bool, false, "") \

    // ====== 分享率 ======
    V(TR_KEY_ratio_limit, ratio_limit, double, 2.0, "分享率限制") \
    V(TR_KEY_ratio_limit_enabled, ratio_limit_enabled, bool, false, "") \
    V(TR_KEY_idle_seeding_limit, idle_seeding_limit_minutes, size_t, 30, "空闲做种限制(分钟)") \
    V(TR_KEY_idle_seeding_limit_enabled, idle_seeding_limit_enabled, bool, false, "") \

    // ====== 加密与隐私 ======
    V(TR_KEY_encryption, encryption_mode, tr_encryption_mode, TR_ENCRYPTION_PREFERRED, "") \
    V(TR_KEY_blocklist_enabled, blocklist_enabled, bool, false, "IP黑名单") \
    V(TR_KEY_blocklist_url, blocklist_url, std::string, "", "黑名单URL") \

    // ====== P2P网络 ======
    V(TR_KEY_dht_enabled, dht_enabled, bool, true, "DHT支持") \
    V(TR_KEY_pex_enabled, pex_enabled, bool, true, "PEX支持") \
    V(TR_KEY_lpd_enabled, lpd_enabled, bool, true, "LPD支持") \
    V(TR_KEY_port_forwarding_enabled, port_forwarding_enabled, bool, true, "UPnP/NAT-PMP") \

    // ====== 脚本钩子 ⭐高级 ======
    V(TR_KEY_script_torrent_added_enabled, script_torrent_added_enabled, bool, false, "") \
    V(TR_KEY_script_torrent_added_filename, script_torrent_added_filename, std::string, "", "") \
    V(TR_KEY_script_torrent_done_enabled, script_torrent_done_enabled, bool, false, "") \
    V(TR_KEY_script_torrent_done_filename, script_torrent_done_filename, std::string, "", "") \
    V(TR_KEY_script_torrent_done_seeding_enabled, script_torrent_done_seeding_enabled, bool, false, "") \
    V(TR_KEY_script_torrent_done_seeding_filename, script_torrent_done_seeding_filename, std::string, "", "") \

    // ====== 其他 ======
    V(TR_KEY_start_added_torrents, should_start_added_torrents, bool, true, "") \
    V(TR_KEY_trash_original_torrent_files, should_delete_source_torrents, bool, false, "") \
    V(TR_KEY_scrape_paused_torrents_enabled, should_scrape_paused_torrents, bool, true, "") \
    V(TR_KEY_message_level, log_level, tr_log_level, TR_LOG_INFO, "") \
    V(TR_KEY_umask, umask, tr_mode_t, 022, "文件权限掩码") \
    V(TR_KEY_peer_socket_tos, peer_socket_tos, tr_tos_t, 0x04, "") \
    V(TR_KEY_peer_congestion_algorithm, peer_congestion_algorithm, std::string, "", "") \
    V(TR_KEY_default_trackers, default_trackers_str, std::string, "", "") \
    V(TR_KEY_announce_ip, announce_ip, std::string, "", "") \
    V(TR_KEY_announce_ip_enabled, announce_ip_enabled, bool, false, "") \
    V(TR_KEY_torrent_added_verify_mode, torrent_added_verify_mode, tr_verify_added_mode, TR_VERIFY_ADDED_FAST, "")
```

### 5.2 PT 场景推荐配置

```json
{
    "method": "session-set",
    "arguments": {
        // ====== 网络优化 ======
        "peer-limit-global": 500,
        "peer-limit-per-torrent": 100,
        "upload-slots-per-torrent": 10,

        // ====== 下载目录 ======
        "download-dir": "/data/downloads/",
        "incomplete-dir": "/data/incomplete/",
        "incomplete-dir-enabled": true,

        // ====== 性能调优 ======
        "cache-size-mb": 16,
        "preallocation": "sparse",
        "prefetch-enabled": true,

        // ====== 队列管理 ======
        "download-queue-size": 10,
        "seed-queue-size": 20,
        "seed-queue-enabled": true,
        "queue-stalled-minutes": 60,

        // ====== 分享率管理 ⭐PT重要 ======
        "ratio-limit": 0,
        "ratio-limit-enabled": false,
        "idle-seeding-limit": 0,
        "idle-seeding-limit-enabled": false,

        // ====== P2P网络 ======
        "dht-enabled": true,
        "pex-enabled": true,
        "lpd-enabled": true,
        "port-forwarding-enabled": true,

        // ====== 加密 ======
        "encryption": "preferred",

        // ====== 行为 ======
        "start-added-torrents": true,
        "trash-original-torrent-files": false,
        "script-torrent-done-filename": "/scripts/on_download_complete.sh"
    }
}
```

---

## 6. 配置系统

### 6.1 配置文件位置

```
~/.config/transmission-daemon/
├── settings.json          # 主配置文件
├── torrents/              # 种子文件(.torrent)
│   ├── *.torrent
│   └── *.resume           # 进度文件
├── blocklists/            # IP黑名单
│   └── blocklist.bin
├── certs/                 # SSL证书（如果启用HTTPS）
├── resume/                # 断点续传数据
└── logs/                  # 日志文件
```

### 6.2 settings.json 示例

```json
{
    "alt-speed-down": 500,
    "alt-speed-enabled": false,
    "alt-speed-time-begin": 540,
    "alt-speed-time-day": 127,
    "alt-speed-time-enabled": false,
    "alt-speed-time-end": 1020,
    "alt-speed-up": 50,
    "bind-address-ipv4": "0.0.0.0",
    "bind-address-ipv6": "::",
    "blocklist-enabled": false,
    "blocklist-url": "http://www.example.com/blocklist",
    "cache-size-mb": 4,
    "download-dir": "/var/lib/transmission-daemon/downloads",
    "download-queue-enabled": true,
    "download-queue-size": 5,
    "dht-enabled": true,
    "encryption": 1,
    "idle-seeding-limit": 30,
    "idle-seeding-limit-enabled": false,
    "incomplete-dir": "/var/lib/transmission-daemon/downloads",
    "incomplete-dir-enabled": false,
    "lpd-enabled": true,
    "message-level": 2,
    "peer-congestion-algorithm": "",
    "peer-limit-global": 200,
    "peer-limit-per-torrent": 50,
    "peer-port": 51413,
    "peer-port-random-high": 65535,
    "peer-port-random-low": 49152,
    "peer-port-random-on-start": false,
    "pex-enabled": true,
    "port-forwarding-enabled": true,
    "preallocation": 1,
    "prefetch-enabled": true,
    "ratio-limit": 2,
    "ratio-limit-enabled": false,
    "rename-partial-files": true,
    "rpc-authentication-required": false,
    "rpc-bind-address": "0.0.0.0",
    "rpc-enabled": true,
    "rpc-host-whitelist": "",
    "rpc-host-whitelist-enabled": true,
    "rpc-password": "{abc123def456}",  // 加盐哈希
    "rpc-port": 9091,
    "rpc-url": "/transmission/",
    "rpc-username": "",
    "rpc-whitelist": "127.0.0.1,::1",
    "rpc-whitelist-enabled": true,
    "scrape-paused-torrents-enabled": true,
    "seed-queue-enabled": false,
    "seed-queue-size": 10,
    "speed-limit-down": 100,
    "speed-limit-down-enabled": false,
    "speed-limit-up": 100,
    "speed-limit-up-enabled": false,
    "start-added-torrents": true,
    "trash-original-torrent-files": false,
    "umask": 18,
    "upload-slots-per-torrent": 8,
    "utp-enabled": true
}
```

### 6.3 动态配置更新

```bash
# 方法1：通过RPC动态更新（无需重启）
transmission-remote --session-set \
    --download-dir=/new/path \
    --peer-limit-global=500

# 方法2：直接编辑settings.json后发送SIGHUP
kill -HUP $(pidof transmission-daemon)

# 方法3：使用WebUI修改
# 访问 http://localhost:9091 → 设置 → 保存
```

---

## 7. 安全机制

### 7.1 安全架构

```
┌─────────────────────────────────────────────┐
│           Transmission 安全防御              │
├─────────────────────────────────────────────┤
│                                             │
│  L4: 认证层                                │
│     ├── 用户名/密码（加盐SHA1）              │
│     ├── Session-ID（CSRF防护）              │
│     └── Token认证                          │
│                                             │
│  L3: 访问控制                              │
│     ├── IP白名单                            │
│     ├── 主机名白名单                        │
│     └── Bind Address                       │
│                                             │
│  L2: 暴力破解防护                          │
│     ├── Anti-Brute-Force                   │
│     ├── 可配置失败阈值                      │
│     └── 自动封禁                           │
│                                             │
│  L1: 网络层                                │
│     ├── Socket权限(0750)                   │
│     └── 端口限制                           │
│                                             │
└─────────────────────────────────────────────┘
```

### 7.2 认证机制

**密码存储（加盐）：**

```c
// transmission使用的密码哈希算法
// 不是明文存储，而是加盐后的单向哈希
char const* tr_sessionGetRPCPassword(tr_session const* session);

// 设置密码时自动加盐
void tr_sessionSetRPCPassword(tr_session* session, char const* password);
```

**注意：** Transmission的密码哈希不如qBittorrent的PBKDF2安全，但足够一般用途。

### 7.3 反暴力破解

**配置项：**

```json
{
    "anti-brute-force-enabled": true,
    "anti-brute-force-threshold": 100
}
```

**工作机制：**
- 追踪每个IP的失败尝试次数
- 达到阈值后暂时拒绝请求
- 与IP白名单配合使用效果更佳

### 7.4 生产环境安全配置

```bash
# /etc/init.d/transmission-daemon 或 systemd service

# 1. 启用认证
transmission-remote --auth \
    --username=admin \
    --password=your_secure_password

# 2. 设置白名单（仅允许内网访问）
transmission-remote --whitelist="192.168.*.*,10.*.*.*,127.0.0.1"

# 3. 启用反暴力破解
transmission-remote --anti-brute-force-enable \
    --anti-brute-force-threshold=20

# 4. 修改默认端口（可选）
transmission-remote --rpc-port=9092

# 5. 使用反向代理+HTTPS（推荐）
# Nginx配置示例见第11节
```

---

## 8. 性能特性

### 8.1 资源占用对比

| 指标 | Transmission | qBittorrent | 比例 |
|------|-------------|-------------|------|
| **空载内存** | ~30MB | ~120MB | **1:4** |
| **运行内存(10种子)** | ~50MB | ~200MB | **1:4** |
| **运行内存(100种子)** | ~80MB | ~350MB | **1:4.4** |
| **启动时间** | <1秒 | 2-3秒 | **3x快** |
| **CPU空闲** | ~0% | ~0.5% | - |
| **磁盘缓存** | 4MB(默认) | 自定义 | 可配置 |

### 8.2 性能优化配置

**针对不同场景：**

#### 场景1：NAS/嵌入式（内存<512MB）

```json
{
    "cache-size-mb": 2,
    "peer-limit-global": 100,
    "peer-limit-per-torrent": 20,
    "download-queue-size": 2,
    "prefetch-enabled": false,
    "pex-enabled": false,  // 减少开销
    "lpd-enabled": false
}
```

#### 场景2：家用服务器（内存1-4GB）

```json
{
    "cache-size-mb": 8,
    "peer-limit-global": 300,
    "peer-limit-per-torrent": 50,
    "download-queue-size": 5,
    "seed-queue-size": 10,
    "prefetch-enabled": true
}
```

#### 场景3：高性能服务器（内存>8GB）

```json
{
    "cache-size-mb": 32,
    "peer-limit-global": 1000,
    "peer-limit-per-torrent": 200,
    "download-queue-size": 20,
    "seed-queue-size": 50,
    "prefetch-enabled": true,
    "upload-slots-per-torrent": 20
}
```

### 8.3 磁盘IO优化

**预分配模式对比：**

| 模式 | 速度 | 空间碎片 | 适用场景 |
|------|------|---------|----------|
| `none` | 最快 | 多 | 临时下载/测试 |
| `sparse` | 快 | 少 | **推荐**（大多数场景） |
| `full` | 慢 | 无 | 需要避免碎片化 |

**缓存策略：**

```json
{
    "cache-size-mb": 16,        // 缓存大小
    "prefetch-enabled": true    // 预读取下一块
}
```

**PT大文件场景建议：**
- 缓存设置为物理内存的 1-2%
- 启用预读取（顺序读写优化）
- 使用稀疏预分配（平衡速度和空间）

---

## 9. PT 场景应用

### 9.1 命令行自动化

**transmission-remote 常用命令：**

```bash
# ====== 基本连接 ======
transmission-remote localhost:9091 \
    --auth admin:password

# ====== 添加种子 ======
# 从URL添加
transmission-remote -a "http://pt.site.com/download.php?id=123&passkey=xxx"

# 从文件添加
transmission-remote -w /downloads/movie/ \
    -g /path/to/file.torrent

# 从磁力链接添加
transmission-remote -m "magnet:?xt=urn:btih:INFOHASH"

# ====== 查询状态 ======
# 列出所有种子
transmission-remote -l

# 详细信息
transmission-remote -t <id> -i

# 文件列表
transmission-remote -t <id> -f

# Peers信息
transmission-remote -t <id> -pr

# Trackers信息
transmission-remote -t <id> -tr

# ====== 控制 ======
# 开始/暂停
transmission-remote -t <id> -s/-S

# 删除（保留文件）
transmission-remote -t <id> -rd

# 删除（删除文件）
transmission-remote -t <id> -rad

# ====== 限速 ======
# 全局限速
transmission-remote --dlimit 500 --ulimit 200

# 单种子限速
transmission-remote -t <id> --dlimit 1000 --ulimit 500

# ====== 高级操作 ======
# 设置优先级
transmission-remote -t <id> -phigh 1,2,3

# 移动数据
transmission-remote -t <id> --move /new/location/

# 设置Tracker
transmission-remote -t <id> --tracker-add "http://tracker.example.com:2710/announce"
```

### 9.2 Python 自动化脚本

```python
import requests
import json

TRANS_URL = "http://localhost:9091/transmission/rpc"
USER = "admin"
PASS = "password"

class TransmissionClient:
    def __init__(self):
        self.session = requests.Session()
        self.session.auth = (USER, PASS)
        self._get_session_id()

    def _get_session_id(self):
        """获取Session-ID"""
        resp = self.session.post(TRANS_URL, json={})
        if resp.status_code == 409:
            self.session.headers.update({
                'X-Transmission-Session-Id': resp.headers['X-Transmission-Session-Id']
            })

    def _request(self, method, arguments={}):
        """发送RPC请求"""
        data = {"method": method, "arguments": arguments}
        resp = self.session.post(TRANS_URL, json=data)
        return resp.json()

    def add_torrent(self, url, download_dir=None, paused=False):
        """添加种子"""
        args = {"filename": url, "paused": paused}
        if download_dir:
            args["download-dir"] = download_dir
        return self._request("torrent-add", args)

    def get_torrents(self, ids=None, fields=None):
        """查询种子"""
        args = {}
        if ids:
            args["ids"] = ids if isinstance(ids, list) else [ids]
        if fields:
            args["fields"] = fields
        return self._request("torrent-get", args)

    def remove_torrent(self, tid, delete_local=False):
        """删除种子"""
        return self._request("torrent-remove", {
            "ids": [tid],
            "delete-local-data": delete_local
        })

    def start_torrent(self, tid):
        """开始种子"""
        return self._request("torrent-start", {"ids": [tid]})

    def stop_torrent(self, tid):
        """暂停种子"""
        return self._request("torrent-stop", {"ids": [tid]})

    def set_session(self, **kwargs):
        """修改会话设置"""
        return self._request("session-set", kwargs)

    def get_session_stats(self):
        """获取统计信息"""
        return self._request("session-stats")

# 使用示例
if __name__ == "__main__":
    client = TransmissionClient()

    # 添加种子
    result = client.add_torrent(
        url="http://pt.site.com/download.php?id=45678&passkey=xxx",
        download_dir="/downloads/movies/"
    )
    print(f"添加结果: {result}")

    # 查询所有活跃种子
    torrents = client.get_torrents(ids="recently-active")
    for t in torrents["arguments"]["torrents"]:
        print(f"{t['name']}: {t['percentDone']*100:.1f}% "
              f"(↓{t['rateDownload']/1024:.1f}KB/s ↑{t['rateUpload']/1024:.1f}KB/s)")

    # 统计信息
    stats = client.get_session_stats()
    cumul = stats["arguments"]["cumulative-stats"]
    print(f"\n总上传: {cumul['uploadedBytes']/1024**3:.2f} GB")
    print(f"总下载: {cumul['downloadedBytes']/1024**3:.2f} GB")
```

### 9.3 Shell 脚本集成

```bash
#!/bin/bash
# transmission_pt_manager.sh
# PT站点自动管理脚本

TRANS_HOST="localhost:9091"
TRANS_USER="admin"
TRANS_PASS="password"

# 辅助函数
call_transmission() {
    transmission-remote ${TRANS_HOST} \
        --auth ${TRANS_USER}:${TRANS_PASS} "$@"
}

# 1. 显示当前状态
show_status() {
    echo "=== Transmission 状态 ==="
    call_transmission -l
    echo ""
    echo "=== 全局统计 ==="
    call_transmission -st
}

# 2. 添加种子（从PT站RSS）
add_from_rss() {
    local url=$1
    local category=$2
    local save_path="/downloads/${category}/"

    echo "[ADD] $url → $save_path"
    call_transmission -w "$save_path" -a "$url"
}

# 3. 清理已完成且分享率达标