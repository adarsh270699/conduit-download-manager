package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	good := map[string]Version{
		"1.2.3":            {1, 2, 3, ""},
		"0.0.1":            {0, 0, 1, ""},
		"10.20.30-rc.1":    {10, 20, 30, "rc.1"},
		"1.0.0-alpha.beta": {1, 0, 0, "alpha.beta"},
	}
	for in, want := range good {
		got, err := Parse(in)
		if err != nil || got != want {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", in, got, err, want)
		}
		if got.String() != in {
			t.Errorf("String() = %q, want %q", got.String(), in)
		}
	}
	for _, in := range []string{"", "v1.2.3", "1.2", "1.2.3.4", "01.2.3", "1.2.3-", "1.2.3+build", " 1.2.3"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", in)
		}
	}
}

func TestCompare(t *testing.T) {
	// Each version is lower than the next (semver 2.0 §11 example plus extras).
	ordered := []string{
		"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta",
		"1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.0.1", "1.1.0", "2.0.0",
	}
	for i := 0; i+1 < len(ordered); i++ {
		a, _ := Parse(ordered[i])
		b, _ := Parse(ordered[i+1])
		if a.Compare(b) != -1 || b.Compare(a) != 1 {
			t.Errorf("%s should sort before %s", a, b)
		}
		if a.Compare(a) != 0 {
			t.Errorf("%s should equal itself", a)
		}
	}
}

func TestQuadAndPrerelease(t *testing.T) {
	v, _ := Parse("1.3.0-rc.2")
	if v.Core() != "1.3.0" || v.Quad() != "1.3.0.0" || !v.Prerelease() {
		t.Fatalf("got core %s quad %s pre %v", v.Core(), v.Quad(), v.Prerelease())
	}
}

const testChangelog = `# Changelog

## [Unreleased]

### Fixed

- A bug.

## [1.2.0] - 2026-10-25

### Added

- A feature.

[Unreleased]: https://example.test/repo/compare/v1.2.0...HEAD
[1.2.0]: https://example.test/repo/compare/v1.1.0...v1.2.0
`

// newRepo writes a minimal checkout at version 1.2.0.
func newRepo(t *testing.T, changelog string) Repo {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		VersionFile:   "1.2.0\n",
		WailsFile:     "{\n  \"info\": {\n    \"productVersion\": \"1.2.0\",\n    \"comments\": \"x\"\n  }\n}\n",
		ManifestFile:  "{\n  \"manifest_version\": 3,\n  \"version\": \"1.2.0\",\n  \"nested\": {\"version\": \"9.9.9\"}\n}\n",
		ChangelogFile: changelog,
	}
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return Repo{Root: root}
}

func read(t *testing.T, r Repo, rel string) string {
	t.Helper()
	data, err := os.ReadFile(r.path(rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCheckAgreeingRepo(t *testing.T) {
	r := newRepo(t, testChangelog)
	if err := r.Check(""); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if err := r.Check("v1.2.0"); err != nil {
		t.Fatalf("Check with tag: %v", err)
	}
}

func TestCheckReportsMismatches(t *testing.T) {
	r := newRepo(t, testChangelog)
	if err := os.WriteFile(r.path(VersionFile), []byte("1.2.1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := r.Check("v1.2.0")
	if err == nil {
		t.Fatal("Check passed with mismatched files")
	}
	for _, want := range []string{WailsFile, ManifestFile, "tag v1.2.0", "## [1.2.1]"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestBump(t *testing.T) {
	r := newRepo(t, testChangelog)
	next, _ := Parse("1.3.0-rc.1")
	if err := r.Bump(next, time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("Bump: %v", err)
	}
	if got := strings.TrimSpace(read(t, r, VersionFile)); got != "1.3.0-rc.1" {
		t.Errorf("VERSION = %q", got)
	}
	if !strings.Contains(read(t, r, WailsFile), `"productVersion": "1.3.0"`) {
		t.Errorf("wails.json not bumped:\n%s", read(t, r, WailsFile))
	}
	manifest := read(t, r, ManifestFile)
	if !strings.Contains(manifest, `"version": "1.3.0"`) || !strings.Contains(manifest, `"version": "9.9.9"`) {
		t.Errorf("manifest should bump only the top-level version:\n%s", manifest)
	}
	cl := read(t, r, ChangelogFile)
	for _, want := range []string{
		"## [Unreleased]\n\n## [1.3.0-rc.1] - 2026-11-02\n\n### Fixed",
		"[Unreleased]: https://example.test/repo/compare/v1.3.0-rc.1...HEAD\n[1.3.0-rc.1]: https://example.test/repo/compare/v1.2.0...v1.3.0-rc.1\n",
	} {
		if !strings.Contains(cl, want) {
			t.Errorf("changelog missing %q:\n%s", want, cl)
		}
	}
	if err := r.Check("v1.3.0-rc.1"); err != nil {
		t.Errorf("Check after bump: %v", err)
	}
	notes, err := r.Notes("1.3.0-rc.1")
	if err != nil || notes != "### Fixed\n\n- A bug." {
		t.Errorf("Notes = %q, %v", notes, err)
	}
}

func TestBumpRefusals(t *testing.T) {
	r := newRepo(t, testChangelog)
	for _, v := range []string{"1.2.0", "1.1.9", "1.2.0-rc.1"} {
		next, _ := Parse(v)
		if err := r.Bump(next, time.Now()); err == nil {
			t.Errorf("Bump(%s) succeeded, want refusal", v)
		}
	}

	empty := newRepo(t, strings.Replace(testChangelog, "### Fixed\n\n- A bug.\n\n", "", 1))
	next, _ := Parse("1.3.0")
	if err := empty.Bump(next, time.Now()); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("Bump with empty Unreleased = %v, want empty-section error", err)
	}
	if got := strings.TrimSpace(read(t, empty, VersionFile)); got != "1.2.0" {
		t.Errorf("a refused bump changed VERSION to %s", got)
	}
}

func TestNotesStopsAtLinkDefinitions(t *testing.T) {
	r := newRepo(t, strings.ReplaceAll(testChangelog, "\n", "\r\n"))
	notes, err := r.Notes("1.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if notes != "### Added\n\n- A feature." {
		t.Errorf("Notes = %q", notes)
	}
	if _, err := r.Notes("9.9.9"); err == nil {
		t.Error("Notes for a missing version succeeded")
	}
}

// TestRepositoryVersionsAgree runs the real check against this checkout so a
// hand edit that forgets a file fails CI before any release is tagged.
func TestRepositoryVersionsAgree(t *testing.T) {
	repo := Repo{Root: filepath.Join("..", "..", "..")}
	if err := repo.Check(""); err != nil {
		t.Fatalf("version fields disagree; run `go run ./tools/release bump <version>` instead of editing by hand:\n%v", err)
	}
}
