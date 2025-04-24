##################################################
# Build stage
##################################################
FROM golang:1.24.2-alpine3.21@sha256:7772cb5322baa875edd74705556d08f0eeca7b9c4b5367754ce3f2f00041ccee AS builder

WORKDIR /build

# Copy dependency files first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags "-s -w" \
    -o main ./cmd/server

##################################################
# Runtime stage
##################################################
FROM alpine:3.21@sha256:a8560b36e8b8210634f77d9f7f9efd7ffa463e380b75e2e74aff4511df3ef88c AS runtime

WORKDIR /app

RUN apk add --no-cache ca-certificates curl && \
    update-ca-certificates

RUN addgroup -S appgroup && \
    adduser -S appuser -G appgroup && \
    chown -R appuser:appgroup /app

COPY --from=builder --chown=appuser:appgroup /build/main .

USER appuser

HEALTHCHECK --start-period=10s --retries=3 \
    CMD [ "curl", "-f", "http://localhost:4174/health" ]

EXPOSE 4174
ENTRYPOINT ["./main"]
