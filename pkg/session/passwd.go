package session

import (
	"errors"
	"fmt"
	"os/exec"
	osUser "os/user"
	"path/filepath"
	"strconv"
	"strings"
)

const getentPath = "/usr/bin/getent"

var ErrUnresolvableUser = errors.New("user cannot be resolved by the system NSS")

type PasswdRecord struct {
	Name  string
	UID   int
	GID   int
	Home  string
	Shell string
}

func LookupPasswd(uid int) (PasswdRecord, error) {
	if uid < 0 {
		return PasswdRecord{}, fmt.Errorf("%w: negative uid %d", ErrUnresolvableUser, uid)
	}

	if found, err := osUser.LookupId(strconv.Itoa(uid)); err == nil {
		return PasswdRecord{
			Name: found.Username,
			UID:  uid,
			GID:  parseIntOrZero(found.Gid),
			Home: found.HomeDir,
		}, nil
	}

	out, err := exec.Command(getentPath, "passwd", strconv.Itoa(uid)).Output()
	if err != nil {
		return PasswdRecord{}, fmt.Errorf("%w: uid %d: %v", ErrUnresolvableUser, uid, err)
	}

	return parsePasswdEntry(string(out))
}

func LookupUserName(uid int) (string, error) {
	record, err := LookupPasswd(uid)
	if err != nil {
		return "", err
	}
	return record.Name, nil
}

func parsePasswdEntry(output string) (PasswdRecord, error) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Split(line, ":")
		if len(fields) < 7 {
			continue
		}

		uid, err := strconv.Atoi(fields[2])
		if err != nil {
			continue
		}

		return PasswdRecord{
			Name:  fields[0],
			UID:   uid,
			GID:   parseIntOrZero(fields[3]),
			Home:  fields[5],
			Shell: fields[6],
		}, nil
	}

	return PasswdRecord{}, fmt.Errorf("%w: no passwd entry in %q", ErrUnresolvableUser, strings.TrimSpace(output))
}

func parseIntOrZero(value string) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

var volatileRoots = []string{"/run", "/var/run", "/var/lib", "/var/tmp", "/tmp"}

func IsUpdateTarget(record PasswdRecord) bool {
	if record.UID <= 0 {
		return false
	}
	if !isPersistentHome(record.Home) {
		return false
	}
	if record.Shell != "" && isDisabledShell(record.Shell) {
		return false
	}
	return true
}

func isPersistentHome(home string) bool {
	if home == "" {
		return false
	}

	home = filepath.Clean(home)
	if home == "/" {
		return false
	}

	for _, root := range volatileRoots {
		if home == root || strings.HasPrefix(home, root+"/") {
			return false
		}
	}
	return true
}

func isDisabledShell(shell string) bool {
	switch filepath.Base(shell) {
	case "nologin", "false":
		return true
	}
	return false
}
