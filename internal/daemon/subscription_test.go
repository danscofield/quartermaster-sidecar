package daemon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSubscriptionAPI(t *testing.T) {
	d := &Daemon{
		available: []string{"payments", "analytics", "other"},
		subscriptions: map[string]*subscription{
			"analytics__payments": {
				billets: []string{"analytics", "payments"},
				status:  "ready",
				cred: billetCredential{
					token:   "combined-jwt",
					expires: time.Now().Add(time.Hour),
				},
			},
			"other__payments": {
				billets: []string{"other", "payments"},
				status:  "pending",
				reason:  "waiting for billet(s): missing",
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
		Available     []string                       `json:"available"`
		Subscriptions map[string]ManifestSubscription `json:"subscriptions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Available) != 3 {
		t.Fatalf("available = %v", manifest.Available)
	}
	if manifest.Subscriptions["analytics__payments"].Status != "ready" {
		t.Fatalf("subscription status = %q", manifest.Subscriptions["analytics__payments"].Status)
	}

	tokenResp, err := http.Get(ts.URL + "/subscriptions/analytics__payments/token")
	if err != nil {
		t.Fatal(err)
	}
	defer tokenResp.Body.Close()
	if tokenResp.StatusCode != http.StatusOK {
		t.Fatalf("token status %d", tokenResp.StatusCode)
	}

	body, _ := json.Marshal(createSubscriptionRequest{Billets: []string{"payments", "analytics"}})
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
	if subscriptionID(got) != "a__b" {
		t.Fatalf("id = %q", subscriptionID(got))
	}
}
