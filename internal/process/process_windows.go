//go:build windows

package process

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

var jobs sync.Map

func prepare(cmd *exec.Cmd) (func(), error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	_, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info)))
	if err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP}
	jobs.Store(cmd, job)
	cmd.Cancel = func() error { return windows.TerminateJobObject(job, 1) }
	return func() { jobs.Delete(cmd); windows.CloseHandle(job) }, nil
}
func attach(cmd *exec.Cmd) error {
	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	j, _ := jobs.Load(cmd)
	if err = windows.AssignProcessToJobObject(j.(windows.Handle), handle); err != nil {
		return fmt.Errorf("attach process to cancellation group: %w", err)
	}
	return nil
}
