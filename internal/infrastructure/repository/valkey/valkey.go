// Package valkey contains methods for working with Valkey repository.
package valkey

import (
	"context"
	"time"

	"github.com/valkey-io/valkey-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// not expected to be changed a lot, so it is a constant here.
const cleanupInterval = 1 * time.Minute

// Repository is a Valkey repository wrapper.
type Repository struct {
	valkey valkey.Client
	tracer trace.Tracer
}

// New returns a new Valkey repository.
func New(ctx context.Context, client valkey.Client) *Repository {
	r := &Repository{
		valkey: client,
		tracer: otel.GetTracerProvider().Tracer("ValkeyRepository"),
	}
	r.StartPeriodicCleanup(ctx, cleanupInterval)

	return r
}
