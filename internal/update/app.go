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
	DownloadURL string // best asset for this platform (prefer thin client .exe)
	IsZip       bool   // true if DownloadURL is a zip archive
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
	pickAppAsset(res, rel.Assets)
	return res
}

func pickAppAsset(res *AppCheckResult, assets []ghAsset) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	platform := goos + "-" + goarch

	var (
		exePlatform string // SWELL-Box-windows-amd64.exe
		exeGeneric  string // SWELL-Box.exe
		fullZip     string // *-full.zip with platform
		anyZip      string
	)

	for _, a := range assets {
		n := strings.ToLower(a.Name)
		url := a.BrowserDownloadURL
		hasPlat := strings.Contains(n, platform)

		switch {
		case hasPlat && strings.HasSuffix(n, ".exe") && strings.Contains(n, "swell"):
			if exePlatform == "" {
				exePlatform = url
			}
		case goos == "windows" && strings.HasSuffix(n, ".exe") && strings.Contains(n, "swell") && !strings.Contains(n, "sing-box"):
			// untagged or other arch naming — keep as weak fallback
			if exeGeneric == "" && !strings.Contains(n, "arm64") && goarch == "amd64" {
				exeGeneric = url
			}
			if exeGeneric == "" && strings.Contains(n, "arm64") && goarch == "arm64" {
				exeGeneric = url
			}
			if exeGeneric == "" && !strings.Contains(n, "amd64") && !strings.Contains(n, "arm64") {
				exeGeneric = url
			}
		case hasPlat && strings.Contains(n, "full") && strings.HasSuffix(n, ".zip"):
			if fullZip == "" {
				fullZip = url
			}
		case hasPlat && (strings.HasSuffix(n, ".zip") || strings.HasSuffix(n, ".tar.gz")):
			if anyZip == "" {
				anyZip = url
			}
		}
	}

	// Prefer thin .exe for in-app replace (smaller, no unzip).
	switch {
	case exePlatform != "":
		res.DownloadURL = exePlatform
		res.IsZip = false
	case exeGeneric != "":
		res.DownloadURL = exeGeneric
		res.IsZip = false
	case fullZip != "":
		res.DownloadURL = fullZip
		res.IsZip = true
	case anyZip != "":
		res.DownloadURL = anyZip
		res.IsZip = true
	}
}

// VersionLess reports whether a < b for simple semver-like strings.
func VersionLess(a, b string) bool {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")
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
