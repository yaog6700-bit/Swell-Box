#!/usr/bin/env bash
# Build a proper macOS .app bundle (icon + no Terminal + menu-bar agent).
# Usage: package_macos_app.sh <binary> <out_app_dir> [version]
set -euo pipefail

BIN="${1:?binary path}"
OUT_APP="${2:?output .app path}"
VERSION="${3:-0.0.0}"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# App icon: same black pickaxe as Windows process icon (not the color brand logo).
PNG="${ROOT}/internal/seed/icon.png"
if [[ ! -f "$PNG" ]]; then
  PNG="${ROOT}/internal/seed/logo.png"
fi
if [[ ! -f "$BIN" ]]; then
  echo "binary not found: $BIN" >&2
  exit 1
fi
if [[ ! -f "$PNG" ]]; then
  echo "icon png not found" >&2
  exit 1
fi

rm -rf "$OUT_APP"
mkdir -p "$OUT_APP/Contents/MacOS" "$OUT_APP/Contents/Resources"

cp "$BIN" "$OUT_APP/Contents/MacOS/Swell-Box"
chmod +x "$OUT_APP/Contents/MacOS/Swell-Box"

# App icon (.icns) via sips + iconutil (macOS only)
WORKDIR="$(mktemp -d)"
ICONSET="$WORKDIR/AppIcon.iconset"
mkdir -p "$ICONSET"
make_icon() {
  local size="$1" name="$2"
  sips -z "$size" "$size" "$PNG" --out "$ICONSET/$name" >/dev/null
}
make_icon 16   icon_16x16.png
make_icon 32   icon_16x16@2x.png
make_icon 32   icon_32x32.png
make_icon 64   icon_32x32@2x.png
make_icon 128  icon_128x128.png
make_icon 256  icon_128x128@2x.png
make_icon 256  icon_256x256.png
make_icon 512  icon_256x256@2x.png
make_icon 512  icon_512x512.png
make_icon 1024 icon_512x512@2x.png
iconutil -c icns "$ICONSET" -o "$OUT_APP/Contents/Resources/AppIcon.icns"
rm -rf "$WORKDIR"

# LSUIElement=true → menu-bar app: no Dock icon, double-click won't open Terminal
cat > "$OUT_APP/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key>
  <string>en</string>
  <key>CFBundleExecutable</key>
  <string>Swell-Box</string>
  <key>CFBundleIdentifier</key>
  <string>com.swellbox.app</string>
  <key>CFBundleInfoDictionaryVersion</key>
  <string>6.0</string>
  <key>CFBundleName</key>
  <string>Swell-Box</string>
  <key>CFBundleDisplayName</key>
  <string>Swell-Box</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>${VERSION}</string>
  <key>CFBundleVersion</key>
  <string>${VERSION}</string>
  <key>CFBundleIconFile</key>
  <string>AppIcon</string>
  <key>LSMinimumSystemVersion</key>
  <string>12.0</string>
  <key>LSUIElement</key>
  <true/>
  <key>NSHighResolutionCapable</key>
  <true/>
  <key>NSSupportsAutomaticGraphicsSwitching</key>
  <true/>
</dict>
</plist>
EOF

printf 'APPL????' > "$OUT_APP/Contents/PkgInfo"
echo "OK -> $OUT_APP"
