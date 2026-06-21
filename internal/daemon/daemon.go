package daemon

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dscof/qm-agent/internal/config"
	"github.com/dscof/qm-agent/internal/identity"
	"github.com/dscof/qm-agent/qmclient"
)

type billetCredential struct {
	token   string
	expires time.Time
	certPEM string
}

// Daemon exchanges workload identity for Quartermaster credentials on a refresh loop.
type Daemon struct {
	cfg    *config.Config
	id     identity.Source
	api    *qmclient.API
	logger *slog.Logger

	mu          sync.RWMutex
	credentials map[string]billetCredential
}

// New creates a daemon from configuration and an identity source.
func New(cfg *config.Config, id identity.Source, logger *slog.Logger) (*Daemon, error) {
	if logger == nil {
		logger = slog.Default()
	}

	tlsCfg, err := resolveTLS(context.Background(), cfg, id)
	if err != nil {
		return nil, err
	}

	httpClient := &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsCfg},
		Timeout:   60 * time.Second,
	}

	api, err := qmclient.New(qmclient.Config{
		BaseURL:    cfg.Quartermaster.URL,
		HTTPClient: httpClient,
	})
	if err != nil {
		return nil, err
	}

	return &Daemon{
		cfg:         cfg,
		id:          id,
		api:         api,
		logger:      logger,
		credentials: make(map[string]billetCredential),
	}, nil
}

func resolveTLS(ctx context.Context, cfg *config.Config, id identity.Source) (*tls.Config, error) {
	if override, err := id.TLSConfig(ctx); err != nil {
		return nil, err
	} else if override != nil {
		if cfg.Quartermaster.MTLS.CAFile != "" {
			return mergeRootCA(override, cfg.Quartermaster.MTLS.CAFile)
		}
		return override, nil
	}

	if cfg.Quartermaster.MTLS.CertFile == "" {
		return &tls.Config{MinVersion: tls.VersionTLS12}, nil
	}

	client, err := httpClientFromMTLS(cfg.Quartermaster.MTLS)
	if err != nil {
		return nil, err
	}
	return client.Transport.(*http.Transport).TLSClientConfig, nil
}

func httpClientFromMTLS(mtls config.MTLSConfig) (*http.Client, error) {
	return qmclient.NewHTTPClient(qmclient.MTLSConfig{
		CertFile: mtls.CertFile,
		KeyFile:  mtls.KeyFile,
		CAFile:   mtls.CAFile,
	})
}

func mergeRootCA(base *tls.Config, caFile string) (*tls.Config, error) {
	cfg := base.Clone()
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("parse CA file %s", caFile)
	}
	if cfg.RootCAs == nil {
		cfg.RootCAs = pool
	} else {
		cfg.RootCAs.AppendCertsFromPEM(caPEM)
	}
	return cfg, nil
}

// Run performs an initial exchange then refreshes until ctx is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	defer d.id.Close()

	if err := d.refresh(ctx); err != nil {
		return err
	}

	for {
		wait := d.timeUntilRefresh()
		d.logger.Info("scheduled refresh", "in", wait)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
			if err := d.refresh(ctx); err != nil {
				d.logger.Error("refresh failed", "error", err)
			}
		}
	}
}

func (d *Daemon) timeUntilRefresh() time.Duration {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.credentials) == 0 {
		return time.Minute
	}

	earliest := time.Time{}
	for _, cred := range d.credentials {
		if earliest.IsZero() || cred.expires.Before(earliest) {
			earliest = cred.expires
		}
	}

	wait := time.Until(earliest.Add(-d.cfg.Refresh.Margin))
	if wait < 30*time.Second {
		return 30 * time.Second
	}
	return wait
}

