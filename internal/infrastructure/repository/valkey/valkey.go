// Package valkey contains methods for working with Valkey repository.
package valkey

import (
	"github.com/valkey-io/valkey-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Repository is a Valkey repository wrapper.
type Repository struct {
	valkey valkey.Client
	tracer trace.Tracer
}

// New returns a new Valkey repository.
func New(client valkey.Client) *Repository {
	return &Repository{
		valkey: client,
		tracer: otel.GetTracerProvider().Tracer("ValkeyRepository"),
	}
}
