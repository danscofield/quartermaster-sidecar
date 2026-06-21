package daemon

import (
	"encoding/json"
	"time"
)

// Manifest is served at GET /manifest.json.
type Manifest struct {
	UpdatedAt     string                          `json:"updated_at"`
	Available     []string                        `json:"available"`
	OpenAPIURL    string                          `json:"openapi_url"`
	DocsURL       string                          `json:"docs_url"`
	Billets       map[string]ManifestBillet       `json:"billets,omitempty"`
	Subscriptions map[string]ManifestSubscription `json:"subscriptions,omitempty"`
}

type ManifestBillet struct {
	Name      string `json:"name"`
	ExpiresAt string `json:"expires_at"`
	TokenPath string `json:"token_path"`
	CertPath  string `json:"cert_path,omitempty"`
	KeyPath   string `json:"key_path,omitempty"`
}

type ManifestSubscription struct {
	Billets       []string `json:"billets"`
	Audience      string   `json:"audience"`
	Status        string   `json:"status"` // pending, ready
	PendingReason string   `json:"pending_reason,omitempty"`
	ExpiresAt     string   `json:"expires_at,omitempty"`
	TokenPath     string   `json:"token_path,omitempty"`
	CertPath      string   `json:"cert_path,omitempty"`
	KeyPath       string   `json:"key_path,omitempty"`
}

type billetCredential struct {
	token   string
	expires time.Time
	certPEM string
	keyPEM  string
}

type subscription struct {
	billets  []string
	audience string
	status   string
	reason   string
	cred     billetCredential
}

func buildManifest(available []string, billets map[string]ManifestBillet, subs map[string]ManifestSubscription) Manifest {
	return Manifest{
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
		Available:     available,
		OpenAPIURL:    openAPIPath,
		DocsURL:       docsPath,
		Billets:       billets,
		Subscriptions: subs,
	}
}

func manifestJSON(available []string, billets map[string]ManifestBillet, subs map[string]ManifestSubscription) ([]byte, error) {
	return json.MarshalIndent(buildManifest(available, billets, subs), "", "  ")
}
