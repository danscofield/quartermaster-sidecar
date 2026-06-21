# qm-agent

Go client library and daemon for [Quartermaster](https://github.com/dscof/qm-agent) — a workload identity federation broker.

The daemon (`qm-agentd`) runs on a workload, obtains identity from the local environment, exchanges it with Quartermaster for short-lived JWTs and optional certificates, and serves credentials over HTTP.

## Layout

```
openapi.json          # Quartermaster broker API (client codegen)
agent.openapi.json    # qm-agentd credential API (embedded, served at GET /openapi.json)
qmclient/             # Generated + hand-written Go client
internal/
  config/             # Daemon YAML configuration
  identity/           # Pluggable workload identity sources
  daemon/             # Token exchange refresh loop + HTTP API
cmd/qm-agentd/        # Daemon entrypoint
config.example.yaml   # Example configuration
```

## Identity sources

| Source | Mechanism |
|--------|-----------|
| `spire` | SPIRE Workload API — X.509 SVID via mTLS (`mode: mtls`) or JWT SVID as `subject_token` (`mode: jwt`) |
| `aws` | Presigned `sts:GetCallerIdentity` URL (`urn:quartermaster:token-type:aws-presigned-sts`) |
| `gcp` | GCE metadata identity token (`urn:quartermaster:token-type:gcp-identity`) |

Quartermaster accepts identity via `subject_token` **or** a SPIFFE client certificate. Client mTLS to Quartermaster is only required for SPIRE `mtls` mode (the X.509 SVID is the client cert). For `aws`, `gcp`, and SPIRE `jwt`, identity is sent as `subject_token`; `quartermaster.mtls` is optional and typically only needs `ca_file` to verify the server.

## Credential HTTP API

The daemon discovers entitled billets via `POST /billets/me` (or uses `exchange.billets` when set). By default it also performs **one token exchange per billet** (`exchange.auto_per_billet: true`). Set `auto_per_billet: false` to discover only and let workloads request **combined** credentials for a subset via subscriptions.

**Self-documenting:** `GET /` returns service links; `GET /openapi.json` serves the OpenAPI 3.1 spec; `GET /docs` is Swagger UI. The manifest includes `openapi_url` and `docs_url`.

| Endpoint | Content |
|----------|---------|
| `GET /` | Service index with links to manifest, OpenAPI, and docs |
| `GET /openapi.json` | OpenAPI 3.1 specification for this API |
| `GET /docs` | Swagger UI (loads `/openapi.json`) |
| `GET /manifest.json` | Available billets, per-billet credentials (if auto), and subscriptions |
| `GET /billets/{name}/token` | Per-billet JWT (`auto_per_billet`) |
| `GET /billets/{name}/cert.pem` | Per-billet cert chain (`csr.enabled`) |
| `GET /billets/{name}/key.pem` | Per-billet EC private key (`csr.enabled`) |
| `POST /subscriptions` | Subscribe to a billet list; exchanges when all are available |
| `GET /subscriptions` | List subscriptions |
| `GET /subscriptions/{id}` | Subscription status |
| `DELETE /subscriptions/{id}` | Remove a subscription |
| `GET /subscriptions/{id}/token` | Combined JWT for the subscribed billets |
| `GET /subscriptions/{id}/cert.pem` | Combined cert (`csr.enabled`) |
| `GET /subscriptions/{id}/key.pem` | Combined key (`csr.enabled`) |

Default listen address: `127.0.0.1:8765` (configure with `server.listen`).

```bash
curl -s http://127.0.0.1:8765/manifest.json | jq .

# Combined credentials for payments + analytics (one QM /token call):
curl -s -X POST http://127.0.0.1:8765/subscriptions \
  -H 'Content-Type: application/json' \
  -d '{"billets":["payments","analytics"]}' | jq .
curl -s http://127.0.0.1:8765/subscriptions/analytics__payments/token
```

Refresh is scheduled from the **earliest** token expiry across per-billet and subscription credentials.

## Quick start

```bash
cp config.example.yaml config.yaml
# edit config.yaml for your environment

go run ./cmd/qm-agentd -config config.yaml
```

## Configuration

See [`config.example.yaml`](config.example.yaml) for all options.

```yaml
quartermaster:
  url: https://quartermaster.example.com
  mtls:
    ca_file: /etc/qm-agent/ca.pem

identity:
  type: aws
  aws:
    region: us-east-1

exchange:
  billets: []
  audience: https://quartermaster.example.com

server:
  listen: 127.0.0.1:8765
```

Environment:

| Variable | Description |
|----------|-------------|
| `QM_AGENT_CONFIG` | Config file path (default `config.yaml`) |

## Client library

```go
api, err := qmclient.New(qmclient.Config{
    BaseURL: "https://quartermaster.example.com",
    MTLS: &qmclient.MTLSConfig{
        CertFile: "client.pem",
        KeyFile:  "client-key.pem",
        CAFile:   "ca.pem",
    },
})

resp, err := api.ExchangeToken(ctx, qmclient.TokenExchangeForm{
    GrantType:        ptr("urn:ietf:params:oauth:grant-type:token-exchange"),
    SubjectToken:     &subjectToken,
    SubjectTokenType: ptr("urn:ietf:params:oauth:token-type:jwt"),
})
```

## Regenerate client

```bash
make generate
```

The prepare script adapts `openapi.json` for oapi-codegen (OpenAPI 3.0, unique operation IDs).
