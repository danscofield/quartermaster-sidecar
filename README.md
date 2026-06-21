# qm-agent

Go client library and daemon for [Quartermaster](https://github.com/dscof/qm-agent) — a workload identity federation broker.

The daemon (`qm-agentd`) runs on a workload, obtains identity from the local environment, exchanges it with Quartermaster for short-lived JWTs and optional certificates, and serves credentials over HTTP.

## Layout

```
openapi.json          # Source OpenAPI 3.1 spec
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

When `exchange.billets` is **empty**, the daemon calls `POST /billets/me`, unions entitled billets, then performs **one token exchange per billet**. Credentials are held in memory and exposed at:

| Endpoint | Content |
|----------|---------|
| `GET /manifest.json` | Index of billets with paths and expiry |
| `GET /billets/{name}/token` | Quartermaster access token (JWT) |
| `GET /billets/{name}/cert.pem` | Issued certificate chain (`csr.enabled`) |
| `GET /billets/{name}/key.pem` | EC private key for the cert (`csr.enabled`) |

Default listen address: `127.0.0.1:8765` (configure with `server.listen`).

```bash
curl -s http://127.0.0.1:8765/manifest.json | jq .
curl -s http://127.0.0.1:8765/billets/payments/token
```

Refresh is scheduled from the **earliest** token expiry across all billets.

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
