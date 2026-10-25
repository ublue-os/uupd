package filelock

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

var ErrLocked = errors.New("lock is held by another process")

func IsFileLocked(file *os.File) bool {
	lock := syscall.Flock_t{}
	err := syscall.FcntlFlock(file.Fd(), syscall.F_GETLK, &lock)
	if err != nil {
		return false
	}
	return lock.Type != syscall.F_UNLCK
}

func GetDefaultLockfile() string {
	return "/run/uupd.lock"
}

func OpenLockfile(filepath string) (*os.File, error) {
	return os.OpenFile(filepath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
}

type TimeoutConfig struct {
	Tries int
}

func AcquireLock(file *os.File, timeout TimeoutConfig) error {
	maxTries := timeout.Tries
	if maxTries < 1 {
		maxTries = 1
	}

	for tries := 0; tries < maxTries; tries++ {
		err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			return fmt.Errorf("failed to acquire lock: %w", err)
		}

		if tries < maxTries-1 {
			time.Sleep(1 * time.Second)
		}
	}

	return ErrLocked
}

func ReleaseLock(file *os.File) error {
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_UN); err != nil {
		return err
	}
	return syscall.Close(int(file.Fd()))
}
