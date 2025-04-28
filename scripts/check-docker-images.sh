#!/bin/bash

set -e

POSTGRES_COMPOSE_VERSION=$(grep 'image: postgres:' docker-compose.yaml | sed -E 's/.*postgres:([^@]+@sha256:[^ ]+).*/\1/')
POSTGRES_TESTCONTAINERS_VERSION=$(grep 'postgres.Run' -A 5 tests/containers/postgres.go | grep '"' | head -n1 | sed -E 's/.*postgres:([^"]+)".*/\1/')

echo "Checking Postgres version consistency..."
echo "docker-compose.yml version: $POSTGRES_COMPOSE_VERSION"
echo "testcontainers-go version:  $POSTGRES_TESTCONTAINERS_VERSION"

if [[ "$POSTGRES_COMPOSE_VERSION" != "$POSTGRES_TESTCONTAINERS_VERSION" ]]; then
  echo "::error::Mismatch between docker-compose Postgres ($POSTGRES_COMPOSE_VERSION) and testcontainers Postgres ($POSTGRES_TESTCONTAINERS_VERSION)"
  exit 1
fi
echo "✅ Postgres versions match!"

# Valkey
VALKEY_COMPOSE_VERSION=$(grep 'image: valkey/valkey:' docker-compose.yaml | sed -E 's/.*valkey\/valkey:([^@]+@sha256:[^ ]+).*/\1/')
VALKEY_TESTCONTAINERS_VERSION=$(grep 'valkey.Run' -A 5 tests/containers/valkey.go | grep '"' | head -n1 | sed -E 's/.*"valkey\/valkey:([^"]+)".*/\1/')

echo "Checking Valkey version consistency..."
echo "docker-compose.yml version: $VALKEY_COMPOSE_VERSION"
echo "testcontainers-go version:  $VALKEY_TESTCONTAINERS_VERSION"

if [[ "$VALKEY_COMPOSE_VERSION" != "$VALKEY_TESTCONTAINERS_VERSION" ]]; then
  echo "::error::Mismatch between docker-compose Valkey ($VALKEY_COMPOSE_VERSION) and testcontainers Valkey ($VALKEY_TESTCONTAINERS_VERSION)"
  exit 1
fi
echo "✅ Valkey versions match!"