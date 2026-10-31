package updater

import "testing"

// Installable says whether this copy can install an update. Only a copy made by
// the Windows setup or the .deb package can.
func TestStatusReportsInstallable(t *testing.T) {
	for _, tc := range []struct {
		name    string
		channel Channel
		want    bool
	}{
		{"check only", ChannelNone, false},
		{"windows setup", ChannelWindowsInstaller, true},
		{"deb package", ChannelDeb, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, err := New(Options{
				ConfigDir:      t.TempDir(),
				CurrentVersion: "1.0.0",
				Channel:        func() Channel { return tc.channel },
			})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if got := svc.Status().Installable; got != tc.want {
				t.Fatalf("Installable = %v, want %v", got, tc.want)
			}
		})
	}
}
