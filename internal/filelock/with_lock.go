package filelock

import (
	"os"

	"k8s.io/client-go/tools/clientcmd"
)

func init() {
	// We're using our own file locking mechanism
	clientcmd.UseModifyConfigLock = false
}

func WithLock(configAccess clientcmd.ConfigAccess, action func() error) error {
	return withLock(configAccess.GetDefaultFilename()+".lock", writeLock, action)
}

func WithRLock(configAccess clientcmd.ConfigAccess, action func() error) error {
	return withLock(configAccess.GetDefaultFilename()+".lock", readLock, action)
}

func withLock(filename string, lt lockType, action func() error) error {
	lockfile, err := os.Create(filename)
	if err != nil {
		return err
	}
	err = lock(lockfile, lt)
	if err != nil {
		return err
	}
	defer func() { _ = Unlock(lockfile) }()
	return action()
}
