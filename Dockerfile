##################################################
# Build stage
##################################################
FROM golang:1.24.2-alpine3.21 AS builder

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
FROM alpine:3.21 AS runtime

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
