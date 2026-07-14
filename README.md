# SWELL Box

**SWELL Box** is a lightweight system-tray client for [sing-box](https://github.com/SagerNet/sing-box).

**Repository:** [github.com/yaog6700-bit/Swell-Box](https://github.com/yaog6700-bit/Swell-Box)

Brand: **SWELL** · Binary: `SWELL-Box` / `SWELL-Box.exe`

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
- Import node (`ss://` / `vless://`), subscription URL, or full config JSON
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
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-H=windowsgui -s -w" -o dist/SWELL-Box.exe ./cmd/swellbox
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=1 go build -ldflags "-s -w" -o dist/SWELL-Box ./cmd/swellbox
GOOS=linux   GOARCH=amd64 CGO_ENABLED=1 go build -ldflags "-s -w" -o dist/SWELL-Box ./cmd/swellbox
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

**Release assets (5 platforms):**

| File | Contents |
|------|----------|
| `SWELL-Box-windows-amd64-full.zip` | Win x64 client + `sing-box.exe` |
| `SWELL-Box-windows-arm64-full.zip` | Win ARM64 client + `sing-box.exe` |
| `SWELL-Box-darwin-arm64-full.zip` | macOS Apple Silicon **`.app`** + `sing-box`（无终端、带图标） |
| `SWELL-Box-linux-amd64-full.zip` | Linux x64 + `sing-box` |
| `SWELL-Box-linux-arm64-full.zip` | Linux ARM64 + `sing-box` |

`sing-box` is **only inside the full zip**, not uploaded as a separate Release file.

Also published: thin clients `SWELL-Box-<os>-<arch>[.exe]` (no core; Start can download if online).

## Usage

1. Extract the full zip for your platform
2. Run:
   - **Windows:** `SWELL-Box.exe`
   - **macOS:** open `SWELL Box.app` (menu bar only; first time: right-click → Open)
   - **Linux:** `./SWELL-Box`
3. **Add** → import node / subscription / config
4. **Start**
5. **Dashboard** for connections / selectors

Custom configs: put `config*.json` under `~/.swellbox/` or use **Import Config File**. Routing in imported files is left as-is.

## License

MIT — see [LICENSE](LICENSE).

sing-box is GPL-3.0; ship or download the official binary separately (this app can auto-download it).

## Disclaimer

For technical and educational use. Comply with local laws and your network provider’s terms.
