// +build windows

// Package graceful facilitates orderly shutdown of processes on Windows.
// It does this by injecting a thread into the target process that
// calls ExitProcess() from within the target process' address space.
//
// This package is based on ideas put forth by Andrew Tucker in a Dr. Dobb's
// article published in 1999 titled "A Safer Alternative to TerminateProcess()":
// http://www.drdobbs.com/a-safer-alternative-to-terminateprocess/184416547
//
// This package also draws inspiration from the git-for-windows implementation
// of the same idea:
// https://github.com/git-for-windows/msys2-runtime/pull/15/commits/e9cb332976cf6ba44d9f5fc0ed4f725ce43fe646
package graceful
