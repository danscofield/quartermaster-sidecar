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
	"sync"
	"time"

	"github.com/dscof/qm-agent/internal/config"
	"github.com/dscof/qm-agent/internal/identity"
	"github.com/dscof/qm-agent/qmclient"
)

// Daemon exchanges workload identity for Quartermaster credentials on a refresh loop
// and serves them over HTTP.
type Daemon struct {
	cfg    *config.Config
	id     identity.Source
	api    *qmclient.API
	logger *slog.Logger

	mu          sync.RWMutex
	credentials map[string]billetCredential
	manifest    map[string]ManifestBillet
	keys        map[string][]byte // billet name -> EC private key PEM (CSR)
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
		manifest:    make(map[string]ManifestBillet),
		keys:        make(map[string][]byte),
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

	tlsCfg, err := identity.QuartermasterTLS(cfg.Quartermaster.MTLS)
	if err != nil {
		return nil, err
	}
	if tlsCfg != nil {
		return tlsCfg, nil
	}
	return &tls.Config{MinVersion: tls.VersionTLS12}, nil
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

// Run starts the credential HTTP server and the refresh loop until ctx is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	defer d.id.Close()

	srv := &http.Server{
		Addr:    d.cfg.Server.Listen,
		Handler: d.handler(),
	}

	go func() {
		d.logger.Info("credential server listening", "addr", d.cfg.Server.Listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			d.logger.Error("credential server", "error", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

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
		d.manifest = make(map[string]ManifestBillet)
		d.mu.Unlock()
		return nil
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

		expires := time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second)
		base := billetBasePath(name)

		entry := ManifestBillet{
			Name:      name,
			ExpiresAt: expires.UTC().Format(time.RFC3339),
			TokenPath: base + "/token",
		}

		certPEM := ""
		if resp.CertificateChain != nil {
			certPEM = *resp.CertificateChain
		}

		keyPEM := ""
		if d.cfg.CSR.Enabled {
			if k, ok := d.keyPEM(name); ok {
				keyPEM = string(k)
			}
			if certPEM != "" {
				entry.CertPath = base + "/cert.pem"
				entry.KeyPath = base + "/key.pem"
			}
		}

		manifest[name] = entry
		updated[name] = billetCredential{
			token:   resp.AccessToken,
			expires: expires,
			certPEM: certPEM,
			keyPEM:  keyPEM,
		}

		d.logger.Info("exchanged credentials",
			"billet", name,
			"expires_at", entry.ExpiresAt,
			"has_certificate", certPEM != "",
		)
	}

	d.mu.Lock()
	d.credentials = updated
	d.manifest = manifest
	// Drop keys for billets no longer entitled.
	for name := range d.keys {
		if _, ok := active[name]; !ok {
			delete(d.keys, name)
		}
	}
	d.mu.Unlock()

	return refreshErr
}

func (d *Daemon) keyPEM(billetName string) ([]byte, bool) {
	d.mu.RLock()
	k, ok := d.keys[billetName]
	d.mu.RUnlock()
	if ok {
		return k, true
	}

	k, err := generateKeyPEM()
	if err != nil {
		return nil, false
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if existing, ok := d.keys[billetName]; ok {
		return existing, true
	}
	d.keys[billetName] = k
	return k, true
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
		csrB64, err := d.createCSR(billetName)
		if err != nil {
			return nil, err
		}
		form.Csr = &csrB64
	}

	resp, err := d.api.ExchangeToken(ctx, form)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (d *Daemon) createCSR(billetName string) (string, error) {
	keyPEM, ok := d.keyPEM(billetName)
	if !ok {
		return "", fmt.Errorf("generate key for %s", billetName)
	}

	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return "", fmt.Errorf("decode private key for %s", billetName)
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
	return base64.StdEncoding.EncodeToString(der), nil
}

func generateKeyPEM() ([]byte, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}), nil
}

func strPtr(s string) *string { return &s }
