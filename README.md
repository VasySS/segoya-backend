# segoya-backend

[![License](https://img.shields.io/github/license/VasySS/segoya-backend)](LICENSE)
[![OpenAPI Spec](https://img.shields.io/badge/OpenAPI-3.1-blue)](api/openapi.yaml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/VasySS/segoya-backend)](go.mod)
[![Go Report Card](https://goreportcard.com/badge/github.com/VasySS/segoya-backend)]()

A backend for [Segoya](https://segoya.vasys.su) - panorama guessing game.

## Getting started

### Prerequisites

- **Docker** & **Docker Compose** installed
- Ensure the following ports are available:
  - **5432** (Postgres), **6379** (Valkey), **4174** (application)
  - **4317**, **4318**, **5778**, **9411**, **16686** (Jaeger)

### Installation

1. **Clone the Repository**:

```sh
git clone https://github.com/VasySS/segoya-backend.git
cd segoya-backend
```

2. Create **.env** in root folder and set required fields (look at **.env.example** for reference)

3. Run the command to start the app and all services needed for it:

```sh
docker compose up -d --build
```

### Started services

| Service                  | URL                                             |
| ------------------------ | ----------------------------------------------- |
| Backend API base URL     | http://localhost:4174                           |
| Backend interactive docs | http://localhost:4174/docs/                     |
| Jaeger UI                | http://localhost:16686                          |
| Postgres                 | postgres://postgres:postgrespass@localhost:5432 |
| Valkey                   | valkey://localhost:6379                         |

## Project overview

### Core Components

- [**Postgres**](https://www.postgresql.org/): data storage for users, games, and panoramas
- [**Valkey**](https://valkey.io/): storage for lobbies and sessions
- [**Jaeger**](https://www.jaegertracing.io/) to collect and visualize traces from OpenTelemetry

### Cloud Integrations

- **Cloudflare R2** - S3-compatible object storage
- **Cloudflare Turnstile** - CAPTCHA
- **Yandex/Discord OAuth**

### Go libraries

- [ogen](https://github.com/ogen-go/ogen) and its dependencies ([go-faster/errors](https://github.com/go-faster/errors), [go-faster/jx](https://github.com/go-faster/jx) and [uber-go/multierr](https://github.com/uber-go/multierr)) for generation of boilerplate code from OpenAPI 3.1 specification
- [jwx](https://github.com/lestrrat-go/jwx) to work with JWT tokens
- [chi](https://github.com/go-chi/chi) to create http router
- [testify](https://github.com/stretchr/testify), [mockery](https://github.com/vektra/mockery) and [testcontainers](https://github.com/testcontainers/testcontainers-go) for tests
- [melody](https://github.com/olahol/melody) to handle WebSocket connections
- [cleanenv](https://github.com/ilyakaznacheev/cleanenv) to read .env file
- [pgx](https://github.com/jackc/pgx) and [scany](https://github.com/georgysavva/scany) to query and scan results from Postgres
- [goose](https://github.com/pressly/goose) to run Postgres migrations
- [valkey-go](https://github.com/valkey-io/valkey-go) to query Valkey (Redis fork)
- [aws-sdk](https://github.com/aws/aws-sdk-go-v2) to work with Cloudflare R2 (S3 storage)
- [opentelemetry-go](https://github.com/open-telemetry/opentelemetry-go) to collect traces
