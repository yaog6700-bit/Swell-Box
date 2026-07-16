//go:build darwin

package app

import (
	"fmt"
	"os/exec"
	"strings"
)

// ConfirmYesNo shows a native macOS Yes/No dialog via osascript.
func ConfirmYesNo(title, body string) bool {
	if title == "" {
		title = "Swell-Box"
	}
	// Escape for AppleScript string literals.
	t := appleString(title)
	b := appleString(body)
	script := fmt.Sprintf(
		`try
	display dialog %s with title %s buttons {"取消", "好"} default button "好" cancel button "取消" with icon caution
	return "yes"
on error number -128
	return "no"
end try`,
		b, t,
	)
	out, err := exec.Command("osascript", "-e", script).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "yes"
}

// AlertInfo shows a native macOS OK dialog via osascript.
func AlertInfo(title, body string) {
	if title == "" {
		title = "Swell-Box"
	}
	t := appleString(title)
	b := appleString(body)
	script := fmt.Sprintf(
		`display dialog %s with title %s buttons {"好"} default button "好" with icon note`,
		b, t,
	)
	_ = exec.Command("osascript", "-e", script).Run()
}

func appleString(s string) string {
	// AppleScript quoted string: "..." with \ and " escaped.
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
