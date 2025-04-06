
MAIN_FILE := ./cmd/server/main.go
MIGRATION_FILE := ./migrations/main.go

.PHONY: all
all: ogen run

.PHONY: run
run:
	go run ${MAIN_FILE}

.PHONY: test
test:
	go install gotest.tools/gotestsum@latest
	gotestsum --format-hide-empty-pkg --format-icons hivis

# npm install -g @redocly/cli
.PHONY: ogen
ogen:
	redocly bundle ./api/openapi/openapi.yaml -o ./api/openapi/bundled.yaml
	go tool ogen \
		-config ./api/ogen.yaml \
		--target ./api/ogen \
		--package api \
		--clean \
		./api/openapi/openapi.yaml

# golangci-lint should use binary installation instead of "go tool":
# https://golangci-lint.run/welcome/install/#local-installation
.PHONY: lint
lint:
	golangci-lint run --show-stats

# npm i -g @stoplight/spectral-cli
.PHONY: lint-spec
lint-spec:
	spectral lint ./api/openapi/openapi.yaml --ruleset ./api/.spectral.yaml

.PHONY: generate
generate:
	go generate ./...

.PHONY: compose-up
compose-up:
	docker compose up -d

.PHONY: compose-up-build
compose-up-build:
	docker compose up -d --build
	docker container prune -f

.PHONY: compose-down
compose-down:
	docker compose down

.PHONY: migrate-up
migrate-up:
	go run ${MIGRATION_FILE} up

.PHONY: migrate-with-data
migrate-up-with-data:
	go run ${MIGRATION_FILE} up-with-data

.PHONY: migrate-data-only
migrate-data-only:
	go run ${MIGRATION_FILE} data-only

.PHONY: migrate-down
migrate-down:
	go run ${MIGRATION_FILE} down

.PHONY: migrate-down-to
migrate-down-to-0:
	go run ${MIGRATION_FILE} down-to 0

.PHONY: migrate-version
migrate-version:
	go run ${MIGRATION_FILE} version
