package daemon

import (
	"encoding/json"
	"net/http"
)

func (d *Daemon) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", d.handleIndex)
	mux.HandleFunc("GET /openapi.json", d.handleOpenAPI)
	mux.HandleFunc("GET /docs", d.handleDocs)
	mux.HandleFunc("GET /docs/", d.handleDocs)
	mux.HandleFunc("GET /manifest.json", d.handleManifest)
	mux.HandleFunc("GET /billets/{name}/token", d.handleBilletToken)
	mux.HandleFunc("GET /billets/{name}/cert.pem", d.handleBilletCert)
	mux.HandleFunc("GET /billets/{name}/key.pem", d.handleBilletKey)
	mux.HandleFunc("GET /subscriptions", d.handleListSubscriptions)
	mux.HandleFunc("POST /subscriptions", d.handleCreateSubscription)
	mux.HandleFunc("GET /subscriptions/{id}", d.handleGetSubscription)
	mux.HandleFunc("DELETE /subscriptions/{id}", d.handleDeleteSubscription)
	mux.HandleFunc("GET /subscriptions/{id}/token", d.handleSubscriptionToken)
	mux.HandleFunc("GET /subscriptions/{id}/cert.pem", d.handleSubscriptionCert)
	mux.HandleFunc("GET /subscriptions/{id}/key.pem", d.handleSubscriptionKey)
	return mux
}

func (d *Daemon) handleIndex(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, ServiceIndex{
		Name:        serviceName,
		Description: serviceDesc,
		ManifestURL: manifestPath,
		OpenAPIURL:  openAPIPath,
		DocsURL:     docsPath,
	})
}

func (d *Daemon) handleDocs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(docsHTML)
}

func (d *Daemon) handleOpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write(openAPISpec)
}

func (d *Daemon) handleManifest(w http.ResponseWriter, _ *http.Request) {
	d.mu.RLock()
	data, err := manifestJSON(d.available, d.manifest, d.subscriptionManifestsLocked())
	d.mu.RUnlock()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (d *Daemon) subscriptionManifestsLocked() map[string]ManifestSubscription {
	subs := make(map[string]ManifestSubscription, len(d.subscriptions))
	for id, sub := range d.subscriptions {
		subs[id] = d.manifestSubscription(id, sub)
	}
	return subs
}

func (d *Daemon) manifestSubscription(id string, sub *subscription) ManifestSubscription {
	base := subscriptionBasePath(id)
	entry := ManifestSubscription{
		Billets:   append([]string(nil), sub.billets...),
		Status:    sub.status,
		TokenPath: base + "/token",
	}
	if sub.reason != "" {
		entry.PendingReason = sub.reason
	}
	if sub.status == "ready" {
		entry.ExpiresAt = sub.cred.expires.UTC().Format("2006-01-02T15:04:05Z07:00")
		if sub.cred.certPEM != "" {
			entry.CertPath = base + "/cert.pem"
			entry.KeyPath = base + "/key.pem"
		}
	}
	return entry
}

func (d *Daemon) findBillet(urlName string) (string, billetCredential, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for name, cred := range d.credentials {
		if sanitizeBilletName(name) == urlName {
			return name, cred, true
		}
	}
	return "", billetCredential{}, false
}

func (d *Daemon) findSubscription(urlID string) (string, *subscription, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	for id, sub := range d.subscriptions {
		if sanitizeBilletName(id) == urlID {
			return id, sub, true
		}
	}
	return "", nil, false
}

func (d *Daemon) handleBilletToken(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	_, cred, ok := d.findBillet(name)
	if !ok || cred.token == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write([]byte(cred.token))
}

func (d *Daemon) handleBilletCert(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	_, cred, ok := d.findBillet(name)
	if !ok || cred.certPEM == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Write([]byte(cred.certPEM))
}

func (d *Daemon) handleBilletKey(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	_, cred, ok := d.findBillet(name)
	if !ok || cred.keyPEM == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Write([]byte(cred.keyPEM))
}

type createSubscriptionRequest struct {
	Billets []string `json:"billets"`
}

func (d *Daemon) handleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	var req createSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	billets, err := normalizeBillets(req.Billets)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id := subscriptionID(billets)
	sub := &subscription{
		billets: billets,
		status:  "pending",
		reason:  "waiting for discovery",
	}

	d.mu.Lock()
	if existing, ok := d.subscriptions[id]; ok {
		entry := d.manifestSubscription(id, existing)
		d.mu.Unlock()
		writeJSON(w, http.StatusOK, entryWithID(id, entry))
		d.triggerRefresh()
		return
	}
	d.subscriptions[id] = sub
	entry := d.manifestSubscription(id, sub)
	d.mu.Unlock()

	writeJSON(w, http.StatusCreated, entryWithID(id, entry))
	d.triggerRefresh()
}

type subscriptionResponse struct {
	ID string `json:"id"`
	ManifestSubscription
}

func entryWithID(id string, entry ManifestSubscription) subscriptionResponse {
	return subscriptionResponse{ID: id, ManifestSubscription: entry}
}

func (d *Daemon) handleListSubscriptions(w http.ResponseWriter, _ *http.Request) {
	d.mu.RLock()
	out := make([]subscriptionResponse, 0, len(d.subscriptions))
	for id, sub := range d.subscriptions {
		out = append(out, entryWithID(id, d.manifestSubscription(id, sub)))
	}
	d.mu.RUnlock()
	writeJSON(w, http.StatusOK, out)
}

func (d *Daemon) handleGetSubscription(w http.ResponseWriter, r *http.Request) {
	urlID := r.PathValue("id")
	id, sub, ok := d.findSubscription(urlID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	d.mu.RLock()
	entry := d.manifestSubscription(id, sub)
	d.mu.RUnlock()
	writeJSON(w, http.StatusOK, entryWithID(id, entry))
}

func (d *Daemon) handleDeleteSubscription(w http.ResponseWriter, r *http.Request) {
	urlID := r.PathValue("id")
	d.mu.Lock()
	var deleted bool
	for id := range d.subscriptions {
		if sanitizeBilletName(id) == urlID {
			delete(d.subscriptions, id)
			delete(d.keys, id)
			deleted = true
			break
		}
	}
	d.mu.Unlock()
	if !deleted {
		http.NotFound(w, r)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (d *Daemon) handleSubscriptionToken(w http.ResponseWriter, r *http.Request) {
	urlID := r.PathValue("id")
	_, sub, ok := d.findSubscription(urlID)
	if !ok || sub.status != "ready" || sub.cred.token == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write([]byte(sub.cred.token))
}

func (d *Daemon) handleSubscriptionCert(w http.ResponseWriter, r *http.Request) {
	urlID := r.PathValue("id")
	_, sub, ok := d.findSubscription(urlID)
	if !ok || sub.status != "ready" || sub.cred.certPEM == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Write([]byte(sub.cred.certPEM))
}

func (d *Daemon) handleSubscriptionKey(w http.ResponseWriter, r *http.Request) {
	urlID := r.PathValue("id")
	_, sub, ok := d.findSubscription(urlID)
	if !ok || sub.status != "ready" || sub.cred.keyPEM == "" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/x-pem-file")
	w.Write([]byte(sub.cred.keyPEM))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
