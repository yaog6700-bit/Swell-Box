# Swell-Box

**Swell-Box** 是一款轻量、现代且易用的跨平台系统托盘代理客户端，原生支持 **sing-box** 与 **Shoes** 双核心架构，并内置完全离线的本地控制面板。

**Swell-Box** is a lightweight, modern cross-platform system-tray proxy client powered by a dual-engine architecture ([sing-box](https://github.com/SagerNet/sing-box) & [shoes](https://github.com/yaog6700-bit/shoes)), featuring an integrated offline dashboard.

**Repository:** [github.com/yaog6700-bit/Swell-Box](https://github.com/yaog6700-bit/Swell-Box)  
**Binary:** `Swell-Box` / `Swell-Box.exe`

> 💡 **项目渊源 (Upstream)**：本项目 Fork 并基于开源项目 [daodao97/SingBoxClient](https://github.com/daodao97/SingBoxClient) 进行深度重构与功能拓展，在此对原作者 [daodao97](https://github.com/daodao97) 的开源奉献致以由衷感谢！  
> Forked and rewritten from [daodao97/SingBoxClient](https://github.com/daodao97/SingBoxClient). Special thanks to the original author for the pioneering work!  
> 
> **架构特点**：轻量托盘外壳 (Go) + 官方内核子进程 (官方独立编译，零侵入无 Vendor) + 内置轻量离线面板。

---

## 🌟 核心新功能亮点 (Key Features)

### 1. 🚀 双核心自由切换 (Dual-Core Engine)
- **多核心无缝切换**：在托盘菜单中即可随时在 **sing-box** 与 **Shoes** 之间一键平滑切换，无需手动修改复杂配置。
- **Shoes (Rust) 内核深度集成**：支持基于 Rust 的高性能多协议代理引擎 [yaog6700-bit/shoes](https://github.com/yaog6700-bit/shoes)（Fork 自 [cfal/shoes](https://github.com/cfal/shoes)），具备极佳的内存占用与并发性能。
- **配置智能无缝转译**：基于 sing-box JSON 格式自动转译生成 Shoes 专用的 YAML 运行态配置，节点和路由策略保持同步。
- **适配 Shoes 0.3.5+ 规范与自动自愈**：
  - 适配 Shoes 0.3.5+ 全新分组规范（`type: selector`），并在启动检查中内置双向版本自愈重试机制，无缝兼容新旧版本。
  - **完整支持 Shadowsocks 2022 Multi-User**：支持 `<server-key>:<user-key>` 双密钥密码格式，SS2022 节点零过滤全量可用。
- **内核独立在线热更新**：支持在托盘内一键检查并在线更新 sing-box（Stable / Pre-release）与 Shoes 内核至官方最新版。

### 2. 📊 内置离线控制面板 (Built-in Offline Dashboard)
- **100% 离线可用**：使用 Go `embed` 内嵌轻量化 Web 控制台（默认监听 `127.0.0.1:9092`），在没有外网或初次配网阶段即可顺畅打开。
- **零配置直达**：托盘点击 **Dashboard** 自动携带当前的 Clash API 端口与 Secret 鉴权凭据，一键打开即连接。
- **可视化节点与流量掌控**：支持节点选择与延迟测速、实时速率/内存监控、活跃连接追踪与一键打断。

### 3. 🛡️ 智能分流与 TUN / 系统代理
- **默认智能分流**：国内常见域名（`.cn` 及本地 `geosite-cn.srs`）和内网/国内 IP（`geoip-cn.srs`）自动直连，其余流量走代理。
- **TUN 虚拟网卡模式**：托盘一键启闭 TUN 模式（支持管理员权限提权启动，兼容 sing-box 1.15+ 最新规范）。
- **系统代理智能管理**：启动/退出自动设置或清除系统代理；当 TUN 模式接管时智能互斥，防止网络冲突。

### 4. 🔗 广泛的节点与订阅协议支持
- 一键导入节点分享链接、在线订阅 URL（自动定时更新），或载入完整 JSON 配置文件。
- 协议支持矩阵覆盖：`Shadowsocks` (含 2022 单密钥与双密钥多用户)、`VMess` (WebSocket / TLS)、`VLESS` (Reality / Vision / TLS)、`Trojan`、`Hysteria`、`Hysteria2`、`TUIC`、`AnyTLS`、`WireGuard`、`SOCKS5`、`HTTP` 等。

---

## 🖥️ 支持平台 (Supported Platforms)

| 操作系统 (OS) | 架构 (Arch) | 说明 (Notes) |
|---|---|---|
| **Windows** | x86_64 (`amd64`), ARM64 (`arm64`) | 托盘全功能、高清 DPI Manifest、开机自启、无黑框控制台 |
| **macOS** | ARM64 (`arm64`) | 原生支持 Apple Silicon (`M1/M2/M3/M4`)，打包为独立 `.app` |
| **Linux** | x86_64 (`amd64`) | 需系统桌面环境提供 libayatana-appindicator / GTK 支持 |

---

## 📦 下载与运行 (Quick Start)

从 [GitHub Releases](https://github.com/yaog6700-bit/Swell-Box/releases) 下载对应平台的离线完整包：

- **Windows:** 解压 `Swell-Box-windows-amd64-full.zip`，直接双击运行 `Swell-Box.exe`。
- **macOS:** 解压 `Swell-Box-darwin-arm64-full.zip`，将 `Swell-Box.app` 拖入 **Applications** 启动（首次启动如提示损坏，在终端执行 `xattr -cr /Applications/Swell-Box.app` 即可）。
- **Linux:** 解压对应压缩包后赋予执行权限 `./Swell-Box` 启动。

> **数据存储目录**：所有配置文件、规则集与日志集中存放于用户目录下的 `~/.swellbox/`（Windows 为 `%USERPROFILE%\.swellbox\`）。

---

## 🛠️ 本地编译 (Build from Source)

### 本地编译 Windows 版（推荐）
```powershell
# 依赖 Go 1.22+
cd swellbox
.\scripts\build.ps1
# 编译产物位于 dist/Swell-Box.exe (含嵌入图标、DPI Manifest 及无控制台窗优化)
```

### 跨平台编译
```bash
# Windows x64
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-H=windowsgui -s -w" -o dist/Swell-Box.exe ./cmd/swellbox

# macOS ARM64
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=1 go build -ldflags "-s -w" -o dist/Swell-Box ./cmd/swellbox

# Linux x64
GOOS=linux   GOARCH=amd64 CGO_ENABLED=1 go build -ldflags "-s -w" -o dist/Swell-Box ./cmd/swellbox
```

---

## 🙏 鸣谢与致敬 (Acknowledgements)

- 🌟 **[daodao97/SingBoxClient](https://github.com/daodao97/SingBoxClient)**：感谢原项目作者 [daodao97](https://github.com/daodao97) 提供的开创性工作与坚实基础。
- 📦 **[SagerNet/sing-box](https://github.com/SagerNet/sing-box)**：感谢 sing-box 官方团队打造的高性能通用通用代理平台。
- 👟 **[cfal/shoes](https://github.com/cfal/shoes) / [yaog6700-bit/shoes](https://github.com/yaog6700-bit/shoes)**：感谢作者提供的高性能 Rust 多协议代理引擎。

---

## 📄 开源许可证 (License)

本项目采用 [MIT License](LICENSE) 开源。  
使用的 sing-box 与 Shoes 内核归各自原作者所有，遵循其对应的开源许可协议。

---

## ⚠️ 免责声明 (Disclaimer)

本项目仅供网络技术学习、科研与交流使用。请自觉遵守所在国家和地区的相关法律法规。
