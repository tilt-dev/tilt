// +build windows

package graceful

import (
	"context"
	"errors"
	"os"
	"syscall"
)

var (
	// ErrDifferentArch is returned when the target process has a different
	// architecture (x86 or x64) than the calling process. Both processes must
	// be of the same architecture for an exit to be performed.
	ErrDifferentArch = errors.New("target process architecture differs from the caller")
)

// These are win32 process security and access rights necessary to perform
// a call to CreateRemoteThread().
//
// Reference: https://msdn.microsoft.com/library/ms682437
const (
	ProcessTerminate        = 0x0001 // PROCESS_TERMINATE
	ProcessCreateThread     = 0x0002 // PROCESS_CREATE_THREAD
	ProcessVMOperation      = 0x0008 // PROCESS_VM_OPERATION
	ProcessVMRead           = 0x0010 // PROCESS_VM_READ
	ProcessVMWrite          = 0x0020 // PROCESS_VM_WRITE
	ProcessQueryInformation = 0x0400 // PROCESS_QUERY_INFORMATION

	ProcessCreateRemoteThread = ProcessCreateThread | ProcessQueryInformation | ProcessVMOperation | ProcessVMWrite | ProcessVMRead | ProcessTerminate
)

// ExitOrTerminate attempts to use Exit to end a target process. If Exit fails
// or the context is cancelled, Terminate will be called.
//
// FIXME: The termination portion of this function is not implemented yet.
func ExitOrTerminate(ctx context.Context, pid, code int) (err error) {
	process, err := openProcess(ProcessCreateRemoteThread, false, uint32(pid))
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(process)
	err = exit(ctx, process, uint32(code))
	if err == nil {
		return nil
	}
	return terminate(process, uint32(code))
}

// Exit attempts to force an exit within a target process. The process is
// identified by its process id. If successful, the process will exit with the
// given exit code.
//
// Exit is more rude than sending a real SIGINT signal.
// Exit is less rude than Terminate.
//
// Exit works by effectively calling os.Exit(code) within the target
// process, with all the caveats that would entail. It calls the ExitProcess()
// function in the win32 API.
//
// If the calling process architecture differs from that of the target process,
// ErrDifferentArch will be returned.
//
// The returned error will be nil if the process exited. If it is non-nil, the
// process may still be running. If the context has been cancelled the context's
// error may be returned.
//
// TODO: Check to make sure the process is actually running first.
func Exit(ctx context.Context, pid, code int) (err error) {
	process, err := openProcess(ProcessCreateRemoteThread, false, uint32(pid))
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(process)
	return exit(ctx, process, uint32(code))
}

func exit(ctx context.Context, process syscall.Handle, exitcode uint32) (err error) {
	var (
		callerProcess, remoteProcess syscall.Handle
	)

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	callerProcess, err = syscall.GetCurrentProcess()
	if err != nil {
		return err
	}

	// Ensure we have a process handle with the necessary privileges
	err = syscall.DuplicateHandle(callerProcess, process, callerProcess, &remoteProcess, ProcessCreateRemoteThread, false, 0)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(remoteProcess)

	if !sameArch(callerProcess, remoteProcess) {
		return ErrDifferentArch
	}

	return execRemoteExit(ctx, remoteProcess, exitcode)
}

func execRemoteExit(ctx context.Context, remoteProcess syscall.Handle, exitcode uint32) (err error) {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Initiate the call into the remote process
	thread, _, err := createRemoteThread(remoteProcess, nil, 0, procExitProcess.Addr(), uintptr(exitcode), 0)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(thread)

	// Block until the remote thread exits or the context is cancelled
	event, cancel, done, err := eventFromContext(ctx)
	if err != nil {
		return err
	}
	defer cancel()
	defer syscall.CloseHandle(event)

	sema := []syscall.Handle{
		thread,
		event,
	}

	result, err := waitForMultipleObjects(sema, false, infinite)
	if err != nil {
		return err
	}

	switch result {
	case waitObject:
		// The thread completed
	case waitObject + 1:
		// The context was cancelled
		err = ctx.Err()
	}

	cancel()
	<-done

	return
}

// Terminate forcefully ends the target process. The process is identified by
// its process id. The process will be given no opportunity to execute a
// system shutdown or run any cleanup functions. Child processes will be
// orphaned.
//
// TODO: Check to make sure the process is actually running first.
//
// TODO: Kill the entire process tree of the target process.
func Terminate(pid int, code int) (err error) {
	// Temporary workaround
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	return process.Kill()

	/*
		process, err := openProcess(ProcessTerminate, false, uint32(pid))
		if err != nil {
			return err
		}
		defer syscall.CloseHandle(process)
		return terminate(process, uint32(code))
	*/
}

func terminate(process syscall.Handle, exitcode uint32) (err error) {
	return errors.New("not yet implemented")
}

// sameArch returns true if both process belong to the same processor
// architecture (x86 or x64)
func sameArch(processA, processB syscall.Handle) bool {
	arch0, err0 := isWow64Process(processA)
	arch1, err1 := isWow64Process(processB)

	return err0 == nil && err1 == nil && arch0 == arch1
}

// eventFromContext returns an event handle that will be signaled when the
// context is closed. The done channel will be closed once the event has
// been signaled.
func eventFromContext(ctx context.Context) (event syscall.Handle, cancel context.CancelFunc, done <-chan struct{}, err error) {
	ctx, cancel = context.WithCancel(ctx)

	d := make(chan struct{})
	done = d

	event, err = createEvent(nil, true, false, "")
	if err != nil {
		close(d)
		return
	}
	go func() {
		defer close(d)
		<-ctx.Done()
		setEvent(event)
	}()
	return
}
