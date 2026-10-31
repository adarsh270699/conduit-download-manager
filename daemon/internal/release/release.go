// Package release keeps the version number consistent across the files that
// must carry it, and reads release notes from CHANGELOG.md.
//
// daemon/version/VERSION is the source of truth. Two other files must repeat
// the numeric part because their formats cannot read it:
//   - desktop/wails.json "productVersion" (Windows file version resource)
//   - extension/manifest.json "version" (browsers accept only dotted numbers)
//
// The NSIS installer and the .deb receive the version from the build scripts,
// so they hold no copy.
package release

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Paths relative to the repository root.
const (
	VersionFile   = "daemon/version/VERSION"
	WailsFile     = "desktop/wails.json"
	ManifestFile  = "extension/manifest.json"
	ChangelogFile = "CHANGELOG.md"
)

// semverPattern accepts MAJOR.MINOR.PATCH with an optional prerelease such as
// -rc.1. Build metadata (+...) is not used by Conduit releases.
var semverPattern = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

// Version is a parsed release version.
type Version struct {
	Major, Minor, Patch int
	Pre                 string
}

// Parse reads a version without a leading v.
func Parse(s string) (Version, error) {
	m := semverPattern.FindStringSubmatch(s)
	if m == nil {
		return Version{}, fmt.Errorf("version %q must look like 1.2.3 or 1.2.3-rc.1 (no leading v)", s)
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch, _ := strconv.Atoi(m[3])
	return Version{Major: major, Minor: minor, Patch: patch, Pre: m[4]}, nil
}

func (v Version) String() string {
	if v.Pre == "" {
		return v.Core()
	}
	return v.Core() + "-" + v.Pre
}

// Core is MAJOR.MINOR.PATCH without the prerelease.
func (v Version) Core() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Quad is the four-part numeric form Windows version resources need.
func (v Version) Quad() string {
	return v.Core() + ".0"
}

// Prerelease reports whether this is a release candidate or similar.
func (v Version) Prerelease() bool {
	return v.Pre != ""
}

// Compare returns -1, 0, or 1 following semver precedence.
func (v Version) Compare(o Version) int {
	for _, d := range []int{v.Major - o.Major, v.Minor - o.Minor, v.Patch - o.Patch} {
		if d != 0 {
			if d < 0 {
				return -1
			}
			return 1
		}
	}
	return comparePre(v.Pre, o.Pre)
}

// comparePre orders prereleases: none > any prerelease; numeric identifiers
// compare numerically and sort before alphanumeric ones.
func comparePre(a, b string) int {
	switch {
	case a == b:
		return 0
	case a == "":
		return 1
	case b == "":
		return -1
	}
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		an, aErr := strconv.Atoi(as[i])
		bn, bErr := strconv.Atoi(bs[i])
		switch {
		case aErr == nil && bErr == nil:
			if an != bn {
				if an < bn {
					return -1
				}
				return 1
			}
		case aErr == nil:
			return -1
		case bErr == nil:
			return 1
		default:
			if c := strings.Compare(as[i], bs[i]); c != 0 {
				return c
			}
		}
	}
	switch {
	case len(as) < len(bs):
		return -1
	case len(as) > len(bs):
		return 1
	}
	return 0
}

// Repo is a checkout of the Conduit repository.
type Repo struct {
	Root string
}

func (r Repo) path(rel string) string {
	return filepath.Join(r.Root, filepath.FromSlash(rel))
}

// Current reads daemon/version/VERSION.
func (r Repo) Current() (Version, error) {
	data, err := os.ReadFile(r.path(VersionFile))
	if err != nil {
		return Version{}, err
	}
	return Parse(strings.TrimSpace(string(data)))
}

var (
	wailsVersionPattern    = regexp.MustCompile(`("productVersion"\s*:\s*")([^"]*)(")`)
	manifestVersionPattern = regexp.MustCompile(`(?m)^(\s*"version"\s*:\s*")([^"]*)(")`)
)

// Check reports every file that disagrees with VERSION. With a non-empty tag,
// it also requires tag == "v"+VERSION and a CHANGELOG section for it.
func (r Repo) Check(tag string) error {
	v, err := r.Current()
	if err != nil {
		return err
	}
	var problems []string
	for _, f := range []struct {
		file    string
		pattern *regexp.Regexp
	}{
		{WailsFile, wailsVersionPattern},
		{ManifestFile, manifestVersionPattern},
	} {
		got, err := r.readField(f.file, f.pattern)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		if got != v.Core() {
			problems = append(problems, fmt.Sprintf("%s has %s, want %s (from %s)", f.file, got, v.Core(), VersionFile))
		}
	}
	if tag != "" {
		if tag != "v"+v.String() {
			problems = append(problems, fmt.Sprintf("tag %s does not match %s %s; the tag must be v%s", tag, VersionFile, v, v))
		}
		if _, err := r.Notes(v.String()); err != nil {
			problems = append(problems, err.Error())
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "\n"))
	}
	return nil
}

