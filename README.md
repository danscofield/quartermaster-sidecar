# qm-agent

Go client library and daemon for [Quartermaster](https://github.com/dscof/qm-agent) — a workload identity federation broker.

The daemon (`qm-agentd`) runs on a workload, obtains identity from the local environment, and exchanges it with Quartermaster for short-lived JWTs and optional certificates.

## Layout

```
openapi.json          # Source OpenAPI 3.1 spec
qmclient/             # Generated + hand-written Go client
internal/
  config/             # Daemon YAML configuration
  identity/           # Pluggable workload identity sources
  daemon/             # Token exchange refresh loop
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

## Billet discovery and output layout

When `exchange.billets` is **empty**, the daemon calls `POST /billets/me`, unions `billets`, `implicit_billets`, and `cedar_billets`, then performs **one token exchange per billet**.

When `exchange.billets` is **set**, it skips discovery and exchanges only for those names (still one token per billet).

Credentials are written under `output.dir` using a per-billet directory layout:

```
/var/run/qm-agent/
  manifest.json
  billets/
    payments/
      token
      key.pem      # when csr.enabled
      cert.pem     # when csr.enabled
    analytics/
      token
      ...
```

`manifest.json` lists each active billet with expiry and file paths so consumers do not need to scan directories. Stale billet directories are removed when a billet is no longer entitled.

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
    cert_file: /etc/qm-agent/client.pem
    key_file: /etc/qm-agent/client-key.pem
    ca_file: /etc/qm-agent/ca.pem

identity:
  type: aws
  aws:
    region: us-east-1

exchange:
  billets: []   # empty = discover all entitled billets

output:
  dir: /var/run/qm-agent
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
