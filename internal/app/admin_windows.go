//go:build windows

package app

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// IsElevated reports whether the current process runs with admin rights.
func IsElevated() bool {
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token)
	if err != nil {
		return false
	}
	defer token.Close()

	type tokenElevation struct {
		TokenIsElevated uint32
	}
	var elev tokenElevation
	var outLen uint32
	err = windows.GetTokenInformation(
		token,
		windows.TokenElevation,
		(*byte)(unsafe.Pointer(&elev)),
		uint32(unsafe.Sizeof(elev)),
		&outLen,
	)
	if err != nil {
		return false
	}
	return elev.TokenIsElevated != 0
}
