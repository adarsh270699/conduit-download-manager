// Command release manages the Conduit version number and release notes.
//
// Run it from the daemon directory:
//
//	go run ./tools/release print            # 1.3.0-rc.1
//	go run ./tools/release print -core      # 1.3.0
//	go run ./tools/release print -quad      # 1.3.0.0
//	go run ./tools/release bump 1.3.0       # update every version field + CHANGELOG
//	go run ./tools/release check            # all version fields agree
//	go run ./tools/release check -tag v1.3.0
//	go run ./tools/release notes 1.3.0      # CHANGELOG section for the release body
//	go run ./tools/release prerelease       # "true" or "false"
//	go run ./tools/release previous v1.2.0 v1.1.0  # candidate tags: prints the highest one older than VERSION
//
// -root points at the repository root and defaults to the parent directory.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"conduit/daemon/internal/release"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: release <print|bump|check|notes|prerelease|previous> [flags]")
	}
	cmd, rest := args[0], args[1:]
	fs := flag.NewFlagSet(cmd, flag.ContinueOnError)
	root := fs.String("root", "..", "repository root")
	core := fs.Bool("core", false, "print: MAJOR.MINOR.PATCH only")
	quad := fs.Bool("quad", false, "print: four-part Windows version")
	tag := fs.String("tag", "", "check: also require this git tag to match")
	if err := fs.Parse(rest); err != nil {
		return err
	}
	repo := release.Repo{Root: *root}

	switch cmd {
	case "print":
		v, err := repo.Current()
		if err != nil {
			return err
		}
		switch {
		case *quad:
			fmt.Println(v.Quad())
		case *core:
			fmt.Println(v.Core())
		default:
			fmt.Println(v)
		}
	case "prerelease":
		v, err := repo.Current()
		if err != nil {
			return err
		}
		fmt.Println(v.Prerelease())
	case "bump":
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: release bump <version>")
		}
		next, err := release.Parse(fs.Arg(0))
		if err != nil {
			return err
		}
		if err := repo.Bump(next, time.Now()); err != nil {
			return err
		}
		fmt.Printf("Bumped to %s. Review the diff, commit, merge, then tag v%s on main.\n", next, next)
	case "check":
		if err := repo.Check(*tag); err != nil {
			return err
		}
		v, _ := repo.Current()
		fmt.Printf("Version fields agree: %s\n", v)
	case "notes":
		if fs.NArg() != 1 {
			return fmt.Errorf("usage: release notes <version>")
		}
		notes, err := repo.Notes(fs.Arg(0))
		if err != nil {
			return err
		}
		fmt.Println(notes)
	case "previous":
		v, err := repo.Current()
		if err != nil {
			return err
		}
		// Prints the tag to test an upgrade from, or nothing when none is older.
		if tag := release.Previous(v, fs.Args()); tag != "" {
			fmt.Println(tag)
		}
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
	return nil
}