func (r Repo) readField(rel string, pattern *regexp.Regexp) (string, error) {
	data, err := os.ReadFile(r.path(rel))
	if err != nil {
		return "", err
	}
	m := pattern.FindSubmatch(data)
	if m == nil {
		return "", fmt.Errorf("%s has no version field", rel)
	}
	return string(m[2]), nil
}

func (r Repo) writeField(rel string, pattern *regexp.Regexp, value string) error {
	path := r.path(rel)
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !pattern.Match(data) {
		return fmt.Errorf("%s has no version field", rel)
	}
	replaced := false
	out := pattern.ReplaceAllFunc(data, func(match []byte) []byte {
		if replaced {
			return match
		}
		replaced = true
		return pattern.ReplaceAll(match, []byte("${1}"+value+"${3}"))
	})
	return os.WriteFile(path, out, 0o644)
}

// Bump moves every version field to next and turns the Unreleased changelog
// section into a dated section for next. next must be newer than the current
// version.
func (r Repo) Bump(next Version, today time.Time) error {
	cur, err := r.Current()
	if err != nil {
		return err
	}
	if next.Compare(cur) <= 0 {
		return fmt.Errorf("new version %s must be newer than %s", next, cur)
	}
	changelog, err := os.ReadFile(r.path(ChangelogFile))
	if err != nil {
		return err
	}
	text := strings.ReplaceAll(string(changelog), "\r\n", "\n")
	updated, err := promoteUnreleased(text, next.String(), today.Format("2006-01-02"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(r.path(VersionFile), []byte(next.String()+"\n"), 0o644); err != nil {
		return err
	}
	if err := r.writeField(WailsFile, wailsVersionPattern, next.Core()); err != nil {
		return err
	}
	if err := r.writeField(ManifestFile, manifestVersionPattern, next.Core()); err != nil {
		return err
	}
	return os.WriteFile(r.path(ChangelogFile), []byte(updated), 0o644)
}

const unreleasedHeading = "## [Unreleased]"

// promoteUnreleased inserts "## [version] - date" under the Unreleased
// heading, so the notes written there belong to the new release and a fresh
// empty Unreleased section stays on top.
func promoteUnreleased(changelog, version, date string) (string, error) {
	idx := strings.Index(changelog, unreleasedHeading)
	if idx < 0 {
		return "", fmt.Errorf("%s has no %q section", ChangelogFile, unreleasedHeading)
	}
	if strings.Contains(changelog, "## ["+version+"]") {
		return "", fmt.Errorf("%s already has a section for %s", ChangelogFile, version)
	}
	end := idx + len(unreleasedHeading)
	body := sectionBody(changelog[end:])
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("the %s section of %s is empty; describe the changes before bumping", unreleasedHeading, ChangelogFile)
	}
	out := changelog[:end] + "\n\n## [" + version + "] - " + date + changelog[end:]

	// Keep the compare links at the bottom current:
	//   [Unreleased]: .../compare/v1.2.0...HEAD
	// becomes
	//   [Unreleased]: .../compare/v1.3.0...HEAD
	//   [1.3.0]: .../compare/v1.2.0...v1.3.0
	if m := unreleasedLink.FindStringSubmatchIndex(out); m != nil {
		base, prev := out[m[2]:m[3]], out[m[4]:m[5]]
		replacement := "[Unreleased]: " + base + "/compare/v" + version + "...HEAD\n" +
			"[" + version + "]: " + base + "/compare/" + prev + "...v" + version
		out = out[:m[0]] + replacement + out[m[1]:]
	}
	return out, nil
}

var unreleasedLink = regexp.MustCompile(`(?m)^\[Unreleased\]: (\S+)/compare/(v\S+)\.\.\.HEAD$`)

// linkDefinition matches the "[1.2.0]: https://..." lines at the end of the file.
var linkDefinition = regexp.MustCompile(`^\[[^\]]+\]: `)

// sectionBody returns the text up to the next "## " heading or the link
// definitions at the end of the file.
func sectionBody(rest string) string {
	var b strings.Builder
	sc := bufio.NewScanner(strings.NewReader(rest))
	first := true
	for sc.Scan() {
		line := sc.Text()
		if !first && (strings.HasPrefix(line, "## ") || linkDefinition.MatchString(line)) {
			break
		}
		first = false
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// Notes returns the CHANGELOG body for version, without its heading.
func (r Repo) Notes(version string) (string, error) {
	data, err := os.ReadFile(r.path(ChangelogFile))
	if err != nil {
		return "", err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	heading := "## [" + version + "]"
	idx := strings.Index(text, heading)
	if idx < 0 {
		return "", fmt.Errorf("%s has no %q section", ChangelogFile, heading)
	}
	rest := text[idx+len(heading):]
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[nl+1:]
	} else {
		rest = ""
	}
	body := strings.TrimSpace(sectionBody("\n" + rest))
	if body == "" {
		return "", fmt.Errorf("the %s section of %s is empty", heading, ChangelogFile)
	}
	return body, nil
}
