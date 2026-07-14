//go:build linux

package app

import (
	"fmt"
	"os/exec"
	"strings"
)

// PickJSONFile opens a file chooser via zenity or kdialog.
func PickJSONFile(title string) (string, error) {
	if title == "" {
		title = "Select config"
	}

	if _, err := exec.LookPath("zenity"); err == nil {
		out, err := exec.Command(
			"zenity", "--file-selection",
			"--title="+title,
			"--file-filter=JSON | *.json",
			"--file-filter=All | *",
		).Output()
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
				return "", nil // cancel
			}
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}

	if _, err := exec.LookPath("kdialog"); err == nil {
		out, err := exec.Command(
			"kdialog", "--getopenfilename", ".", "*.json|JSON files",
			"--title", title,
		).Output()
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
				return "", nil
			}
			return "", err
		}
		return strings.TrimSpace(string(out)), nil
	}

	return "", fmt.Errorf("file dialog: install zenity or kdialog")
}
