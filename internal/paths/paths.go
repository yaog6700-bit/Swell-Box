package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

const (
	AppName     = "SWELL Box"
	AppID       = "swellbox"
	Brand       = "SWELL"
	DefaultPort = 9091
)

// HomeDir returns the SWELL Box user data directory:
//   Windows: %USERPROFILE%\.swellbox
//   others:  $HOME/.swellbox
func HomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "."+AppID), nil
}

func ConfigDir() (string, error) {
	return HomeDir()
}

func AppConfigPath() (string, error) {
	dir, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "app.json"), nil
}

func BinDir() (string, error) {
	dir, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "bin"), nil
}

func LogsDir() (string, error) {
	dir, err := HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "logs"), nil
}

// CoreBinaryName is the expected sing-box executable name on this OS.
func CoreBinaryName() string {
	if runtime.GOOS == "windows" {
		return "sing-box.exe"
	}
	return "sing-box"
}

// DashboardURL is the official sing-box dashboard endpoint.
func DashboardURL(port int) string {
	if port <= 0 {
		port = DefaultPort
	}
	return "http://127.0.0.1:" + itoa(port) + "/dashboard/"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
