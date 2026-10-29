package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
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

// Repositories for core engines.
const (
	SingBoxRepo = "SagerNet/sing-box"
	ShoesRepo   = "yaog6700-bit/shoes"
)

// CheckCore queries GitHub for sing-box matching this OS/arch.
// channel: ChannelStable or ChannelPre.
func CheckCore(channel string) (*CoreInfo, error) {
	return CheckCoreForEngine(paths.EngineSingBox, channel)
}

// CheckCoreForEngine queries GitHub for the specified engine matching this OS/arch.
func CheckCoreForEngine(engine, channel string) (*CoreInfo, error) {
	if channel != ChannelPre {
		channel = ChannelStable
	}
	info := &CoreInfo{Installed: InstalledCoreVersionFor(engine), Channel: channel}

	repo := SingBoxRepo
	picker := pickAsset
	if engine == paths.EngineShoes {
		repo = ShoesRepo
		picker = pickShoesAsset
	}

	var rel *ghRelease
	var err error
	if channel == ChannelPre {
		rel, err = fetchNewestReleaseForRepo(repo, true, picker)
	} else {
		rel, err = fetchJSON(fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo))
	}
	if err != nil {
		return info, err
	}
	info.Latest = strings.TrimPrefix(rel.TagName, "v")
	info.Prerelease = rel.Prerelease
	name, url, err := picker(rel.Assets)
	if err != nil {
		return info, err
	}
	info.AssetName = name
	info.AssetURL = url
	return info, nil
}

// fetchNewestRelease returns the newest non-draft release for sing-box.
func fetchNewestRelease(allowPre bool) (*ghRelease, error) {
	return fetchNewestReleaseForRepo(SingBoxRepo, allowPre, pickAsset)
}

// fetchNewestReleaseForRepo returns the newest non-draft release for repo.
// If allowPre is false, skips prereleases (same idea as /latest).
// If allowPre is true, takes the first list entry that has a platform asset (alpha/rc ok).
func fetchNewestReleaseForRepo(repo string, allowPre bool, picker func([]ghAsset) (string, string, error)) (*ghRelease, error) {
	list, err := fetchJSONList(fmt.Sprintf("https://api.github.com/repos/%s/releases?per_page=20", repo))
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
		if _, _, err := picker(r.Assets); err != nil {
			continue
		}
		return r, nil
	}
	return nil, fmt.Errorf("no suitable %s release found", repo)
}

// InstalledCoreVersion runs `sing-box version` on the binary that Start() uses
// (~/.swellbox/bin first). Empty if no core is found.
func InstalledCoreVersion() string {
	return InstalledCoreVersionFor(paths.EngineSingBox)
}

// InstalledCoreVersionFor returns the installed version of the specified engine.
func InstalledCoreVersionFor(engine string) string {
	if engine == paths.EngineShoes {
		bin, err := resolveShoesBin()
		if err != nil {
			return ""
		}
		out, err := runCmd(bin, "version")
		if err != nil {
			return ""
		}
		fields := strings.Fields(strings.TrimSpace(out))
		if len(fields) > 0 {
			return fields[len(fields)-1]
		}
		return strings.TrimSpace(out)
	}
	return installedCoreVersion()
}

