//go:build windows

package core

import (
	"os"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows Job Object: when Swell-Box exits (or job handle closes), all assigned
// child processes (sing-box) are terminated — zero residual cores.
var (
	jobOnce   sync.Once
	jobHandle windows.Handle
	jobOK     bool
)

func ensureKillOnCloseJob() {
	jobOnce.Do(func() {
		h, err := windows.CreateJobObject(nil, nil)
		if err != nil || h == 0 {
			return
		}
		var info windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		_, err = windows.SetInformationJobObject(
			h,
			windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)),
			uint32(unsafe.Sizeof(info)),
		)
		if err != nil {
			_ = windows.CloseHandle(h)
			return
		}
		jobHandle = h
		jobOK = true
	})
}

// assignToJob puts the process into the kill-on-close job (best-effort).
func assignToJob(p *os.Process) {
	if p == nil {
		return
	}
	ensureKillOnCloseJob()
	if !jobOK || jobHandle == 0 {
		return
	}
	const processSetQuota = 0x0100
	const processTerminate = 0x0001
	const processSetInformation = 0x0200
	access := uint32(processSetQuota | processTerminate | processSetInformation)
	h, err := windows.OpenProcess(access, false, uint32(p.Pid))
	if err != nil || h == 0 {
		return
	}
	defer windows.CloseHandle(h)
	_ = windows.AssignProcessToJobObject(jobHandle, h)
}

// CloseJob releases the job handle (kills remaining children if any).
func CloseJob() {
	if jobHandle != 0 {
		_ = windows.CloseHandle(jobHandle)
		jobHandle = 0
		jobOK = false
	}
}
