package identity

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/dscof/qm-agent/internal/config"
)

// Credential is a workload identity presented to Quartermaster.
type Credential struct {
	SubjectToken     string
	SubjectTokenType string
}

// Source obtains workload identity credentials and optional TLS material for Quartermaster.
type Source interface {
	// Credential returns a subject token for token exchange. Empty when identity is conveyed via mTLS only.
	Credential(ctx context.Context) (*Credential, error)

	// TLSConfig returns TLS settings for the Quartermaster client connection.
	// When non-nil, overrides quartermaster.mtls from config.
	TLSConfig(ctx context.Context) (*tls.Config, error)

	Close() error
}

// New builds the configured identity source.
func New(ctx context.Context, cfg *config.Config) (Source, error) {
	switch cfg.Identity.Type {
	case "spire":
		return newSPIRE(ctx, cfg)
	case "aws":
		return newAWS(cfg)
	case "gcp":
		return newGCP(cfg)
	default:
		return nil, fmt.Errorf("unsupported identity type %q", cfg.Identity.Type)
	}
}
