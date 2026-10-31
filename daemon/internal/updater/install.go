package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
)

// pageSize is how many queue entries one TellWaiting call reads. It matches the
// page size the downloads list uses.
const pageSize = 100

var (
	// ErrNotDownloaded means no verified installer is waiting to be installed.
	ErrNotDownloaded = errors.New("no downloaded installer is ready to install")
	// ErrUnsupportedChannel means this copy was not installed by a supported installer.
	ErrUnsupportedChannel = errors.New("not installed by a supported installer")
	// ErrDownloadsActive means a download could be interrupted by quitting.
	ErrDownloadsActive = errors.New("downloads are active; wait for them to finish or pause them")
	// ErrRecordMismatch means update.json names a file other than the one download wrote.
	ErrRecordMismatch = errors.New("the saved update record does not match the downloaded installer; download it again")
	// ErrChecksum means the installer on disk no longer matches its recorded hash.
	ErrChecksum = errors.New("the downloaded installer no longer matches its checksum; download it again")
)

// debRootStep runs as root under pkexec. It copies the package into a directory
// only root can write, checks the copy against the recorded SHA-256, and installs
// the copy. The bytes that are checked are the bytes that are installed, so the
// user cannot swap the file between the check and the install. $1 is the package
// path and $2 its SHA-256.
const debRootStep = `tmp=$(mktemp -d) || exit 1
cp "$1" "$tmp/conduit.deb" && echo "$2  $tmp/conduit.deb" | sha256sum -c - && apt-get install -y "$tmp/conduit.deb"
status=$?
rm -rf "$tmp"
exit $status`

// debInstallScript runs the root step, then starts Conduit again even if the
// install was cancelled or failed, so the user is not left with no window.
// $1 is the .deb path and $2 its SHA-256.
const debInstallScript = `pkexec /bin/sh -c '` + debRootStep + `' sh "$1" "$2"; exec /usr/bin/conduit --background`

// Command is a program the daemon starts after it has quit.
type Command struct {
	Name string
	Args []string
}

// busy reports whether quitting now could interrupt a download. Active and
// queued (waiting) downloads count. Paused ones do not, because aria2.session
// restores them. An RPC failure counts as busy: waiting is the safer default.
// With the engine down nothing can be downloading, so the queue is not read.
func (s *Service) busy(ctx context.Context) (bool, error) {
	if s.downloads == nil || (s.engineRunning != nil && !s.engineRunning()) {
		return false, nil
	}
	active, err := s.downloads.TellActive(ctx)
	if err != nil {
		return true, fmt.Errorf("read active downloads: %w", err)
	}
	if len(active) > 0 {
		return true, nil
	}
	// TellWaiting also returns paused items, so filter them by status.
	for offset := 0; ; offset += pageSize {
		page, err := s.downloads.TellWaiting(ctx, offset, pageSize)
		if err != nil {
			return true, fmt.Errorf("read queued downloads: %w", err)
		}
		for _, item := range page {
			if item.Status == "waiting" {
				return true, nil
			}
		}
		if len(page) < pageSize {
			return false, nil
		}
	}
}

// Install queues the downloaded installer. It moves the service to installing
// and stores the command for TakePendingInstall. It does not start anything:
// the caller answers the request, then quits, and the installer starts only
// after Conduit has released its files and lock.
func (s *Service) Install(ctx context.Context) error {
	s.mu.Lock()
	if s.status.State != StateDownloaded {
		s.mu.Unlock()
		return ErrNotDownloaded
	}
	path, ok := s.recordedInstaller()
	if !ok {
		s.mu.Unlock()
		return ErrRecordMismatch
	}
	want := s.rec.DownloadedSHA256
	cmd, err := installCommand(runtime.GOOS, s.channel, path, want)
	if err != nil {
		s.mu.Unlock()
		return err
	}
	// Claim the step first, so a check or download cannot start while the
	// installer is verified and the queue is read.
	s.status.State = StateInstalling
	s.mu.Unlock()

	if err := verifyInstaller(path, want); err != nil {
		s.mu.Lock()
		s.status.State = StateError
		s.status.Error = ErrChecksum.Error()
		s.mu.Unlock()
		return fmt.Errorf("%w: %v", ErrChecksum, err)
	}

	busy, err := s.busy(ctx)
	if err != nil {
		log.Printf("Warning: could not read the download queue; treating it as busy: %v", err)
	}
	if busy {
		s.mu.Lock()
		s.status.State = StateDownloaded
		s.mu.Unlock()
		return ErrDownloadsActive
	}

	s.mu.Lock()
	s.pending = &cmd
	s.mu.Unlock()
	return nil
}

// TakePendingInstall returns the queued installer command, or nil. It returns
// the command once, so a second caller cannot launch it twice.
func (s *Service) TakePendingInstall() *Command {
	s.mu.Lock()
	defer s.mu.Unlock()
	cmd := s.pending
	s.pending = nil
	return cmd
}

// installCommand returns the program that installs the file at path on this
// channel. Windows runs the NSIS installer silently and asks it to start
// Conduit in the tray. Linux installs through pkexec, which asks for a
// password once.
func installCommand(goos string, ch Channel, path, sum string) (Command, error) {
	switch {
	case goos == "windows" && ch == ChannelWindowsInstaller:
		return Command{Name: path, Args: []string{"/S", "/LAUNCH=background"}}, nil
	case goos == "linux" && ch == ChannelDeb:
		return Command{Name: "/bin/sh", Args: []string{"-c", debInstallScript, "sh", path, sum}}, nil
	}
	return Command{}, ErrUnsupportedChannel
}

// verifyInstaller hashes the file at path and compares it with want.
func verifyInstaller(path, want string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return err
	}
	if hex.EncodeToString(hash.Sum(nil)) != want {
		return errors.New("hash differs from the recorded value")
	}
	return nil
}
