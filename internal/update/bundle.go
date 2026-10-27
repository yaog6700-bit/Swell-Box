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
// (sing-box and shoes next to Swell-Box.exe) when data-dir cores are missing.
//
// After a successful seed (or if data-dir already has a core), the zip-side
// seed files are removed — runtime only uses ~/.swellbox/bin.
//
// Layout supported next to Swell-Box.exe:
//
//	./sing-box.exe
//	./shoes.exe
//	./libcronet.dll
//	./bin/sing-box.exe
//	./bin/shoes.exe
func InstallBundledCore() (bool, error) {
	installedAny := false
	for _, engine := range []string{paths.EngineSingBox, paths.EngineShoes} {
		if ok, err := installBundledEngine(engine); err == nil && ok {
			installedAny = true
		}
	}
	return installedAny, nil
}

func installBundledEngine(engine string) (bool, error) {
	name := paths.CoreBinaryNameFor(engine)
	binDir, err := paths.BinDir()
	if err != nil {
		return false, err
	}
	dest := filepath.Join(binDir, name)
	if st, err := os.Stat(dest); err == nil && !st.IsDir() && st.Size() > 0 {
		removeBundledSeedFor(engine)
		return false, nil
	}

	src, err := findBundledBinary(name)
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return false, err
	}
	if err := copyFile(src, dest); err != nil {
		return false, err
	}
	srcDir := filepath.Dir(src)
	_ = copySidecars(srcDir, binDir)

	if st, err := os.Stat(dest); err == nil && !st.IsDir() && st.Size() > 1024 {
		removeSeedAtFor(src, engine)
	}
	return true, nil
}

// removeBundledSeed deletes the zip-side cores (+ sidecars) when present.
func removeBundledSeed() {
	removeBundledSeedFor(paths.EngineSingBox)
	removeBundledSeedFor(paths.EngineShoes)
}

func removeBundledSeedFor(engine string) {
	name := paths.CoreBinaryNameFor(engine)
	src, err := findBundledBinary(name)
	if err != nil {
		return
	}
	removeSeedAtFor(src, engine)
}

func removeSeedAt(src string) {
	removeSeedAtFor(src, paths.EngineSingBox)
}

func removeSeedAtFor(src string, engine string) {
	if src == "" {
		return
	}
	name := paths.CoreBinaryNameFor(engine)
	if binDir, err := paths.BinDir(); err == nil {
		dest := filepath.Join(binDir, name)
		if sameFilePath(src, dest) {
			return
		}
	}
	srcDir := filepath.Dir(src)
	_ = os.Remove(src)
	if engine == paths.EngineSingBox {
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
	return findBundledBinary(paths.CoreBinaryName())
}

func findBundledBinary(name string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	dir := filepath.Dir(exe)
	candidates := []string{
		filepath.Join(dir, name),
		filepath.Join(dir, "bin", name),
		filepath.Join(dir, "core", name),
		filepath.Join(dir, "..", "Resources", name),
		filepath.Join(dir, "..", "..", "..", name),
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
