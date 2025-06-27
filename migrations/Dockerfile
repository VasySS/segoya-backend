#*#################################################
#* Build stage
#*#################################################
FROM golang:1.24.4-alpine3.22@sha256:68932fa6d4d4059845c8f40ad7e654e626f3ebd3706eef7846f319293ab5cb7a AS builder

WORKDIR /build

# Copy dependency files first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags "-s -w" \
    -o main ./migrations/main.go

#*#################################################
#* Runtime stage
#*#################################################
FROM alpine:3.22@sha256:8a1f59ffb675680d47db6337b49d22281a139e9d709335b492be023728e11715 AS runtime

WORKDIR /app

COPY --from=builder /build/main .

RUN chown -R nobody:nogroup /app
USER nobody

ENTRYPOINT ["./main"]