func (d *Daemon) refresh(ctx context.Context) error {
	cred, err := d.id.Credential(ctx)
	if err != nil {
		return fmt.Errorf("obtain identity: %w", err)
	}

	billets, err := d.resolveBillets(ctx, cred)
	if err != nil {
		return err
	}
	if len(billets) == 0 {
		d.logger.Info("no entitled billets")
		d.mu.Lock()
		d.credentials = make(map[string]billetCredential)
		d.mu.Unlock()
		if err := writeManifest(d.cfg.Output.Dir, map[string]ManifestBillet{}); err != nil {
			return err
		}
		return pruneBilletDirs(d.cfg.Output.Dir, map[string]struct{}{})
	}

	d.logger.Info("refreshing billets", "count", len(billets), "discovered", len(d.cfg.Exchange.Billets) == 0)

	manifest := make(map[string]ManifestBillet, len(billets))
	active := make(map[string]struct{}, len(billets))
	updated := make(map[string]billetCredential, len(billets))
	var refreshErr error

	for _, name := range billets {
		active[name] = struct{}{}

		resp, err := d.exchangeBillet(ctx, cred, name)
		if err != nil {
			refreshErr = errors.Join(refreshErr, fmt.Errorf("%s: %w", name, err))
			continue
		}

		paths := billetPaths(d.cfg.Output.Dir, name)
		expires := time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)

		if err := writeFileAtomic(paths.TokenFile, []byte(resp.AccessToken), 0o600); err != nil {
			refreshErr = errors.Join(refreshErr, fmt.Errorf("%s: write token: %w", name, err))
			continue
		}

		entry := ManifestBillet{
			Name:      name,
			ExpiresAt: expires.UTC().Format(time.RFC3339),
			TokenPath: paths.TokenFile,
		}

		certPEM := ""
		if resp.CertificateChain != nil {
			certPEM = *resp.CertificateChain
		}
		if d.cfg.CSR.Enabled && certPEM != "" {
			if err := writeFileAtomic(paths.CertFile, []byte(certPEM), 0o644); err != nil {
				refreshErr = errors.Join(refreshErr, fmt.Errorf("%s: write cert: %w", name, err))
				continue
			}
			entry.CertPath = paths.CertFile
			entry.KeyPath = paths.KeyFile
		}

		manifest[name] = entry
		updated[name] = billetCredential{
			token:   resp.AccessToken,
			expires: expires,
			certPEM: certPEM,
		}

		d.logger.Info("exchanged credentials",
			"billet", name,
			"expires_at", entry.ExpiresAt,
			"has_certificate", certPEM != "",
		)
	}

	d.mu.Lock()
	d.credentials = updated
	d.mu.Unlock()

	if err := writeManifest(d.cfg.Output.Dir, manifest); err != nil {
		return err
	}
	if err := pruneBilletDirs(d.cfg.Output.Dir, active); err != nil {
		return err
	}
	return refreshErr
}

func (d *Daemon) exchangeBillet(ctx context.Context, cred *identity.Credential, billetName string) (*qmclient.TokenExchangeResponse, error) {
	form := qmclient.TokenExchangeForm{
		GrantType: strPtr(d.cfg.Exchange.GrantType),
		Billets:   strPtr(billetName),
	}
	if d.cfg.Exchange.Audience != "" {
		form.Audience = &d.cfg.Exchange.Audience
	}

	if cred != nil && cred.SubjectToken != "" {
		form.SubjectToken = &cred.SubjectToken
		tokenType := cred.SubjectTokenType
		if d.cfg.Exchange.SubjectTokenType != "" {
			tokenType = d.cfg.Exchange.SubjectTokenType
		}
		form.SubjectTokenType = &tokenType
	}

	if d.cfg.CSR.Enabled {
		paths := billetPaths(d.cfg.Output.Dir, billetName)
		csrPEM, err := d.loadOrCreateCSR(paths.KeyFile, billetName)
		if err != nil {
			return nil, err
		}
		form.Csr = &csrPEM
	}

	resp, err := d.api.ExchangeToken(ctx, form)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (d *Daemon) loadOrCreateCSR(keyPath, billetName string) (string, error) {
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		keyPEM, err = generateKeyPEM(keyPath)
		if err != nil {
			return "", err
		}
	}

	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return "", fmt.Errorf("decode private key from %s", keyPath)
	}

	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}

	tmpl := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: billetName},
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
	if err != nil {
		return "", err
	}
	// Quartermaster expects base64-encoded CSR DER in the token exchange form.
	return base64.StdEncoding.EncodeToString(der), nil
}

func generateKeyPEM(path string) ([]byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})
	if err := writeFileAtomic(path, pemBytes, 0o600); err != nil {
		return nil, err
	}
	return pemBytes, nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func strPtr(s string) *string { return &s }

// Credentials returns a snapshot of current billet credentials.
func (d *Daemon) Credentials() map[string]billetCredential {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make(map[string]billetCredential, len(d.credentials))
	for k, v := range d.credentials {
		out[k] = v
	}
	return out
}
