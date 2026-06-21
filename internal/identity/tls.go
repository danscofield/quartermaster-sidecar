package identity

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/dscof/qm-agent/internal/config"
)

func tlsConfigFromFiles(mtls config.MTLSConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(mtls.CertFile, mtls.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load mTLS key pair: %w", err)
	}

	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
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
