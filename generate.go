//go:build ignore

package main

//go:generate bash scripts/prepare-openapi.sh
//go:generate oapi-codegen -config oapi-codegen.yaml openapi.codegen.json
