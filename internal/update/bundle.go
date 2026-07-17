package update

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/swell-app/swellbox/internal/paths"
)

// InstallBundledCore seeds ~/.swellbox/bin from the offline full.zip layout
// (sing-box next to Swell-Box.exe) when the data-dir core is missing.
//
// After a successful seed (or if data-dir already has a core), the zip-side
// seed files are removed — runtime only uses ~/.swellbox/bin.
//
// Layout supported next to Swell-Box.exe:
//
//	./sing-box.exe
//	./libcronet.dll
//	./bin/sing-box.exe
func InstallBundledCore() (bool, error) {
	name := paths.CoreBinaryName()
	binDir, err := paths.BinDir()
	if err != nil {
		return false, err
	}
	dest := filepath.Join(binDir, name)
	if st, err := os.Stat(dest); err == nil && !st.IsDir() && st.Size() > 0 {
		// Data dir already ready — drop leftover seed next to the app.
		removeBundledSeed()
		return false, nil
	}

	src, err := findBundledCore()
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return false, err
	}
	if err := copyFile(src, dest); err != nil {
		return false, err
	}
	// copy dlls from same directory as source
	srcDir := filepath.Dir(src)
	_ = copySidecars(srcDir, binDir)

	// Verify dest then remove seed so users do not keep two kernels.
	if st, err := os.Stat(dest); err == nil && !st.IsDir() && st.Size() > 1024 {
		removeSeedAt(src)
	}
	return true, nil
}

// removeBundledSeed deletes the zip-side sing-box (+ sidecars) when present.
func removeBundledSeed() {
	src, err := findBundledCore()
	if err != nil {
		return
	}
	removeSeedAt(src)
}

func removeSeedAt(src string) {
	if src == "" {
		return
	}
	// Never delete the data-dir binary.
	if binDir, err := paths.BinDir(); err == nil {
		dest := filepath.Join(binDir, paths.CoreBinaryName())
		if sameFilePath(src, dest) {
			return
		}
	}
	srcDir := filepath.Dir(src)
	_ = os.Remove(src)
	// Sidecar DLLs that ship with windows full packages.
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := strings.ToLower(e.Name())
		if strings.HasSuffix(n, ".dll") && (strings.Contains(n, "cronet") || strings.Contains(n, "sing")) {
			_ = os.Remove(filepath.Join(srcDir, e.Name()))
		}
	}
}

func sameFilePath(a, b string) bool {
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return strings.EqualFold(aa, bb)
}

func findBundledCore() (string, error) {
	name := paths.CoreBinaryName()
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	dir := filepath.Dir(exe)
	// Layouts:
	//   ./sing-box[.exe] next to Swell-Box
	//   ./bin/sing-box , ./core/sing-box
	//   macOS .app: Contents/MacOS/Swell-Box → also Contents/Resources, and folder next to .app
	candidates := []string{
		filepath.Join(dir, name),
		filepath.Join(dir, "bin", name),
		filepath.Join(dir, "core", name),
		filepath.Join(dir, "..", "Resources", name),
		filepath.Join(dir, "..", "..", "..", name), // next to Foo.app
	}
	for _, p := range candidates {
		p = filepath.Clean(p)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Size() > 1024 {
			return p, nil
		}
	}
	return "", fmt.Errorf("bundled %s not found next to app", name)
}

func copyFile(src, dst string) error {
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
