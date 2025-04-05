package middleware

import (
	"strings"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/ogen-go/ogen/middleware"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// OpenTelemetry is a middleware that sets additional span attributes from request context.
type OpenTelemetry struct{}

// Middleware sets additional span attributes from request context.
func (ot OpenTelemetry) Middleware(req middleware.Request, next middleware.Next) (middleware.Response, error) {
	ctx := req.Context
	span := trace.SpanFromContext(ctx)

	remoteAddr := strings.Split(req.Raw.Header.Get("X-Forwarded-For"), ",")[0]
	if remoteAddr == "" {
		remoteAddr = req.Raw.RemoteAddr
	}

	attrs := []attribute.KeyValue{
		semconv.UserAgentName(req.Raw.UserAgent()),
		semconv.ClientAddress(remoteAddr),
		{
			Key:   "request_id",
			Value: attribute.StringValue(chiMiddleware.GetReqID(ctx)),
		},
	}
	span.SetAttributes(attrs...)

	return next(req)
}
