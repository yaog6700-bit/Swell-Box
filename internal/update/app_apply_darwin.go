//go:build darwin

package update

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ApplyAppUpdate downloads a macOS release (usually .zip with Swell-Box.app),
// then schedules a shell updater to replace the running app after exit and relaunch.
func ApplyAppUpdate(downloadURL string, isZip bool, stopFn func() error) error {
	if downloadURL == "" {
		return fmt.Errorf("no download url")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if r, err2 := filepath.EvalSymlinks(exe); err2 == nil {
		exe = r
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "swellbox-upd-*")
	if err != nil {
		return err
	}

	dlPath := filepath.Join(tmpDir, "download.bin")
	if err := downloadFile(downloadURL, dlPath); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("download: %w", err)
	}

	// Prefer replacing the whole .app bundle when we are running from one.
	targetApp := macAppBundleRoot(exe)
	var newApp string
	var newBin string

	if isZip || looksLikeZipFile(dlPath) {
		appPath, binPath, err := extractMacClientFromZip(dlPath, tmpDir)
		if err != nil {
			_ = os.RemoveAll(tmpDir)
			return err
		}
		newApp = appPath
		newBin = binPath
	} else {
		// Thin bare binary asset
		newBin = filepath.Join(tmpDir, "Swell-Box.new")
		if err := copyFilePathShared(dlPath, newBin); err != nil {
			_ = os.RemoveAll(tmpDir)
			return err
		}
		_ = os.Chmod(newBin, 0o755)
	}

	if targetApp != "" && newApp == "" && newBin != "" {
		// Running from .app but download was bare binary — replace only MacOS/Swell-Box.
		// Leave structure intact.
	}
	if targetApp == "" && newApp != "" {
		// Not in a bundle (dev run); install/replace next to cwd or use extracted app path only for open.
		// Fall through to binary replace of current exe if possible.
	}

	if stopFn != nil {
		_ = stopFn()
	}

	pid := os.Getpid()
	scriptPath := filepath.Join(tmpDir, "swellbox-update.sh")

	var script string
	switch {
	case targetApp != "" && newApp != "":
		// Full .app → .app replace (normal release path).
		script = fmt.Sprintf(`#!/bin/bash
set -e
TARGET=%q
NEW=%q
PID=%d
TMPDIR=%q
while kill -0 "$PID" 2>/dev/null; do sleep 0.4; done
sleep 0.6
rm -rf "$TARGET"
mkdir -p "$(dirname "$TARGET")"
/bin/cp -R "$NEW" "$TARGET"
/usr/bin/xattr -cr "$TARGET" 2>/dev/null || true
/usr/bin/open "$TARGET"
rm -rf "$TMPDIR"
`, targetApp, newApp, pid, tmpDir)
	case targetApp != "" && newBin != "":
		// Replace executable inside existing bundle only.
		destBin := filepath.Join(targetApp, "Contents", "MacOS", "Swell-Box")
		script = fmt.Sprintf(`#!/bin/bash
set -e
DEST=%q
NEW=%q
PID=%d
TMPDIR=%q
APP=%q
while kill -0 "$PID" 2>/dev/null; do sleep 0.4; done
sleep 0.6
/bin/cp -f "$NEW" "$DEST"
/bin/chmod +x "$DEST"
/usr/bin/xattr -cr "$APP" 2>/dev/null || true
/usr/bin/open "$APP"
rm -rf "$TMPDIR"
`, destBin, newBin, pid, tmpDir, targetApp)
	case newBin != "":
		// Bare binary replace (dev / non-bundle layout).
		script = fmt.Sprintf(`#!/bin/bash
set -e
TARGET=%q
NEW=%q
PID=%d
TMPDIR=%q
while kill -0 "$PID" 2>/dev/null; do sleep 0.4; done
sleep 0.6
/bin/cp -f "$NEW" "$TARGET"
/bin/chmod +x "$TARGET"
/usr/bin/xattr -cr "$TARGET" 2>/dev/null || true
"$TARGET" >/dev/null 2>&1 &
rm -rf "$TMPDIR"
`, exe, newBin, pid, tmpDir)
	case newApp != "":
		// Have a new .app but no install target — open it from temp is wrong.
		// Place beside current executable's parent.
		fallback := filepath.Join(filepath.Dir(exe), "Swell-Box.app")
		script = fmt.Sprintf(`#!/bin/bash
set -e
TARGET=%q
NEW=%q
PID=%d
TMPDIR=%q
while kill -0 "$PID" 2>/dev/null; do sleep 0.4; done
sleep 0.6
rm -rf "$TARGET"
/bin/cp -R "$NEW" "$TARGET"
/usr/bin/xattr -cr "$TARGET" 2>/dev/null || true
/usr/bin/open "$TARGET"
rm -rf "$TMPDIR"
`, fallback, newApp, pid, tmpDir)
	default:
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("update package has no Swell-Box.app or binary")
	}

	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		_ = os.RemoveAll(tmpDir)
		return err
	}
	cmd := exec.Command("/bin/bash", scriptPath)
	cmd.Dir = tmpDir
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("start updater: %w", err)
	}
	_ = cmd.Process.Release()
	return nil
}

