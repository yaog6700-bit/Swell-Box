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
		if len(msg) > 400 {
			msg = msg[:400] + "…"
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}
