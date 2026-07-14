//go:build windows

package app

import (
	"os/exec"
	"strings"
	"syscall"
)

// PickJSONFile opens a Windows open-file dialog and returns the selected path.
func PickJSONFile(title string) (string, error) {
	if title == "" {
		title = "Select config"
	}
	// Escape single quotes for PowerShell single-quoted string
	t := strings.ReplaceAll(title, "'", "''")
	ps := `
Add-Type -AssemblyName System.Windows.Forms
$d = New-Object System.Windows.Forms.OpenFileDialog
$d.Title = '` + t + `'
$d.Filter = 'JSON (*.json)|*.json|All files (*.*)|*.*'
$d.FilterIndex = 1
$d.Multiselect = $false
$d.CheckFileExists = $true
if ($d.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) {
  Write-Output $d.FileName
}
`
	cmd := exec.Command("powershell", "-NoProfile", "-STA", "-WindowStyle", "Hidden", "-Command", ps)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	path := strings.TrimSpace(string(out))
	return path, nil
}
