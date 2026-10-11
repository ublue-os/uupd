package session_test

import (
	"fmt"
	"math"
	"os"
	"testing"

	"github.com/godbus/dbus/v5"
	"github.com/ublue-os/uupd/pkg/session"
)

func TestUserParsingInvalidUID(t *testing.T) {
	t.Parallel()
	testVariants := []dbus.Variant{
		dbus.MakeVariant(0.3),
		dbus.MakeVariant(-1),
		dbus.MakeVariant(math.MaxInt),
		dbus.MakeVariant(math.MinInt),
	}

	userName := dbus.MakeVariant("root")
	for _, uidVariant := range testVariants {
		t.Run(fmt.Sprintf("variant: %v", uidVariant.Value()), func(t *testing.T) {
			t.Parallel()
			_, err := session.ParseUserFromVariant(uidVariant, userName)
			if err == nil {
				t.Fatalf("Parser accepted invalid input: %v %v", uidVariant, userName)
			}
		})
	}
}

func TestUserParsingInvalidName(t *testing.T) {
	t.Parallel()
	testVariants := []dbus.Variant{
		dbus.MakeVariant(0.3),
		dbus.MakeVariant(-1),
		dbus.MakeVariant(math.MaxInt),
		dbus.MakeVariant(math.MinInt),
	}

	uidVariant := dbus.MakeVariant(uint32(0))
	for _, nameVariant := range testVariants {
		t.Run(fmt.Sprintf("variant: %v", uidVariant.Value()), func(t *testing.T) {
			t.Parallel()
			_, err := session.ParseUserFromVariant(uidVariant, nameVariant)
			if err == nil {
				t.Fatalf("Parser accepted invalid input: %v", err)
			}
		})
	}
}

func TestUserParsingValidUser(t *testing.T) {
	t.Parallel()
	testVariants := []struct {
		UidVariant  dbus.Variant
		NameVariant dbus.Variant
	}{
		{dbus.MakeVariant(uint32(10)), dbus.MakeVariant("bob")},
		{dbus.MakeVariant(uint32(20)), dbus.MakeVariant("beatryz")},
		{dbus.MakeVariant(uint32(math.MaxUint16)), dbus.MakeVariant("zorg_the_destroyer")},
	}

	for _, variant := range testVariants {
		t.Run(fmt.Sprintf("variant: %v", variant.NameVariant.Value()), func(t *testing.T) {
			t.Parallel()
			_, err := session.ParseUserFromVariant(variant.UidVariant, variant.NameVariant)
			if err != nil {
				t.Fatalf("Parser rejected valid input: %v", err)
			}
		})
	}
}

func TestGdmGreeterIsNotAnUpdateTarget(t *testing.T) {
	t.Parallel()
	greeter := session.PasswdRecord{
		Name:  "gdm-greeter",
		UID:   60578,
		GID:   42,
		Home:  "/run/gdm/home/gdm-greeter",
		Shell: "/usr/sbin/nologin",
	}

	if session.IsUpdateTarget(greeter) {
		t.Fatalf("Expected the gdm greeter to not be an update target: %v", greeter)
	}

	secondSeat := greeter
	secondSeat.Name = "gdm-greeter-2"
	secondSeat.UID = 60579
	secondSeat.Home = "/run/gdm/home/gdm-greeter-2"
	if session.IsUpdateTarget(secondSeat) {
		t.Fatalf("Expected a second seat greeter to not be an update target: %v", secondSeat)
	}
}

func TestUpdateTargets(t *testing.T) {
	t.Parallel()
	cases := []struct {
		description string
		record      session.PasswdRecord
		expected    bool
	}{
		{"regular user", session.PasswdRecord{Name: "antonio", UID: 1000, GID: 1000, Home: "/home/antonio", Shell: "/bin/bash"}, true},
		{"root", session.PasswdRecord{Name: "root", UID: 0, GID: 0, Home: "/root", Shell: "/bin/bash"}, false},
		{"no home", session.PasswdRecord{Name: "ghost", UID: 4242}, false},
		{"home on tmpfs", session.PasswdRecord{Name: "tmp", UID: 1001, Home: "/tmp/user", Shell: "/bin/bash"}, false},
		{"home under /var/lib", session.PasswdRecord{Name: "var", UID: 1002, Home: "/var/lib/user", Shell: "/bin/bash"}, false},
		{"disabled shell", session.PasswdRecord{Name: "svc", UID: 1003, Home: "/home/svc", Shell: "/sbin/nologin"}, false},
		{"false shell", session.PasswdRecord{Name: "svc2", UID: 1004, Home: "/home/svc2", Shell: "/usr/bin/false"}, false},
		{"root as a directory", session.PasswdRecord{Name: "weird", UID: 1005, Home: "/"}, false},
		{"trailing slash", session.PasswdRecord{Name: "slashy", UID: 1006, Home: "/home/slashy/", Shell: "/bin/bash"}, true},
	}

	for _, testCase := range cases {
		t.Run(testCase.description, func(t *testing.T) {
			t.Parallel()
			if reported := session.IsUpdateTarget(testCase.record); reported != testCase.expected {
				t.Fatalf("IsUpdateTarget(%v) = %v, expected %v", testCase.record, reported, testCase.expected)
			}
		})
	}
}

func TestLookupPasswdResolvesCurrentUser(t *testing.T) {
	t.Parallel()
	record, err := session.LookupPasswd(os.Getuid())
	if err != nil {
		t.Skipf("Skipping, uid %d is unresolvable in this environment: %v", os.Getuid(), err)
	}

	if record.UID != os.Getuid() {
		t.Fatalf("Expected uid %d, got %d", os.Getuid(), record.UID)
	}
	if record.Name == "" {
		t.Fatalf("Expected a user name for uid %d", os.Getuid())
	}
	if record.Home == "" {
		t.Fatalf("Expected a home directory for uid %d", os.Getuid())
	}
}

func TestLookupPasswdRejectsNegativeUID(t *testing.T) {
	t.Parallel()
	if _, err := session.LookupPasswd(-1); err == nil {
		t.Fatal("Expected a negative uid to be rejected")
	}
}