// macAppBundleRoot returns the .app path if exe is .../Something.app/Contents/MacOS/<bin>.
func macAppBundleRoot(exe string) string {
	dir := filepath.Dir(exe) // MacOS
	if !strings.EqualFold(filepath.Base(dir), "MacOS") {
		return ""
	}
	contents := filepath.Dir(dir)
	if !strings.EqualFold(filepath.Base(contents), "Contents") {
		return ""
	}
	app := filepath.Dir(contents)
	if strings.HasSuffix(strings.ToLower(app), ".app") {
		return app
	}
	return ""
}

// extractMacClientFromZip unpacks zip and finds Swell-Box.app and/or Swell-Box binary.
func extractMacClientFromZip(zipPath, destDir string) (appPath, binPath string, err error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", "", fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	extractRoot := filepath.Join(destDir, "extract")
	if err := os.MkdirAll(extractRoot, 0o755); err != nil {
		return "", "", err
	}

	for _, f := range r.File {
		name := f.Name
		// Zip slip guard
		clean := filepath.Clean(name)
		if strings.HasPrefix(clean, "..") {
			continue
		}
		outPath := filepath.Join(extractRoot, clean)
		if f.FileInfo().IsDir() || strings.HasSuffix(name, "/") {
			_ = os.MkdirAll(outPath, 0o755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			return "", "", err
		}
		rc, err := f.Open()
		if err != nil {
			return "", "", err
		}
		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			_ = rc.Close()
			return "", "", err
		}
		_, copyErr := io.Copy(out, rc)
		_ = out.Close()
		_ = rc.Close()
		if copyErr != nil {
			return "", "", copyErr
		}
	}

	// Prefer deepest / first Swell-Box.app
	_ = filepath.Walk(extractRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil {
			return nil
		}
		if info.IsDir() && strings.EqualFold(info.Name(), "Swell-Box.app") {
			if appPath == "" {
				appPath = path
			}
			return filepath.SkipDir
		}
		return nil
	})
	if appPath != "" {
		// Ensure main binary is executable
		mainBin := filepath.Join(appPath, "Contents", "MacOS", "Swell-Box")
		_ = os.Chmod(mainBin, 0o755)
		coreBin := filepath.Join(appPath, "Contents", "MacOS", "sing-box")
		_ = os.Chmod(coreBin, 0o755)
		return appPath, mainBin, nil
	}

	// Fallback: bare binary named Swell-Box
	_ = filepath.Walk(extractRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		base := info.Name()
		if base == "Swell-Box" || strings.EqualFold(base, "swell-box") {
			binPath = path
			return filepath.SkipAll
		}
		return nil
	})
	if binPath != "" {
		_ = os.Chmod(binPath, 0o755)
		return "", binPath, nil
	}
	return "", "", fmt.Errorf("zip has no Swell-Box.app or Swell-Box binary")
}

func looksLikeZipFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var hdr [4]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return false
	}
	return hdr[0] == 'P' && hdr[1] == 'K'
}

func copyFilePathShared(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
