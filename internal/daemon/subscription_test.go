package daemon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dscof/qm-agent/internal/config"
)

func TestSubscriptionAPI(t *testing.T) {
	audience := "https://api.example.com"
	subID := subscriptionID([]string{"analytics", "payments"}, audience)

	d := &Daemon{
		cfg: &config.Config{
			Exchange: config.ExchangeConfig{Audience: "http://localhost:8443"},
		},
		available: []string{"payments", "analytics", "other"},
		subscriptions: map[string]*subscription{
			subID: {
				billets:  []string{"analytics", "payments"},
				audience: audience,
				status:   "ready",
				cred: billetCredential{
					token:   "combined-jwt",
					expires: time.Now().Add(time.Hour),
				},
			},
			subscriptionID([]string{"other", "payments"}, audience): {
				billets:  []string{"other", "payments"},
				audience: audience,
				status:   "pending",
				reason:   "waiting for billet(s): missing",
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
		Available     []string                        `json:"available"`
		Subscriptions map[string]ManifestSubscription `json:"subscriptions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Available) != 3 {
		t.Fatalf("available = %v", manifest.Available)
	}
	if manifest.Subscriptions[subID].Status != "ready" {
		t.Fatalf("subscription status = %q", manifest.Subscriptions[subID].Status)
	}
	if manifest.Subscriptions[subID].Audience != audience {
		t.Fatalf("audience = %q", manifest.Subscriptions[subID].Audience)
	}

	tokenResp, err := http.Get(ts.URL + "/subscriptions/" + subID + "/token")
	if err != nil {
		t.Fatal(err)
	}
	defer tokenResp.Body.Close()
	if tokenResp.StatusCode != http.StatusOK {
		t.Fatalf("token status %d", tokenResp.StatusCode)
	}

	body, _ := json.Marshal(createSubscriptionRequest{
		Billets:  []string{"payments", "analytics"},
		Audience: audience,
	})
	createResp, err := http.Post(ts.URL+"/subscriptions", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusOK {
		t.Fatalf("create existing status %d", createResp.StatusCode)
	}
}

func TestNormalizeBillets(t *testing.T) {
	got, err := normalizeBillets([]string{"b", "a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("normalize = %v", got)
	}
	if subscriptionID(got, "https://api.example.com") != "a__b__aud__https_api.example.com" {
		t.Fatalf("id = %q", subscriptionID(got, "https://api.example.com"))
	}
}

func TestResolveAudience(t *testing.T) {
	d := &Daemon{cfg: &config.Config{Exchange: config.ExchangeConfig{Audience: "http://default"}}}

	got, err := d.resolveAudience("")
	if err != nil || got != "http://default" {
		t.Fatalf("default audience = %q, err = %v", got, err)
	}

	got, err = d.resolveAudience("https://custom")
	if err != nil || got != "https://custom" {
		t.Fatalf("custom audience = %q, err = %v", got, err)
	}
}
