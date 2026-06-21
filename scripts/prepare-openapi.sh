#!/usr/bin/env bash
# Produces openapi.codegen.json: OpenAPI 3.0 + unique operationIds for oapi-codegen.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
jq '
  .openapi = "3.0.3" |
  .paths["/admin/billets/{name}"].get.operationId = "get_admin_billet" |
  .paths["/billets/{name}"].get.operationId = "get_billet_metadata" |
  .components.schemas.BilletDiscoveryForm.properties.subject_token = {"type": "string", "nullable": true} |
  .components.schemas.BilletDiscoveryForm.properties.subject_token_type = {"type": "string", "nullable": true} |
  .components.schemas.TokenExchangeForm.properties.audience = {"type": "string", "nullable": true} |
  .components.schemas.TokenExchangeForm.properties.billets = {"type": "string", "nullable": true} |
  .components.schemas.TokenExchangeForm.properties.csr = {"type": "string", "nullable": true} |
  .components.schemas.TokenExchangeForm.properties.grant_type = {"type": "string", "nullable": true} |
  .components.schemas.TokenExchangeForm.properties.subject_token = {"type": "string", "nullable": true} |
  .components.schemas.TokenExchangeForm.properties.subject_token_type = {"type": "string", "nullable": true} |
  .components.schemas.TokenExchangeResponse.properties.certificate_chain = {"type": "string", "nullable": true} |
  .components.schemas.UpdateBilletRequest.properties.associated_aws_roles = {"type": "array", "nullable": true, "items": {"type": "string"}} |
  .components.schemas.UpdateBilletRequest.properties.associated_gcp_sas = {"type": "array", "nullable": true, "items": {"type": "string"}} |
  .components.schemas.UpdateBilletRequest.properties.description = {"type": "string", "nullable": true} |
  .components.schemas.UpdateBilletRequest.properties.tags = {"type": "array", "nullable": true, "items": {"type": "string"}}
' "$ROOT/openapi.json" > "$ROOT/openapi.codegen.json"
