package core

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/swell-app/swellbox/internal/paths"
)

// CheckConfigForEngine validates the configuration according to the specified core engine.
func CheckConfigForEngine(engine, bin, configPath, workDir string) error {
	if engine == paths.EngineShoes {
		cmd := exec.Command(bin, "check", configPath)
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		if err := cmd.Run(); err != nil {
			msg := strings.TrimSpace(buf.String())
			// Auto-heal compatibility difference between shoes <= 0.3.4 (strategy) and >= 0.3.5 (type)
			if strings.Contains(msg, "unknown field `strategy`") || strings.Contains(msg, "unknown field `type`") {
				if raw, rerr := os.ReadFile(configPath); rerr == nil {
					var fixed []byte
					if strings.Contains(msg, "unknown field `strategy`") {
						fixed = bytes.Replace(raw, []byte("strategy: selector"), []byte("type: selector"), 1)
					} else {
						fixed = bytes.Replace(raw, []byte("type: selector"), []byte("strategy: selector"), 1)
					}
					if !bytes.Equal(raw, fixed) {
						if werr := os.WriteFile(configPath, fixed, 0o644); werr == nil {
							var retryBuf bytes.Buffer
							retryCmd := exec.Command(bin, "check", configPath)
							retryCmd.Stdout = &retryBuf
							retryCmd.Stderr = &retryBuf
							if rerr2 := retryCmd.Run(); rerr2 == nil {
								return nil
							}
							buf = retryBuf
						}
					}
				}
			}
			if msg == "" {
				msg = err.Error()
			}
			msg = stripANSI(msg)
			lines := strings.Split(msg, "\n")
			var filtered []string
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || line == "Starting dry run." {
					continue
				}
				filtered = append(filtered, line)
			}
			if len(filtered) > 0 {
				msg = strings.Join(filtered, "\n")
			}
			if len(msg) > 400 {
				msg = msg[:400] + "…"
			}
			return fmt.Errorf("%s", msg)
		}
		return nil
	}
	return CheckConfig(bin, configPath, workDir)
}

// CheckConfig runs `sing-box check -c config -D workdir`.
func CheckConfig(bin, configPath, workDir string) error {
	cmd := exec.Command(bin, "check", "-c", configPath, "-D", workDir)
	cmd.Dir = workDir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(buf.String())
		if msg == "" {
			msg = err.Error()
		}
		msg = stripANSI(msg)
		// Keep first line — usually the FATAL reason.
		if i := strings.IndexAny(msg, "\r\n"); i >= 0 {
			msg = strings.TrimSpace(msg[:i])
		}
		if len(msg) > 400 {
			msg = msg[:400] + "…"
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}

// stripANSI removes terminal color codes from sing-box stderr (e.g. "\x1b[31mFATAL\x1b[0m").
func stripANSI(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) {
				c := s[j]
				j++
				if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') {
					break
				}
			}
			i = j - 1
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
