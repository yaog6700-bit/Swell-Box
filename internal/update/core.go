package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/swell-app/swellbox/internal/paths"
)

type ghRelease struct {
	TagName    string    `json:"tag_name"`
	Prerelease bool      `json:"prerelease"`
	Draft      bool      `json:"draft"`
	Assets     []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// Core channel: stable (GitHub "latest") or pre (newest including alpha/beta).
const (
	ChannelStable = "stable"
	ChannelPre    = "pre"
)

// CoreInfo is installed / remote core version info.
type CoreInfo struct {
	Installed string
	Latest    string
	AssetURL  string
	AssetName string
	Channel   string
	Prerelease bool
}

// CheckCore queries GitHub for sing-box matching this OS/arch.
// channel: ChannelStable or ChannelPre.
func CheckCore(channel string) (*CoreInfo, error) {
	if channel != ChannelPre {
		channel = ChannelStable
	}
	info := &CoreInfo{Installed: installedCoreVersion(), Channel: channel}

	var rel *ghRelease
	var err error
	if channel == ChannelPre {
		rel, err = fetchNewestRelease(true)
	} else {
		rel, err = fetchJSON("https://api.github.com/repos/SagerNet/sing-box/releases/latest")
	}
	if err != nil {
		return info, err
	}
	info.Latest = strings.TrimPrefix(rel.TagName, "v")
	info.Prerelease = rel.Prerelease
	name, url, err := pickAsset(rel.Assets)
	if err != nil {
		return info, err
	}
	info.AssetName = name
	info.AssetURL = url
	return info, nil
}

// fetchNewestRelease returns the newest non-draft release.
// If allowPre is false, skips prereleases (same idea as /latest).
// If allowPre is true, takes the first list entry that has a platform asset (alpha/rc ok).
func fetchNewestRelease(allowPre bool) (*ghRelease, error) {
	list, err := fetchJSONList("https://api.github.com/repos/SagerNet/sing-box/releases?per_page=20")
	if err != nil {
		return nil, err
	}
	for i := range list {
		r := &list[i]
		if r.Draft {
			continue
		}
		if !allowPre && r.Prerelease {
			continue
		}
		if _, _, err := pickAsset(r.Assets); err != nil {
			continue
		}
		return r, nil
	}
	return nil, fmt.Errorf("no suitable sing-box release found")
}

func installedCoreVersion() string {
	bin, err := resolveCoreBin()
	if err != nil {
		return ""
	}
	// run sing-box version
	// avoid import cycle with core — exec here
	out, err := runCmd(bin, "version")
	if err != nil {
		return ""
	}
	// first line often: sing-box version x.y.z
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "version") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				return fields[len(fields)-1]
			}
		}
	}
	return strings.TrimSpace(out)
}

