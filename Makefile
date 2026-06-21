.PHONY: generate build test

generate:
	bash scripts/prepare-openapi.sh
	oapi-codegen -config oapi-codegen.yaml openapi.codegen.json

build:
	go build -o bin/qm-agentd ./cmd/qm-agentd

test:
	go test ./...
