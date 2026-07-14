package update

import (
	"fmt"
	"runtime"
	"strings"
)

// AppCheckResult is the outcome of an app update check.
type AppCheckResult struct {
	Current     string
	Latest      string
	HasUpdate   bool
	DownloadURL string
	Message     string
}

// CheckApp checks GitHub releases for a newer SWELL Box build when AppReleaseRepo is set.
func CheckApp() *AppCheckResult {
	res := &AppCheckResult{Current: AppVersion}
	if AppReleaseRepo == "" {
		res.Message = "manual"
		return res
	}
	url := "https://api.github.com/repos/" + AppReleaseRepo + "/releases/latest"
	rel, err := fetchJSON(url)
	if err != nil {
		res.Message = err.Error()
		return res
	}
	res.Latest = strings.TrimPrefix(rel.TagName, "v")
	res.HasUpdate = VersionLess(res.Current, res.Latest)
	// Prefer full offline zip for this platform, then bare client binary.
	goos, goarch := runtime.GOOS, runtime.GOARCH
	platform := goos + "-" + goarch
	var fullURL, thinURL string
	for _, a := range rel.Assets {
		n := strings.ToLower(a.Name)
		if !strings.Contains(n, platform) {
			// Windows bare client may be named SWELL-Box.exe without arch tag
			if goos == "windows" && strings.HasSuffix(n, ".exe") && strings.Contains(n, "swell") && thinURL == "" {
				thinURL = a.BrowserDownloadURL
			}
			continue
		}
		if strings.Contains(n, "full") && strings.HasSuffix(n, ".zip") {
			fullURL = a.BrowserDownloadURL
		} else if strings.HasSuffix(n, ".zip") || strings.HasSuffix(n, ".tar.gz") || strings.HasSuffix(n, ".tgz") {
			if thinURL == "" {
				thinURL = a.BrowserDownloadURL
			}
		} else if goos == "windows" && strings.HasSuffix(n, ".exe") {
			if thinURL == "" {
				thinURL = a.BrowserDownloadURL
			}
		}
	}
	if fullURL != "" {
		res.DownloadURL = fullURL
	} else {
		res.DownloadURL = thinURL
	}
	return res
}

// VersionLess reports whether a < b for simple semver-like strings.
func VersionLess(a, b string) bool {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")
	// strip pre-release suffix for rough compare
	if i := strings.IndexAny(a, "-+"); i >= 0 {
		a = a[:i]
	}
	if i := strings.IndexAny(b, "-+"); i >= 0 {
		b = b[:i]
	}
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < 3; i++ {
		var ai, bi int
		if i < len(as) {
			fmt.Sscanf(as[i], "%d", &ai)
		}
		if i < len(bs) {
			fmt.Sscanf(bs[i], "%d", &bi)
		}
		if ai < bi {
			return true
		}
		if ai > bi {
			return false
		}
	}
	return false
}
