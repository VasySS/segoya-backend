// Package apiembed contains embedded OpenAPI specification and OpenAPI HTML docs.
package apiembed

import _ "embed"

var (
	// OpenAPISpec is an embedded OpenAPI v3.1 yaml specification.
	//go:embed openapi.yaml
	OpenAPISpec []byte
	// OpenAPIDocsHTML is an embedded HTML index.html stoplight docs.
	//go:embed index.html
	OpenAPIDocsHTML []byte
)
