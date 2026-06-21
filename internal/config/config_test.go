package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dscof/qm-agent/internal/config"
)

func TestLoadSPIREMTLS(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
quartermaster:
  url: https://qm.example.com
  mtls:
    ca_file: /etc/qm/ca.pem
identity:
  type: spire
  spire:
    mode: mtls
output:
  dir: /var/run/qm-agent
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Identity.SPIRE.Mode != "mtls" {
		t.Fatalf("mode = %q", cfg.Identity.SPIRE.Mode)
	}
}

func TestLoadAWSRequiresMTLS(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte(`
quartermaster:
  url: https://qm.example.com
identity:
  type: aws
  aws:
    region: us-east-1
output:
  dir: /var/run/qm-agent
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := config.Load(path); err == nil {
		t.Fatal("expected validation error for missing mtls certs")
	}
}
