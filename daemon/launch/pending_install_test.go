package launch

import "testing"

// Desktop calls LaunchPendingInstall on every quit path, including when no
// window ever opened. With nothing queued it must do nothing and not fail.
func TestLaunchPendingInstallWithoutAppIsNoop(t *testing.T) {
	var none *App
	if err := none.LaunchPendingInstall(); err != nil {
		t.Fatalf("nil app: %v", err)
	}
	if err := (&App{}).LaunchPendingInstall(); err != nil {
		t.Fatalf("app without a daemon: %v", err)
	}
}
