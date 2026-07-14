//go:build windows

package update

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// ApplyAppUpdate downloads the new client and schedules an in-place replace after exit.
// stopFn should stop the proxy / release locks; may be nil.
func ApplyAppUpdate(downloadURL string, isZip bool, stopFn func() error) error {
	if downloadURL == "" {
		return fmt.Errorf("no download url")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)

	tmpDir, err := os.MkdirTemp("", "swellbox-upd-*")
	if err != nil {
		return err
	}
	// Do not remove tmpDir here — updater script cleans it after replace.

	dlPath := filepath.Join(tmpDir, "download.bin")
	if err := downloadFile(downloadURL, dlPath); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("download: %w", err)
	}

	newExe := filepath.Join(tmpDir, "SWELL-Box.new.exe")
	if isZip || looksLikeZip(dlPath) {
		extracted, err := extractWindowsClientFromZip(dlPath, tmpDir)
		if err != nil {
			_ = os.RemoveAll(tmpDir)
			return err
		}
		if err := os.Rename(extracted, newExe); err != nil {
			// cross-device rename may fail; copy
			if err := copyFilePath(extracted, newExe); err != nil {
				_ = os.RemoveAll(tmpDir)
				return err
			}
		}
	} else {
		if err := os.Rename(dlPath, newExe); err != nil {
			if err := copyFilePath(dlPath, newExe); err != nil {
				_ = os.RemoveAll(tmpDir)
				return err
			}
		}
	}

	if stopFn != nil {
		_ = stopFn()
	}

	// Batch updater: wait for this process to exit, then replace and relaunch.
	pid := os.Getpid()
	bat := filepath.Join(dir, "swellbox-update.bat")
	// Use short delays; Windows can't overwrite a running exe.
	script := fmt.Sprintf(`@echo off
setlocal
set "TARGET=%s"
set "NEW=%s"
set "PID=%d"
set "TMPDIR=%s"
:wait
tasklist /FI "PID eq %%PID%%" 2>NUL | find "%%PID%%" >NUL
if not errorlevel 1 (
  timeout /t 1 /nobreak >NUL
  goto wait
)
copy /Y "%%NEW%%" "%%TARGET%%" >NUL
if errorlevel 1 (
  ping -n 2 127.0.0.1 >NUL
  copy /Y "%%NEW%%" "%%TARGET%%" >NUL
)
start "" "%%TARGET%%"
rd /s /q "%%TMPDIR%%" 2>NUL
del "%%~f0"
`, exe, newExe, pid, tmpDir)

	if err := os.WriteFile(bat, []byte(script), 0o644); err != nil {
		_ = os.RemoveAll(tmpDir)
		return err
	}

	cmd := exec.Command("cmd.exe", "/C", bat)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		_ = os.Remove(bat)
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("start updater: %w", err)
	}
	// Detach: parent will exit so file lock is released.
	_ = cmd.Process.Release()
	return nil
}

func looksLikeZip(path string) bool {
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

func extractWindowsClientFromZip(zipPath, destDir string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	var best *zip.File
	for _, f := range r.File {
		base := strings.ToLower(filepath.Base(f.Name))
		if base == "swell-box.exe" || base == "swellbox.exe" {
			best = f
			break
		}
		if strings.HasPrefix(base, "swell-box") && strings.HasSuffix(base, ".exe") && best == nil {
			best = f
		}
	}
	if best == nil {
		return "", fmt.Errorf("zip has no SWELL-Box.exe")
	}
	outPath := filepath.Join(destDir, "from-zip.exe")
	rc, err := best.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(out, rc)
	closeErr := out.Close()
	if copyErr != nil {
		return "", copyErr
	}
	return outPath, closeErr
}

func copyFilePath(src, dst string) error {
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
