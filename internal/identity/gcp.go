package identity

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dscof/qm-agent/internal/config"
)

type gcpSource struct {
	cfg    *config.Config
	client *http.Client
}

func newGCP(cfg *config.Config) (*gcpSource, error) {
	return &gcpSource{cfg: cfg, client: &http.Client{Timeout: 30 * time.Second}}, nil
}

func (s *gcpSource) Credential(ctx context.Context) (*Credential, error) {
	host := s.cfg.Identity.GCP.MetadataHost
	audience := s.cfg.Identity.GCP.Audience
	tokenType := s.cfg.Identity.GCP.SubjectTokenType

	url := fmt.Sprintf("http://%s/computeMetadata/v1/instance/service-accounts/default/identity?audience=%s&format=full",
		host, audience)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Metadata-Flavor", "Google")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch gcp identity token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gcp metadata server returned %d: %s", resp.StatusCode, string(body))
	}

	return &Credential{
		SubjectToken:     string(body),
		SubjectTokenType: tokenType,
	}, nil
}

func (s *gcpSource) TLSConfig(ctx context.Context) (*tls.Config, error) {
	return tlsConfigFromFiles(s.cfg.Quartermaster.MTLS)
}

func (s *gcpSource) Close() error {
	if s.client != nil {
		s.client.CloseIdleConnections()
	}
	return nil
}
