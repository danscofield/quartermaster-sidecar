package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenAPIAndIndex(t *testing.T) {
	d := &Daemon{}
	ts := httptest.NewServer(d.handler())
	defer ts.Close()

	indexResp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer indexResp.Body.Close()
	if indexResp.StatusCode != http.StatusOK {
		t.Fatalf("index status %d", indexResp.StatusCode)
	}
	var index ServiceIndex
	if err := json.NewDecoder(indexResp.Body).Decode(&index); err != nil {
		t.Fatal(err)
	}
	if index.OpenAPIURL != "/openapi.json" {
		t.Fatalf("openapi_url = %q", index.OpenAPIURL)
	}
	if index.DocsURL != "/docs" {
		t.Fatalf("docs_url = %q", index.DocsURL)
	}

	specResp, err := http.Get(ts.URL + "/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	defer specResp.Body.Close()
	if specResp.StatusCode != http.StatusOK {
		t.Fatalf("openapi status %d", specResp.StatusCode)
	}
	if ct := specResp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q", ct)
	}
	var spec map[string]any
	if err := json.NewDecoder(specResp.Body).Decode(&spec); err != nil {
		t.Fatal(err)
	}
	if spec["openapi"] == nil {
		t.Fatal("missing openapi version field")
	}

	docsResp, err := http.Get(ts.URL + "/docs")
	if err != nil {
		t.Fatal(err)
	}
	defer docsResp.Body.Close()
	if docsResp.StatusCode != http.StatusOK {
		t.Fatalf("docs status %d", docsResp.StatusCode)
	}
	if ct := docsResp.Header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("docs content-type = %q", ct)
	}
}

func TestCredentialHTTPServer(t *testing.T) {
	d := &Daemon{
		available: []string{"payments"},
		credentials: map[string]billetCredential{
			"payments": {
				token:   "jwt-token",
				expires: time.Now().Add(time.Hour),
				certPEM: "-----BEGIN CERTIFICATE-----\ntest\n-----END CERTIFICATE-----\n",
				keyPEM:  "-----BEGIN EC PRIVATE KEY-----\ntest\n-----END EC PRIVATE KEY-----\n",
			},
		},
		manifest: map[string]ManifestBillet{
			"payments": {
				Name:      "payments",
				ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
				TokenPath: "/billets/payments/token",
				CertPath:  "/billets/payments/cert.pem",
				KeyPath:   "/billets/payments/key.pem",
			},
		},
	}

	ts := httptest.NewServer(d.handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("manifest status %d", resp.StatusCode)
	}

	var manifest struct {
		Available  []string `json:"available"`
		OpenAPIURL string   `json:"openapi_url"`
		DocsURL    string   `json:"docs_url"`
		Billets    map[string]struct {
			TokenPath string `json:"token_path"`
		} `json:"billets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Billets["payments"].TokenPath != "/billets/payments/token" {
		t.Fatalf("token_path = %q", manifest.Billets["payments"].TokenPath)
	}
	if manifest.OpenAPIURL != "/openapi.json" {
		t.Fatalf("openapi_url = %q", manifest.OpenAPIURL)
	}
	if manifest.DocsURL != "/docs" {
		t.Fatalf("docs_url = %q", manifest.DocsURL)
	}

	tokenResp, err := http.Get(ts.URL + "/billets/payments/token")
	if err != nil {
		t.Fatal(err)
	}
	defer tokenResp.Body.Close()
	if tokenResp.StatusCode != http.StatusOK {
		t.Fatalf("token status %d", tokenResp.StatusCode)
	}
}
