// Package apiembed contains embedded OpenAPI specification and OpenAPI HTML docs.
package apiembed

import (
	"embed"
)

var (
	// OpenAPISpec is an embedded OpenAPI v3.1 specification folder.
	//go:embed openapi/*
	OpenAPISpec embed.FS
	// OpenAPIDocsHTML is an embedded HTML index.html stoplight docs.
	//go:embed index.html
	OpenAPIDocsHTML []byte
)
