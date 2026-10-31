package server

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"conduit/daemon/internal/platform"
)

type extensionManifest struct {
	Key                     string `json:"key"`
	BrowserSpecificSettings struct {
		Gecko struct {
			ID string `json:"id"`
		} `json:"gecko"`
	} `json:"browser_specific_settings"`
}

func readExtensionManifest() ([]byte, error) {
	// Prefer the installed layout next to the executable, then the working
	// directory used by source checkouts and tests.
	var starts []string
	if exe, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(exe))
	}
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}

	var candidates []string
	for _, start := range starts {
		dir := start
		for i := 0; i < 8; i++ {
			candidates = append(candidates, filepath.Join(dir, "extension", "manifest.json"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	// Installed packages keep the extension away from the executable:
	// /usr/share/conduit/extension on Linux and
	// %LocalAppData%\Programs\Conduit\extension on Windows.
	for _, dir := range platform.ExtensionCandidates() {
		candidates = append(candidates, filepath.Join(dir, "manifest.json"))
	}

	seen := map[string]bool{}
	for _, candidate := range candidates {
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		data, err := os.ReadFile(candidate)
		if err == nil {
			return data, nil
		}
	}
	return nil, errors.New("extension manifest not found")
}

func extensionOrigins(data []byte) ([]string, error) {
	var manifest extensionManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}

	var origins []string
	if manifest.Key != "" {
		id, err := chromeExtensionID(manifest.Key)
		if err != nil {
			return nil, err
		}
		origins = append(origins, "chrome-extension://"+id)
	}
	if id := manifest.BrowserSpecificSettings.Gecko.ID; id != "" {
		origins = append(origins, "moz-extension://"+id)
	}
	if len(origins) == 0 {
		return nil, errors.New("extension manifest has no extension id")
	}
	return origins, nil
}

func chromeExtensionID(keyB64 string) (string, error) {
	der, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(der)
	const alphabet = "abcdefghijklmnop"
	id := make([]byte, 32)
	for i, b := range sum[:16] {
		id[i*2] = alphabet[b>>4]
		id[i*2+1] = alphabet[b&0x0f]
	}
	return string(id), nil
}

func loadAllowedExtensionOrigins() map[string]struct{} {
	allowed := map[string]struct{}{}
	data, err := readExtensionManifest()
	if err != nil {
		return allowed
	}
	origins, err := extensionOrigins(data)
	if err != nil {
		return allowed
	}
	for _, origin := range origins {
		allowed[origin] = struct{}{}
	}
	return allowed
}
