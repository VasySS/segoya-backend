#!/bin/bash

set -e

GO_MOD_VERSION=$(grep '^go ' go.mod | awk '{print $2}')
GITHUB_CI_VERSION=$(grep 'GO_VERSION:' .github/workflows/ci.yaml | awk '{print $2}' | tr -d "'")

# Extract Go version from Dockerfile: take only before "-" or "@".
APP_DOCKERFILE_VERSION=$(grep '^FROM golang:' Dockerfile | sed -E 's/FROM golang:([0-9]+\.[0-9]+\.[0-9]+).*/\1/')
MIGRATIONS_DOCKERFILE_VERSION=$(grep '^FROM golang:' migrations/Dockerfile | sed -E 's/FROM golang:([0-9]+\.[0-9]+\.[0-9]+).*/\1/')

echo "Checking Go version consistency..."
echo "go.mod version:                 $GO_MOD_VERSION"
echo "application Dockerfile version: $APP_DOCKERFILE_VERSION"
echo "migrations Dockerfile version:  $MIGRATIONS_DOCKERFILE_VERSION"
echo "GitHub Actions version:         $GITHUB_CI_VERSION"

if [[ "$GO_MOD_VERSION" != "$APP_DOCKERFILE_VERSION" ]]; then
  echo "::error::Mismatch between go.mod ($GO_MOD_VERSION) and Dockerfile ($APP_DOCKERFILE_VERSION)"
  exit 1
fi

if [[ "$GO_MOD_VERSION" != "$MIGRATIONS_DOCKERFILE_VERSION" ]]; then
  echo "::error::Mismatch between go.mod ($GO_MOD_VERSION) and migrations Dockerfile ($MIGRATIONS_DOCKERFILE_VERSION)"
  exit 1
fi

if [[ "$GO_MOD_VERSION" != "$GITHUB_CI_VERSION" ]]; then
  echo "::error::Mismatch between go.mod ($GO_MOD_VERSION) and GitHub CI workflow ($GITHUB_CI_VERSION)"
  exit 1
fi

echo "✅ Go versions match!"
