#!/usr/bin/env bash
# Generate a dev CA + server + client cert chain for local Quartermaster mTLS.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="${1:-$ROOT/data/certs}"
DAYS=825
CN_CA="qm-dev-ca"
CN_SERVER="localhost"
CN_CLIENT="qm-agentd"

mkdir -p "$OUT"
cd "$OUT"

if [[ -f ca.pem ]]; then
  echo "CA already exists at $OUT/ca.pem — remove it first to regenerate" >&2
  exit 1
fi

# 1. CA
openssl genrsa -out ca-key.pem 4096
openssl req -x509 -new -nodes -key ca-key.pem -sha256 -days "$DAYS" \
  -subj "/CN=$CN_CA" -out ca.pem

extfile() {
  local cn="$1" dns="$2"
  cat >"$3" <<EOF
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth, clientAuth
subjectAltName = DNS:$dns, DNS:localhost, IP:127.0.0.1
EOF
}

sign() {
  local name="$1" cn="$2" dns="$3"
  openssl genrsa -out "${name}-key.pem" 2048
  openssl req -new -key "${name}-key.pem" -subj "/CN=$cn" -out "${name}.csr"
  extfile "$cn" "$dns" "${name}.ext"
  openssl x509 -req -in "${name}.csr" -CA ca.pem -CAkey ca-key.pem -CAcreateserial \
    -out "${name}.pem" -days "$DAYS" -sha256 -extfile "${name}.ext"
  rm -f "${name}.csr" "${name}.ext"
}

# 2. Server (Quartermaster HTTPS listener)
sign server "$CN_SERVER" "localhost"

# 3. Client (qm-agentd → Quartermaster)
sign client "$CN_CLIENT" "qm-agentd"

chmod 600 *-key.pem ca-key.pem
chmod 644 ca.pem server.pem client.pem

cat <<EOF

Generated dev mTLS material in $OUT:

  ca.pem, ca-key.pem          — trust anchor
  server.pem, server-key.pem  — configure Quartermaster TLS listener
  client.pem, client-key.pem  — configure qm-agentd quartermaster.mtls

qm-agentd config:

  quartermaster:
    mtls:
      ca_file: $OUT/ca.pem
      cert_file: $OUT/client.pem
      key_file: $OUT/client-key.pem

Configure Quartermaster to:
  - serve HTTPS with server.pem / server-key.pem
  - trust client certs signed by ca.pem

EOF
