package version

import (
	"regexp"
	"testing"
)

func TestVersionIsSemver(t *testing.T) {
	if !regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`).MatchString(Version) {
		t.Fatalf("VERSION %q is not MAJOR.MINOR.PATCH[-pre]", Version)
	}
}

func TestString(t *testing.T) {
	defer func(c string) { Commit = c }(Commit)

	Commit = "dev"
	if String() != Version {
		t.Errorf("dev build: %q", String())
	}
	Commit = "0123456789abcdef0123"
	if want := Version + " (0123456789ab)"; String() != want {
		t.Errorf("release build: %q, want %q", String(), want)
	}
}
