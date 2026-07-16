# Swell-Box

**Swell-Box** is a lightweight system-tray client for [sing-box](https://github.com/SagerNet/sing-box).

**Repository:** [github.com/yaog6700-bit/Swell-Box](https://github.com/yaog6700-bit/Swell-Box)

Brand: **Swell** · Binary: `Swell-Box` / `Swell-Box.exe`

> Architecture: **tray shell (Go) + official sing-box binary (subprocess) + official Dashboard**.  
> Inspired by [daodao97/SingBoxClient](https://github.com/daodao97/SingBoxClient), rewritten without vendoring the core.

## Supported platforms

| OS | Architectures | Notes |
|----|---------------|--------|
| **Windows** | amd64, arm64 | Full tray + system proxy + autostart |
| **macOS** | arm64 only | Apple Silicon (`M1/M2/M3…`) |
| **Linux** | amd64, arm64 | Needs tray/appindicator libs at runtime |

## Features

- Start / Stop / Restart proxy from the tray
- Official Dashboard (`http://127.0.0.1:9091/dashboard/`)
- Import node share links (`ss` / `vmess` / `vless` / `trojan` / `hysteria` / `hysteria2` / `tuic` / `anytls` / `wireguard` / `socks` / `http` / `snell` / `naive` / `ssh`), subscription URL, or full config JSON
- Multi-config switch + open in editor
- Default split routing: CN direct / others via proxy (local `geosite-cn.srs` / `geoip-cn.srs`)
- Update core (stable / pre-release), update Geo rules
- Launch at login, auto-connect, system proxy
- **TUN mode** (tray toggle; runtime inject; prefer run as admin)
- Chinese / English UI

## Requirements

- Windows 10+, macOS 12+ (Apple Silicon), or a modern Linux desktop
- Network once if core is missing (auto-download official `sing-box`)

User data: `~/.swellbox/` (Windows: `%USERPROFILE%\.swellbox\`)

## Build

### Local (Windows)

```powershell
# Go 1.22+
cd swellbox
go mod tidy
.\scripts\build.ps1
```

### Cross targets (any OS with Go)

```bash
# examples
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-H=windowsgui -s -w" -o dist/Swell-Box.exe ./cmd/swellbox
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=1 go build -ldflags "-s -w" -o dist/Swell-Box ./cmd/swellbox
GOOS=linux   GOARCH=amd64 CGO_ENABLED=1 go build -ldflags "-s -w" -o dist/Swell-Box ./cmd/swellbox
```

> Linux / macOS tray builds need CGO. On Linux install `libgtk-3-dev` and `libayatana-appindicator3-dev`.

### GitHub Actions (recommended)

| Workflow | Trigger | Output |
|----------|---------|--------|
| **CI** | push / PR to `main` | compile check for all 5 targets |
| **Release** | push tag `v*` | full offline zips + clients for all targets |

Create a release:

```bash
git tag v0.2.4
git push origin v0.2.4
```

**Release assets (5 files only — offline full packages):**

| File | Contents |
|------|----------|
| `Swell-Box-windows-amd64-full.zip` | Win x64 client + `sing-box.exe` |
| `Swell-Box-windows-arm64-full.zip` | Win ARM64 client + `sing-box.exe` |
| `Swell-Box-darwin-arm64-full.zip` | macOS Apple Silicon **`.app`** + `sing-box` |
| `Swell-Box-linux-amd64-full.zip` | Linux x64 + `sing-box` |
| `Swell-Box-linux-arm64-full.zip` | Linux ARM64 + `sing-box` |

No thin clients or bare binaries are published — only `*-full.zip`.

## Usage

1. Extract the full zip for your platform
2. Run:
   - **Windows:** `Swell-Box.exe`
   - **macOS:** drag `Swell-Box.app` into **Applications**, then open it (menu bar only).  
     First time: right-click → Open, or `xattr -cr /Applications/Swell-Box.app` if “damaged”.  
     The core (`sing-box`) is **inside** the `.app`; first launch copies it to `~/.swellbox/bin`. You do **not** need to keep the unzipped folder after installing.  
     **Updates:** tray → Update core (in-app). Tray → Check app update downloads a new `.app` and replaces the running bundle (then relaunches).
   - **Linux:** `./Swell-Box`
3. **Add** → import node / subscription / config
4. **Start** (menu bar icon)
5. **Dashboard** for connections / selectors

Custom configs: put `config*.json` under `~/.swellbox/` or use **Import Config File**. Routing in imported files is left as-is.

### macOS: “opened but no internet”

Usually **system proxy is still pointing at `127.0.0.1:7890`** while the core is not running (or a node is dead). Fix:

```bash
# 1) Turn off leftover system proxy (all services)
networksetup -listallnetworkservices
# then for your active interface, e.g. Wi-Fi:
networksetup -setwebproxystate "Wi-Fi" off
networksetup -setsecurewebproxystate "Wi-Fi" off
networksetup -setsocksfirewallproxystate "Wi-Fi" off

# 2) Check whether mixed proxy is listening
lsof -iTCP:7890 -sTCP:LISTEN

# 3) Core log
tail -n 80 ~/.swellbox/logs/core.log
```

In the tray menu:

1. Confirm you **imported a subscription / node** and selected a working node (not only `direct`).
2. Turn **System Proxy** on → **Start**.
3. **TUN on macOS:** tray menu → enable TUN → confirm → **Start** → enter Mac password when prompted (authorizes sing-box only). Or keep using **System Proxy** without TUN.
4. Open **Dashboard** (`http://127.0.0.1:9091/dashboard/`) — if it fails, the core is not up.

## License

MIT — see [LICENSE](LICENSE).

sing-box is GPL-3.0; ship or download the official binary separately (this app can auto-download it).

## Disclaimer

For technical and educational use. Comply with local laws and your network provider’s terms.
