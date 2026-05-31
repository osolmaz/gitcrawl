//go:build windows

package cli

import (
	"fmt"
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var commandJobs sync.Map

func configureCommandGroup(cmd *exec.Cmd) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return
	}
	commandJobs.Store(cmd, job)
}

func attachCommandGroup(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	value, ok := commandJobs.Load(cmd)
	if !ok {
		return nil
	}
	job, ok := value.(windows.Handle)
	if !ok || job == 0 {
		return nil
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		cleanupCommandGroup(cmd)
		return nil
	}
	defer windows.CloseHandle(process)
	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		cleanupCommandGroup(cmd)
		return nil
	}
	return nil
}

func killCommandGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	if value, ok := commandJobs.Load(cmd); ok {
		if job, ok := value.(windows.Handle); ok && job != 0 {
			_ = windows.TerminateJobObject(job, 1)
		}
	}
	_ = exec.Command("taskkill", "/T", "/F", "/PID", fmt.Sprint(cmd.Process.Pid)).Run()
	_ = cmd.Process.Kill()
}

func cleanupCommandGroup(cmd *exec.Cmd) {
	value, ok := commandJobs.LoadAndDelete(cmd)
	if !ok {
		return
	}
	if job, ok := value.(windows.Handle); ok && job != 0 {
		_ = windows.CloseHandle(job)
	}
}
