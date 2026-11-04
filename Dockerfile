#*#################################################
#* Build stage
#*#################################################
FROM golang:1.27.3-alpine3.24@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS builder

WORKDIR /build

# Copy dependency files first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags "-s -w" \
    -o main ./cmd/server

#*#################################################
#* Runtime stage
#*#################################################
FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b AS runtime

WORKDIR /app

RUN apk add --no-cache ca-certificates curl && \
    update-ca-certificates

COPY --from=builder /build/main .

RUN chown -R nobody:nogroup /app
USER nobody

HEALTHCHECK --start-period=10s --retries=3 \
    CMD [ "curl", "-f", "http://localhost:4174/health" ]

EXPOSE 4174
ENTRYPOINT ["./main"]