func resolveShoesBin() (string, error) {
	name := paths.CoreBinaryNameFor(paths.EngineShoes)
	if binDir, err := paths.BinDir(); err == nil {
		p := filepath.Join(binDir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Size() > 0 {
			return p, nil
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		p := filepath.Join(home, "Desktop", name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Size() > 0 {
			return p, nil
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("shoes binary not found")
}

// ResolvedCorePath returns the absolute path of the sing-box core binary.
func ResolvedCorePath() (string, error) {
	return ResolvedCorePathFor(paths.EngineSingBox)
}

// ResolvedCorePathFor returns the absolute path of the specified core engine binary.
func ResolvedCorePathFor(engine string) (string, error) {
	if engine == paths.EngineShoes {
		return resolveShoesBin()
	}
	return resolveCoreBin()
}

func installedCoreVersion() string {
	bin, err := resolveCoreBin()
	if err != nil {
		return ""
	}
	// run sing-box version — avoid import cycle with core (exec here)
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

// resolveCoreBin matches core.Manager.ResolveBinary without CorePath:
// data-dir only (zip-side binary is seed, not a second runtime).
func resolveCoreBin() (string, error) {
	name := paths.CoreBinaryName()
	if binDir, err := paths.BinDir(); err == nil {
		p := filepath.Join(binDir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Size() > 0 {
			return p, nil
		}
	}
	return "", fmt.Errorf("core binary not found in data dir")
}

// UpdateCore downloads sing-box for the given channel and installs into ~/.swellbox/bin.
// Download/extract runs first so TUN/system proxy can still help reach GitHub; stopFn is
// only called right before replacing the on-disk binary (which may be locked while running).
// stopFn may be nil. Temp archive under os.TempDir is always removed via defer.
func UpdateCore(channel string, stopFn func() error) (string, error) {
	return UpdateCoreForEngine(paths.EngineSingBox, channel, stopFn)
}

// UpdateCoreForEngine downloads the specified engine for the given channel and installs into ~/.swellbox/bin.
func UpdateCoreForEngine(engine, channel string, stopFn func() error) (string, error) {
	info, err := CheckCoreForEngine(engine, channel)
	if err != nil {
		return "", err
	}
	if info.AssetURL == "" {
		return "", fmt.Errorf("no asset for this platform")
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

	// 1) Download + extract while the old core may still be proxying traffic.
	archivePath := filepath.Join(tmpDir, info.AssetName)
	if err := downloadFile(info.AssetURL, archivePath); err != nil {
		return "", err
	}
	extracted, err := extractCoreBinaryForEngine(archivePath, tmpDir, engine)
	if err != nil {
		return "", err
	}

	// 2) Stop running core only when we are ready to replace the file.
	if stopFn != nil {
		_ = stopFn()
		time.Sleep(500 * time.Millisecond)
	}

	// 3) Install into data dir; Windows may need remove first if the file was locked.
	dest := filepath.Join(binDir, paths.CoreBinaryNameFor(engine))
	_ = os.Remove(dest)
	data, err := os.ReadFile(extracted)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dest, data, 0o755); err != nil {
		return "", fmt.Errorf("install core: %w", err)
	}
	_ = os.Chmod(dest, 0o755)
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

func pickShoesAsset(assets []ghAsset) (name, url string, err error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	var targetMatch string
	switch {
	case goos == "windows" && goarch == "amd64":
		targetMatch = "x86_64-pc-windows"
	case goos == "windows" && goarch == "arm64":
		targetMatch = "aarch64-pc-windows"
	case goos == "linux" && goarch == "amd64":
		targetMatch = "x86_64-unknown-linux"
	case goos == "linux" && goarch == "arm64":
		targetMatch = "aarch64-unknown-linux"
	case goos == "darwin" && goarch == "amd64":
		targetMatch = "x86_64-apple-darwin"
	case goos == "darwin" && goarch == "arm64":
		targetMatch = "aarch64-apple-darwin"
	default:
		return "", "", fmt.Errorf("no shoes build for %s/%s", goos, goarch)
	}

	for _, a := range assets {
		n := strings.ToLower(a.Name)
		if strings.Contains(n, targetMatch) && (strings.HasSuffix(n, ".zip") || strings.HasSuffix(n, ".tar.gz") || strings.HasSuffix(n, ".tgz")) {
			return a.Name, a.BrowserDownloadURL, nil
		}
	}
	return "", "", fmt.Errorf("no shoes release asset found for %s (%s/%s)", targetMatch, goos, goarch)
}

func extractCoreBinary(archive, destDir string) (string, error) {
	return extractCoreBinaryForEngine(archive, destDir, paths.EngineSingBox)
}

func extractCoreBinaryForEngine(archive, destDir, engine string) (string, error) {
	lower := strings.ToLower(archive)
	wantName := paths.CoreBinaryNameFor(engine)
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
	client := newHTTPClient(45 * time.Second)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Swell-Box/"+AppVersion)
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

// newHTTPClient prefers the local mixed proxy (127.0.0.1:7890) when the core
// is running, then HTTP(S)_PROXY env. overall is the total request deadline.
func newHTTPClient(overall time.Duration) *http.Client {
	return &http.Client{
		Timeout: overall,
		Transport: &http.Transport{
			Proxy: downloadProxy,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   30 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
			IdleConnTimeout:       90 * time.Second,
			ForceAttemptHTTP2:     true,
		},
	}
}

// downloadFile fetches a large release asset. GitHub assets are ~20MB+; on slow
// or interrupted links a short Client.Timeout produces:
//
//	context deadline exceeded (Client.Timeout or context cancellation while reading body)
//
// Strategy: long total budget, retries, then public GitHub mirrors as fallback.
func downloadFile(url, dest string) error {
	candidates := downloadURLCandidates(url)
	var lastErr error
	for _, u := range candidates {
		for attempt := 1; attempt <= 2; attempt++ {
			if err := downloadFileOnce(u, dest); err != nil {
				lastErr = err
				_ = os.Remove(dest)
				if attempt < 2 {
					time.Sleep(time.Duration(attempt) * 2 * time.Second)
				}
				continue
			}
			return nil
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("download failed")
	}
	return fmt.Errorf("%w (network slow/blocked? try proxy or manual install of sing-box)", lastErr)
}

func downloadURLCandidates(url string) []string {
	out := []string{url}
	// Only mirror official GitHub release/object URLs.
	if !strings.Contains(url, "github.com/") && !strings.Contains(url, "githubusercontent.com/") {
		return out
	}
	// Public reverse proxies commonly used when github.com is slow (e.g. CN).
	// Tried only after the direct URL fails.
	mirrors := []string{
		"https://ghfast.top/",
		"https://mirror.ghproxy.com/",
		"https://ghproxy.net/",
	}
	for _, m := range mirrors {
		out = append(out, m+url)
	}
	return out
}

func downloadFileOnce(url, dest string) error {
	// 10 minutes per URL/attempt; with local proxy this is plenty, and avoids
	// "stuck downloading forever" when system-proxy mode was still going direct.
	client := newHTTPClient(10 * time.Minute)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Swell-Box/"+AppVersion)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download http %d", resp.StatusCode)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	// Ensure partial files are discarded on failure.
	ok := false
	defer func() {
		out.Close()
		if !ok {
			_ = os.Remove(dest)
		}
	}()
	n, err := io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	if n < 1024 {
		return fmt.Errorf("download too small (%d bytes)", n)
	}
	if err := out.Close(); err != nil {
		return err
	}
	ok = true
	return nil
}
