# SWELL Box

**SWELL Box** is a lightweight Windows system-tray client for [sing-box](https://github.com/SagerNet/sing-box).

Brand: **SWELL** · Binary: `swellbox` / `SWELL-Box.exe`

> Architecture: **tray shell (Go) + official sing-box binary (subprocess) + official Dashboard**.  
> Inspired by [daodao97/SingBoxClient](https://github.com/daodao97/SingBoxClient), rewritten without vendoring the core.

## Features

- Start / Stop / Restart proxy from the tray
- Official Dashboard (`http://127.0.0.1:9091/dashboard/`)
- Import node (`ss://` / `vless://`), subscription URL, or full config JSON
- Multi-config switch + open in Notepad
- Default split routing: CN direct / others via proxy (local `geosite-cn.srs` / `geoip-cn.srs`)
- Update core (stable / pre-release), update Geo rules
- Launch at login, auto-connect, system proxy
- Chinese / English UI

## Requirements

- Windows 10+
- Network once if core is missing (auto-download official `sing-box`)

User data: `%USERPROFILE%\.swellbox\`

## Build

### Local (Windows)

```powershell
# Go 1.22+
cd swellbox
go mod tidy
.\scripts\build.ps1
```

### GitHub Actions (recommended)

| Workflow | Trigger | Output |
|----------|---------|--------|
| **CI** | push / PR to `main` | compile check |
| **Release** | push tag `v*` | full offline zip + client |

Create a release:

```bash
git tag v0.2.2
git push origin v0.2.2
```

**Release assets:**

| File | Contents |
|------|----------|
| `SWELL-Box-windows-amd64-full.zip` | **Recommended** — `SWELL-Box.exe` + `sing-box.exe` (no download needed) |
| `SWELL-Box.exe` | Client only (Start will use next-to-exe core, or download if online) |

Put `sing-box.exe` in the **same folder** as `SWELL-Box.exe` for offline use.

Adding nodes does not download the core; **Start** does (local first, then network).## Usage

1. Run `SWELL-Box.exe` (tray icon)
2. **Add** → import node / subscription / config
3. **Start**
4. **Dashboard** for connections / selectors

Custom configs: put `config*.json` under `%USERPROFILE%\.swellbox\` or use **Import Config File**. Routing in imported files is left as-is.

## License

MIT — see [LICENSE](LICENSE).

sing-box is GPL-3.0; ship or download the official binary separately (this app can auto-download it).

## Disclaimer

For technical and educational use. Comply with local laws and your network provider’s terms.
