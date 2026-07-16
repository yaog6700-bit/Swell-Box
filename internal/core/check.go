package core

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

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
