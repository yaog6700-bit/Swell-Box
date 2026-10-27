package update

import (
	"runtime"
	"testing"
)

func TestPickShoesAsset(t *testing.T) {
	assets := []ghAsset{
		{Name: "shoes-aarch64-apple-darwin.tar.gz", BrowserDownloadURL: "https://example.com/darwin-arm"},
		{Name: "shoes-x86_64-apple-darwin.tar.gz", BrowserDownloadURL: "https://example.com/darwin-amd64"},
		{Name: "shoes-x86_64-unknown-linux-gnu.tar.gz", BrowserDownloadURL: "https://example.com/linux-amd64"},
		{Name: "shoes-x86_64-pc-windows-msvc.zip", BrowserDownloadURL: "https://example.com/win-amd64"},
	}

	name, url, err := pickShoesAsset(assets)
	if runtime.GOOS == "windows" && runtime.GOARCH == "amd64" {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if name != "shoes-x86_64-pc-windows-msvc.zip" {
			t.Errorf("expected shoes-x86_64-pc-windows-msvc.zip, got %s", name)
		}
		if url != "https://example.com/win-amd64" {
			t.Errorf("expected win url, got %s", url)
		}
	}
}
