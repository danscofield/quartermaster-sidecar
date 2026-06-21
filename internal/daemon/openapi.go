package daemon

import _ "embed"

// OpenAPI document for the qm-agentd credential HTTP API (also served at GET /openapi.json).
//
//go:embed agent.openapi.json
var openAPISpec []byte

// Swagger UI page served at GET /docs.
//
//go:embed docs.html
var docsHTML []byte

const (
	openAPIPath    = "/openapi.json"
	docsPath       = "/docs"
	manifestPath   = "/manifest.json"
	serviceName    = "qm-agentd"
	serviceDesc    = "Quartermaster workload credential sidecar"
)

type ServiceIndex struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	ManifestURL  string `json:"manifest_url"`
	OpenAPIURL   string `json:"openapi_url"`
	DocsURL      string `json:"docs_url"`
}