func resolveCoreBin() (string, error) {
	name := paths.CoreBinaryName()
	if binDir, err := paths.BinDir(); err == nil {
		p := filepath.Join(binDir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("core binary not found")
}

// UpdateCore downloads sing-box for the given channel and installs into ~/.swellbox/bin.
// stopFn should stop the running core before replace; may be nil.
func UpdateCore(channel string, stopFn func() error) (string, error) {
	info, err := CheckCore(channel)
	if err != nil {
		return "", err
	}
	if info.AssetURL == "" {
		return "", fmt.Errorf("no asset for this platform")
	}
	if stopFn != nil {
		_ = stopFn()
		time.Sleep(500 * time.Millisecond)
	}

	binDir, err := paths.BinDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", err
	}

	tmpDir, err := os.MkdirTemp("", "swell-core-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, info.AssetName)
	if err := downloadFile(info.AssetURL, archivePath); err != nil {
		return "", err
	}

	extracted, err := extractCoreBinary(archivePath, tmpDir)
	if err != nil {
		return "", err
	}

	dest := filepath.Join(binDir, paths.CoreBinaryName())
	// Windows: may need to remove old file if locked
	_ = os.Remove(dest)
	data, err := os.ReadFile(extracted)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dest, data, 0o755); err != nil {
		return "", fmt.Errorf("install core: %w", err)
	}
	// copy dlls next to binary if present (libcronet)
	_ = copySidecars(filepath.Dir(extracted), binDir)

	return info.Latest, nil
}

func pickAsset(assets []ghAsset) (name, url string, err error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH
	// Prefer modern builds, e.g. sing-box-1.14.0-alpha.43-windows-amd64.zip
	// Avoid legacy-windows-7 and android packages.
	platform := fmt.Sprintf("%s-%s", goos, goarch)

	var fallbackName, fallbackURL string
	for _, a := range assets {
		n := strings.ToLower(a.Name)
		if !strings.Contains(n, platform) {
			continue
		}
		if !(strings.HasSuffix(n, ".zip") || strings.HasSuffix(n, ".tar.gz") || strings.HasSuffix(n, ".tgz")) {
			continue
		}
		if strings.Contains(n, "android") || strings.Contains(n, "legacy") {
			continue
		}
		// exact-ish: ends with windows-amd64.zip
		if strings.Contains(n, platform+".zip") || strings.Contains(n, platform+".tar.gz") || strings.Contains(n, platform+".tgz") {
			return a.Name, a.BrowserDownloadURL, nil
		}
		if fallbackName == "" {
			fallbackName, fallbackURL = a.Name, a.BrowserDownloadURL
		}
	}
	if fallbackName != "" {
		return fallbackName, fallbackURL, nil
	}
	return "", "", fmt.Errorf("no release asset for %s/%s", goos, goarch)
}

func extractCoreBinary(archive, destDir string) (string, error) {
	lower := strings.ToLower(archive)
	wantName := paths.CoreBinaryName()
	if strings.HasSuffix(lower, ".zip") {
		return extractZipBin(archive, destDir, wantName)
	}
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return extractTarGzBin(archive, destDir, wantName)
	}
	return "", fmt.Errorf("unknown archive format")
}

func extractZipBin(archive, destDir, wantName string) (string, error) {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return "", err
	}
	defer r.Close()
	var found string
	for _, f := range r.File {
		base := filepath.Base(f.Name)
		// extract dlls and binary
		if base != wantName && !strings.HasSuffix(strings.ToLower(base), ".dll") {
			continue
		}
		outPath := filepath.Join(destDir, base)
		if err := writeZipFile(f, outPath); err != nil {
			return "", err
		}
		if base == wantName {
			found = outPath
		}
	}
	if found == "" {
		return "", fmt.Errorf("%s not found in zip", wantName)
	}
	return found, nil
}

func writeZipFile(f *zip.File, outPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

func extractTarGzBin(archive, destDir, wantName string) (string, error) {
	f, err := os.Open(archive)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var found string
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := filepath.Base(hdr.Name)
		if base != wantName {
			continue
		}
		outPath := filepath.Join(destDir, base)
		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, tr); err != nil {
			out.Close()
			return "", err
		}
		out.Close()
		found = outPath
	}
	if found == "" {
		return "", fmt.Errorf("%s not found in tar.gz", wantName)
	}
	return found, nil
}

func copySidecars(srcDir, dstDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".dll") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(srcDir, name))
		if err != nil {
			continue
		}
		_ = os.WriteFile(filepath.Join(dstDir, name), data, 0o644)
	}
	return nil
}

func fetchJSON(url string) (*ghRelease, error) {
	body, err := httpGet(url)
	if err != nil {
		return nil, err
	}
	var rel ghRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func fetchJSONList(url string) ([]ghRelease, error) {
	body, err := httpGet(url)
	if err != nil {
		return nil, err
	}
	var list []ghRelease
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func httpGet(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SWELLBox/"+AppVersion)
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("github api http %d: %s", resp.StatusCode, msg)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("github api empty body")
	}
	return body, nil
}

func downloadFile(url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "SWELLBox/"+AppVersion)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download http %d", resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}
