package identity

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/dscof/qm-agent/internal/config"
)

// QuartermasterTLS builds TLS settings for connecting to Quartermaster.
// Client certificates are optional; ca_file alone verifies the server cert.
// Returns nil when no mtls fields are configured.
func QuartermasterTLS(mtls config.MTLSConfig) (*tls.Config, error) {
	hasCert := mtls.CertFile != "" || mtls.KeyFile != ""
	if hasCert && (mtls.CertFile == "" || mtls.KeyFile == "") {
		return nil, fmt.Errorf("quartermaster.mtls.cert_file and key_file must both be set")
	}

	if mtls.CertFile == "" && mtls.CAFile == "" {
		return nil, nil
	}

	cfg := &tls.Config{MinVersion: tls.VersionTLS12}

	if mtls.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(mtls.CertFile, mtls.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load mTLS key pair: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}

	if mtls.CAFile != "" {
		caPEM, err := os.ReadFile(mtls.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("parse CA file %s", mtls.CAFile)
		}
		cfg.RootCAs = pool
	}

	return cfg, nil
}
