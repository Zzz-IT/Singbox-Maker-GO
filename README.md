
# Singbox Maker GO

Singbox Maker GO 是一款专为现代网络环境设计、基于 Go 语言构建的 Sing-box 代理核心控制平面与节点编排面板。

本项目旨在为 Linux 基础设施（涵盖低配轻量应用服务器及 Alpine 极简容器环境）提供零依赖、极低资源开销且具备高度容错与进程级自愈能力的代理服务管控方案。从多协议节点部署、Cloudflare Argo 穿透编排，到复杂的系统守护进程接管，Singbox Maker GO 致力于提供开箱即用的企业级路由管理体验。

---

## 核心架构优势 (Architectural Advantages)

从传统 Shell 脚本迁移至 Go 语言的重构，为项目带来了系统级的性能跃升：

* **零外部依赖 (Zero Dependencies)**：无需预装 Python、Node.js 甚至基础的网络请求工具 (curl/wget)。单个静态编译的二进制文件即可在任何纯净的 Linux 环境中独立运行。
* **内存极客优化 (Memory Optimization)**：底层核心更新采用网络流直接解压技术。无需将全量 `.tar.gz` 存档落盘后再解压，显著降低小内存实例的磁盘 I/O 压力与 OOM (Out Of Memory) 风险。
* **原子化配置写入 (Atomic Write)**：针对 JSON、YAML 等关键配置及元数据文件的修改，全面引入原子化写入机制。通过先写入 `.tmp` 临时文件再执行系统级重命名 (Rename) 的操作，彻底杜绝因意外断电、内核 Panic 或磁盘耗尽导致的配置清零灾难。

---

## 核心特性 (Core Features)

### 1. 进程级自愈体系 (Self-Healing System)
传统面板在遭遇进程非正常退出或锁文件残留时易发生服务死锁。本项目深度接管了 Systemd 与 OpenRC (专为 Alpine 优化) 守护进程：
* **安全重启链路**：在执行重启或停止指令时，主动执行 `pkill -9` 终止挂起进程，强制回收 `/var/run/sing-box.pid` 锁文件，并下发状态重置 (`zap` / `reset-failed`) 指令，确保每次启动均处于绝对隔离的干净态。
* **依赖缺失静默自愈**：当检测到运行于纯净 Alpine 且缺少 glibc 兼容层时，面板将在后台静默拉取并安装 `gcompat` 与 `tzdata`，全程无需人工干预。

### 2. 全协议矩阵栈 (Protocol Matrix)
支持一键编排抗封锁协议，并自动生成 URL Encode 安全编码的 URI 分享链接与 `clash.yaml` 代理拓扑：
* **VLESS 系列**: VLESS-Reality (动态生成公私钥对与 ShortID)、VLESS-WS-TLS、VLESS-TCP
* **Trojan 系列**: Trojan-WS-TLS
* **次世代 UDP**: Hysteria2 (支持 QUIC 端口混淆)、TUICv5 (BBR 拥塞控制引擎)
* **经典协议**: Shadowsocks (aes-256-gcm / ss-2022)、AnyTLS、SOCKS5

### 3. Cloudflare 生态整合 (Cloudflare Ecosystem)
* **Argo Tunnel 独立控制流**：支持无公网 IP 环境下的流量穿透。兼容临时试用域名 (trycloudflare) 与 Token 绑定的固定隧道配置，守护进程生命周期与数据主核心完全解耦。
* **ECH 防阻断注入 (Client ECH)**：提供全局 ECH 注入开关。启用后，CDN 节点的底层请求将自动把 SNI 伪装为 `cloudflare-ech.com`，有效抵御针对明文 SNI 的深度包检测 (DPI)。

### 4. 路由与策略控制 (Routing & Policies)
* **DNS 策略分流**：支持一键切换“国外解析优先”与“国内解析优先” DNS 组，自动生成标准化的 DoH 路由解析树。
* **出站 IP 偏好**：支持动态切换 `prefer_ipv6` 与 `prefer_ipv4` 出站策略，满足纯 IPv6 宿主机或特定流媒体解锁环境需求。

---

## 部署指南 (Deployment)

请使用 **root** 权限在目标 Linux 服务器（支持 amd64 / arm64 架构）上执行以下初始化脚本：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/Zzz-IT/singbox-maker-go/main/install.sh)
```

> **注**：安装程序将自动探测 CPU 架构并部署最新的 Go 编译二进制文件，随后完成基础目录的构建。

---

## 操作手册 (Operations)

部署完毕后，在终端执行 `sb` 唤出交互式控制台：

```bash
sb
```

### 命令行静默调用 (CLI Mode)
面向自动化运维场景，面板支持直接通过命令行参数执行核心动作，无需进入交互菜单：
* `sb start` —— 拉起代理服务
* `sb stop` —— 停止并深度清理残留进程
* `sb restart` —— 执行进程级安全重启 (强制清理并重新拉起)

---

## 目录规范 (Directory Standard)

系统严格遵循 Linux FHS (Filesystem Hierarchy Standard) 规范，配置与日志物理隔离：

| 路径 | 用途 |
| :--- | :--- |
| `/usr/local/bin/sb` | 控制平面核心程序 (Go 二进制文件) |
| `/usr/local/bin/sing-box` | Sing-box 数据平面引擎 |
| `/usr/local/bin/cloudflared` | Cloudflare Argo 穿透引擎 |
| `/usr/local/etc/sing-box/` | **配置中心根目录** |
| `.../config.json` | Sing-box 原生渲染配置 |
| `.../clash.yaml` | Clash/Mihomo 订阅自动同步配置 |
| `.../metadata.json` | 节点元数据存储 (UUID, 密钥等状态信息) |
| `.../argo_metadata.json`| Argo 隧道端口映射与 Token 状态 |
| `/var/log/sing-box.log` | 服务运行日志 (非 Systemd 环境) |

---

## 声明 (Disclaimer)

本项目仅供网络协议原理研究、Go 语言开发实践及 Linux 系统级管理学习使用。使用者应严格遵守所在国家或地区的法律法规。切勿将本项目用于任何非法用途，开发者对项目使用过程中产生的任何法律责任概不负责。
