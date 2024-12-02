package lib

import (
	"fmt"
	"io"
	"os"
	"syscall"
	"time"
)

const fileLockPath string = "/run/uupd.lock"

func IsFileLocked(file *os.File) bool {
	lock := syscall.Flock_t{
		Type:   syscall.F_WRLCK,
		Whence: io.SeekStart,
	}
	err := syscall.FcntlFlock(file.Fd(), syscall.F_GETLK, &lock)
	if err != nil {
		return false
	}
	return lock.Type != syscall.F_UNLCK
}

func AcquireLock() (*os.File, error) {
	file, err := os.OpenFile(fileLockPath, os.O_RDONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return nil, err
	}

	timeout := 5.0
	startTime := time.Now()
	var lockFile *os.File

	for time.Since(startTime).Seconds() < timeout {
		err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			lockFile = file
			break
		}

		time.Sleep(1 * time.Second)
	}

	if lockFile == nil {
		file.Close()
		return nil, fmt.Errorf("Could not acquire lock at %s", fileLockPath)
	}

	return lockFile, nil
}

func ReleaseLock(file *os.File) error {
	return syscall.Close(int(file.Fd()))
}
