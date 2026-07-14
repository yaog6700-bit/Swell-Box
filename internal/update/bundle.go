package update

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/swell-app/swellbox/internal/paths"
)

// InstallBundledCore copies sing-box (and sidecars like libcronet.dll) from
// next to the SWELL Box executable into ~/.swellbox/bin when the data-dir
// core is missing.
//
// Layout supported next to SWELL-Box.exe:
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
		return false, nil // already installed
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
	_ = copySidecars(filepath.Dir(src), binDir)
	return true, nil
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
	//   ./sing-box[.exe] next to SWELL-Box
	//   ./bin/sing-box , ./core/sing-box
	//   macOS .app: Contents/MacOS/SWELL-Box → also Contents/Resources, and folder next to .app
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
