package paths

import (
	"os"
	"path/filepath"
	"runtime"
)

const (
	// AppName is the user-visible product name (tray, notifications, about).
	AppName = "Swell-Box"
	// AppID is the stable id for data dirs (must stay "swellbox" for existing users).
	AppID = "swellbox"
	// Brand short mark.
	Brand = "Swell"
	// BinaryBase is the release/client binary base name without extension.
	BinaryBase  = "Swell-Box"
	DefaultPort = 9091
)

// HomeDir returns the Swell-Box user data directory:
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

const (
	EngineSingBox = "sing-box"
	EngineShoes   = "shoes"
)

// CoreBinaryName is the expected sing-box executable name on this OS.
func CoreBinaryName() string {
	return CoreBinaryNameFor(EngineSingBox)
}

// CoreBinaryNameFor returns the binary name for the specified engine.
func CoreBinaryNameFor(engine string) string {
	base := "sing-box"
	if engine == EngineShoes {
		base = "shoes"
	}
	if runtime.GOOS == "windows" {
		return base + ".exe"
	}
	return base
}

// DashboardURL is the official sing-box dashboard endpoint.
func DashboardURL(port int) string {
	if port <= 0 {
		port = DefaultPort
	}
	return "http://127.0.0.1:" + itoa(port) + "/dashboard/"
}

// ZashboardURL is the web Zashboard endpoint configured for shoes Clash API.
func ZashboardURL(port int, secret string) string {
	if port <= 0 {
		port = 9090
	}
	if secret == "" {
		secret = "swellbox"
	}
	return "https://board.zash.run/#/setup?hostname=127.0.0.1&port=" + itoa(port) + "&secret=" + secret
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
