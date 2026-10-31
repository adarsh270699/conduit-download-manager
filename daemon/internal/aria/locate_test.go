package aria

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeEngineFile(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte("engine"), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestExecutableName(t *testing.T) {
	want := "aria2c"
	if runtime.GOOS == "windows" {
		want = "aria2c.exe"
	}
	if got := ExecutableName(); got != want {
		t.Fatalf("ExecutableName() = %q, want %q", got, want)
	}
}

func TestLocateEngineSearchOrder(t *testing.T) {
	name := ExecutableName()
	root := t.TempDir()
	envBin := writeEngineFile(t, filepath.Join(root, "env", name))
	cfgBin := writeEngineFile(t, filepath.Join(root, "chosen", name))
	exeDir := filepath.Join(root, "install")
	installBin := writeEngineFile(t, filepath.Join(exeDir, "engine", name))
	configDir := filepath.Join(root, "config")
	userBin := writeEngineFile(t, filepath.Join(configDir, "bin", name))
	pathDir := filepath.Join(root, "path")
	pathBin := writeEngineFile(t, filepath.Join(pathDir, name))
	t.Setenv("PATH", pathDir)

	cases := []struct {
		name    string
		in      engineSearch
		want    string
		managed bool
	}{
		{
			name:    "environment variable wins",
			in:      engineSearch{envPath: envBin, configPath: cfgBin, exeDir: exeDir, configDir: configDir},
			want:    envBin,
			managed: false,
		},
		{
			name:    "config path beats the installer engine",
			in:      engineSearch{configPath: cfgBin, exeDir: exeDir, configDir: configDir},
			want:    cfgBin,
			managed: false,
		},
		{
			name:    "installer engine is managed",
			in:      engineSearch{exeDir: exeDir, configDir: configDir},
			want:    installBin,
			managed: true,
		},
		{
			name:    "manual placement in the config bin folder",
			in:      engineSearch{exeDir: filepath.Join(root, "no-install"), configDir: configDir},
			want:    userBin,
			managed: true,
		},
		{
			name:    "PATH is the last resort",
			in:      engineSearch{exeDir: filepath.Join(root, "no-install"), configDir: filepath.Join(root, "no-config")},
			want:    pathBin,
			managed: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := locateEngine(tc.in)
			if err != nil {
				t.Fatalf("locateEngine: %v", err)
			}
			if got.Path != tc.want || got.Managed != tc.managed {
				t.Fatalf("locateEngine() = %+v, want path %s managed %v", got, tc.want, tc.managed)
			}
		})
	}
}

func TestLocateEngineFailures(t *testing.T) {
	name := ExecutableName()
	root := t.TempDir()
	t.Setenv("PATH", filepath.Join(root, "empty"))
	configDir := filepath.Join(root, "config")
	exeDir := filepath.Join(root, "install")
	// An installed engine must not rescue a bad explicit path.
	writeEngineFile(t, filepath.Join(exeDir, "engine", name))

	_, err := locateEngine(engineSearch{exeDir: filepath.Join(root, "no-install"), configDir: configDir})
	if !errors.Is(err, ErrEngineMissing) {
		t.Fatalf("missing engine error = %v, want ErrEngineMissing", err)
	}
	if !strings.Contains(err.Error(), filepath.Join(configDir, "bin")) {
		t.Fatalf("error should name the folder to place aria2c in: %v", err)
	}

	typo := filepath.Join(root, "typo", name)
	if _, err := locateEngine(engineSearch{envPath: typo, exeDir: exeDir, configDir: configDir}); !errors.Is(err, ErrEngineMissing) {
		t.Fatalf("bad environment path error = %v, want ErrEngineMissing", err)
	}
	if _, err := locateEngine(engineSearch{configPath: typo, exeDir: exeDir, configDir: configDir}); !errors.Is(err, ErrEngineMissing) {
		t.Fatalf("bad config path error = %v, want ErrEngineMissing", err)
	}

	if runtime.GOOS != "windows" {
		plain := filepath.Join(root, "plain", name)
		if err := os.MkdirAll(filepath.Dir(plain), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(plain, []byte("not executable"), 0o644); err != nil {
			t.Fatal(err)
		}
		if isUsableEngine(plain) {
			t.Fatal("a non-executable file must not be treated as an engine")
		}
	}
}
