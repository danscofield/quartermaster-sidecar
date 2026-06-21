.PHONY: generate build test sync-agent-openapi

generate:
	bash scripts/prepare-openapi.sh
	oapi-codegen -config oapi-codegen.yaml openapi.codegen.json

sync-agent-openapi:
	cp internal/daemon/agent.openapi.json agent.openapi.json

build:
	go build -o bin/qm-agentd ./cmd/qm-agentd

test:
	go test ./...
