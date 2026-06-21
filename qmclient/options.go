package qmclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
)

// Config holds connection settings for a Quartermaster client.
type Config struct {
	BaseURL     string
	BearerToken string
	MTLS        *MTLSConfig
	HTTPClient  *http.Client
}

// MTLSConfig configures mutual TLS for token exchange and billet discovery.
type MTLSConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
}

// New creates a high-level Quartermaster API client.
func New(cfg Config) (*API, error) {
	opts := []ClientOption{}

	if cfg.HTTPClient != nil {
		opts = append(opts, WithHTTPClient(cfg.HTTPClient))
	} else if cfg.MTLS != nil {
		httpClient, err := httpClientWithMTLS(cfg.MTLS)
		if err != nil {
			return nil, err
		}
		opts = append(opts, WithHTTPClient(httpClient))
	}

	if cfg.BearerToken != "" {
		token := cfg.BearerToken
		opts = append(opts, WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("Authorization", "Bearer "+token)
			return nil
		}))
	}

	client, err := NewClientWithResponses(cfg.BaseURL, opts...)
	if err != nil {
		return nil, err
	}
	return &API{client: client}, nil
}

func httpClientWithMTLS(mtls *MTLSConfig) (*http.Client, error) {
	cert, err := tls.LoadX509KeyPair(mtls.CertFile, mtls.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load client certificate: %w", err)
	}

	tlsCfg := &tls.Config{
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
		tlsCfg.RootCAs = pool
	}

	return &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
	}, nil
}

// NewHTTPClient builds an HTTP client with the given mTLS settings.
func NewHTTPClient(mtls MTLSConfig) (*http.Client, error) {
	return httpClientWithMTLS(&mtls)
}
