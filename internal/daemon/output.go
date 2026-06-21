package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var unsafeBilletName = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// BilletPaths holds on-disk paths for one billet's credentials.
type BilletPaths struct {
	Dir       string
	TokenFile string
	KeyFile   string
	CertFile  string
}

func billetPaths(outputDir, billetName string) BilletPaths {
	dir := filepath.Join(outputDir, "billets", sanitizeBilletName(billetName))
	return BilletPaths{
		Dir:       dir,
		TokenFile: filepath.Join(dir, "token"),
		KeyFile:   filepath.Join(dir, "key.pem"),
		CertFile:  filepath.Join(dir, "cert.pem"),
	}
}

func sanitizeBilletName(name string) string {
	s := strings.TrimSpace(name)
	if s == "" {
		return "_"
	}
	return unsafeBilletName.ReplaceAllString(s, "_")
}

// Manifest is written to output.dir/manifest.json for consumers.
type Manifest struct {
	UpdatedAt string                    `json:"updated_at"`
	Billets   map[string]ManifestBillet `json:"billets"`
}

type ManifestBillet struct {
	Name      string `json:"name"`
	ExpiresAt string `json:"expires_at"`
	TokenPath string `json:"token_path"`
	CertPath  string `json:"cert_path,omitempty"`
	KeyPath   string `json:"key_path,omitempty"`
}

func writeManifest(outputDir string, entries map[string]ManifestBillet) error {
	manifest := Manifest{
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Billets:   entries,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(outputDir, "manifest.json"), data, 0o644)
}

func pruneBilletDirs(outputDir string, active map[string]struct{}) error {
	root := filepath.Join(outputDir, "billets")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	activeDirs := make(map[string]struct{}, len(active))
	for name := range active {
		activeDirs[sanitizeBilletName(name)] = struct{}{}
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, ok := activeDirs[entry.Name()]; ok {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
			return fmt.Errorf("remove stale billet dir %s: %w", entry.Name(), err)
		}
	}
	return nil
}
