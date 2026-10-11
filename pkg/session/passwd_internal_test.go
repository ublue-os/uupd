package session

import (
	"os"
	"testing"

	appLogging "github.com/ublue-os/uupd/pkg/logging"
)

var discardLogger = appLogging.NewMuteLogger()

func TestParsePasswdEntry(t *testing.T) {
	t.Parallel()
	cases := []struct {
		description string
		output      string
		expected    PasswdRecord
	}{
		{
			"regular entry",
			"antonio:x:1000:1000:Antonio:/home/antonio:/bin/bash\n",
			PasswdRecord{Name: "antonio", UID: 1000, GID: 1000, Home: "/home/antonio", Shell: "/bin/bash"},
		},
		{
			"dynamic userdb entry",
			"gdm-greeter:x:60578:42:GDM Greeter:/run/gdm/home/gdm-greeter:/usr/sbin/nologin\n",
			PasswdRecord{Name: "gdm-greeter", UID: 60578, GID: 42, Home: "/run/gdm/home/gdm-greeter", Shell: "/usr/sbin/nologin"},
		},
		{
			"multiple nss sources",
			"gdm-greeter:x:60578:42::/run/gdm/home/gdm-greeter:/usr/sbin/nologin\ngdm-greeter:x:60578:42::/run/gdm/home/gdm-greeter:/usr/sbin/nologin\n",
			PasswdRecord{Name: "gdm-greeter", UID: 60578, GID: 42, Home: "/run/gdm/home/gdm-greeter", Shell: "/usr/sbin/nologin"},
		},
		{
			"no trailing newline",
			"root:x:0:0:root:/root:/bin/bash",
			PasswdRecord{Name: "root", UID: 0, GID: 0, Home: "/root", Shell: "/bin/bash"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()
			parsed, err := parsePasswdEntry(testCase.output)
			if err != nil {
				t.Fatalf("Parser rejected valid getent output %q: %v", testCase.output, err)
			}
			if parsed != testCase.expected {
				t.Fatalf("Parser returned %v, expected %v", parsed, testCase.expected)
			}
		})
	}
}

func TestParsePasswdEntryRejectsGarbage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		description string
		output      string
	}{
		{"empty output", ""},
		{"only whitespace", "  \n\n"},
		{"too few fields", "gdm-greeter:x:60578\n"},
		{"non numeric uid", "gdm-greeter:x:notanumber:42::/run:/usr/sbin/nologin\n"},
		{"comments", "# a comment\n"},
	}

	for _, testCase := range cases {
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()
			if _, err := parsePasswdEntry(testCase.output); err == nil {
				t.Fatalf("Parser accepted invalid getent output: %q", testCase.output)
			}
		})
	}
}

func TestFilterUsers(t *testing.T) {
	t.Parallel()
	users := []User{
		{UID: 0, Name: "root"},
		{UID: os.Getuid(), Name: "tester"},
		{UID: 60578, Name: "gdm-greeter"},
	}

	filtered := filterUsers(users, discardLogger)

	if len(filtered) > 1 {
		t.Fatalf("Expected at most one update target, got %v", filtered)
	}
	if len(filtered) == 1 && filtered[0].UID != os.Getuid() {
		t.Fatalf("Expected only the current user to survive, got %v", filtered)
	}
	for _, user := range filtered {
		if user.UID == 0 {
			t.Fatalf("root must never be an update target: %v", filtered)
		}
	}
}

func TestFilterUsersAlwaysDropsRoot(t *testing.T) {
	t.Parallel()
	if filtered := filterUsers([]User{{UID: 0, Name: "root"}}, discardLogger); len(filtered) != 0 {
		t.Fatalf("Expected root to be filtered out, got %v", filtered)
	}
}
