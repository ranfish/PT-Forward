# PT 自动转发与发种系统 (PT Auto Transfer)

这是一款基于 Python 与 Docker 的全自动化 PT 刷流与发种工具。支持从 M-Team (MT) 或 TTG 等站点自动抓取符合规则的种子，自动推送到 qBittorrent 进行下载，下载完成后智能解析本地视频信息 (MediaInfo)、自动截取抽样截图并上传至免费图床，最后将其辅种发布至 HDArea (HDA) 或其他支持的主站点。

## ✨ 核心特性
- **高度自动化**：全流程无人值守（抓取 -> 下载 -> 解析 -> 截图 -> 传图床 -> 发布 -> 辅种）。
- **精准的 MediaInfo 嗅探**：自带 `ffmpeg` 及 `mediainfo`，精准地抓取本地视频实际的音视频编码，智能映射到 HDA 表单中，极低出错率。
- **自动获取外网信息**：支持对接豆瓣 API 与 IMDb API，自动生成优质的 BBCode 简介模板。
- **Web UI 监控板**：自带小型控制台，方便在网页端实时查看日志和队列任务状态（默认占用 8888 端口）。
- **空间/限速保护**：自动检测硬盘容量避免爆盘，自动对单种应用限速以确保安全稳定运行。

---

## 🛠 配置与运行环境要求
1. **Linux 服务器 / NAS / Windows (Docker 环境)**。
2. 安装并正在运行的 **qBittorrent**（用于接收下载任务）。
3. 已完整安装 **Docker** 和 **Docker Compose**。

---

## 🚀 部署教程

### 步骤 1：准备配置文件
进入你解压或克隆的源码目录，你会看到一个名为 `config.example.yaml` 的文件：
1. 复制或重命名该文件为 `config.yaml`。
2. 使用文本编辑器（如 VSCode 或记事本）打开 `config.yaml`。
3. 把文件里带有 `your_xxx_here` 或各种占位符的数据全部替换为你自己的：
   - **qbittorrent**: 修改 `host` (带上端口号)、`username`、`password` 等信息，确保程序能连接上您的 qB。
   - **sites**: 对应填入你在 HDArea 以及 TTG 的实际 `cookie` 字符串（按 F12 在浏览器网络抓包获取）。对于 M-Team，填入属于你的真实 `api_key`。
   - **web_ui**: 自定义一个后台登陆密码。
   
   **【提示】**：您也可以在重命名并在步骤 2 中配置完映射后，什么都不管直接无脑启动 Docker，然后在通过浏览器访问 8888 端口的可视化管理面板中填写以上信息并随时动态保存！

### 步骤 2：配置物理磁盘路径映射 (重点)
因为本程序基于 Docker 运行，为了使程序能准确找到 qBittorrent 下载的视频文件去抽样截图和扫库，必须统一路径转换。

1. **设置 `config.yaml` 中的 `path_mapping`**：
   假如您的 qBittorrent 显示的下载保存路径是 `/downloads/disk1`
   但实际映射给本程序容器读取的内测路径是 `/app/data`，则应保持在 config 里如此设置：
   ```yaml
   settings:
     path_mapping:
       /downloads/disk1: /app/data
   ```

2. **修正 `docker-compose.yml` 中的挂载点**：
   打开 `docker-compose.yml`，在 `volumes` 节下找到以下这一行需要你自己按需更改：
   ```yaml
       # 冒号左侧为您宿主机的实际电影下载归档目录，右侧固定为 /app/data
       - /root/ardtu/pt:/app/data
   ```
   **注意**：一定要确保程序对该目录具有**读取权限**，否则程序无法计算 `mediainfo` 与提取截图。

### 步骤 3：一键构建和启动服务
所有配置调整完毕后，在终端（命令行）中导航到当前目录，直接运行：

```bash
docker compose up -d --build
```
> 如果您的 Docker 版本较老，命令可能是 `docker-compose up -d --build`。

此命令会自动依据 `Dockerfile` 构建系统依赖库（安装 libmediainfo、ffmpeg 等）并启动常驻服务。

---

## 📊 管理面板
系统启动后，在浏览器访问容器映射的端口：
- **地址**: `http://你的IP地址:8888/`
- **密码**: 在 `config.yaml` 的 `web_ui` 字典下设置的字符串密码。

您可以进入 Web UI 查阅最新的执行日志，进行调试或查看队列中处理卡死的任务。

---

## 常见问题与排错 (FAQ)

**Q：为什么 qBittorrent 添加了种子却显示“0%”或“校验中”，导致任务停在那里？**
程序会尝试免校验强行推入正在辅种的 Hash 任务。请确保程序的 `config.yaml` 设置中给 qBittorrent 推送任务的 `save_path` 和你宿主机/服务器现有种子的物理目录严格一致且拥有读取权限。

**Q：有些种子发布失败提示“无法定位视频主文件”？**
请检查您是否按照"步骤 2"严格配置了 qBittorrent 到 `path_mapping`，并且挂载目录正确。程序默认通过寻找体积最大的结尾为 `.mkv/.mp4/.m2ts/.ts` 的文件判断为主视频，若您的主文件被压缩包打包没有解烂，它将抛弃此任务。

**Q：如何更新配置的限速规则或者挂载？**
修改 `config.yaml` 以后，并不需要重启整个 Docker 容器，主引擎 (`main.py`) 每 120 秒会自动热重载限速配置及映射。但如果你修改了 `docker-compose.yml` 或者安装环境，那么仍需使用 `docker compose up -d --build` 重新起容器。
