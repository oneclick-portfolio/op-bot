# op-bot

Go backend for GitHub OAuth, deployment APIs, resume validation, and static frontend hosting.

## Files

- `main.go`: API server and static file host
- `main_test.go`: tests
- `openapi.json`: OpenAPI 3.0 specification
- `.env.example`: environment template
- `Makefile`: build commands

## Environment

Copy `.env.example` to `.env` and set:

- `APP_CLIENT_ID`
- `APP_CLIENT_SECRET`
- `APP_INSTALL_URL` (optional)
- `PORT` (optional, default `8080`)
- `CORS_ALLOWED_ORIGINS` (optional, comma-separated)

```bash
cp .env.example .env
```

## Prerequisites

- Go 1.24+
- Optional for hot reload: Air

Install Air:

```bash
go install github.com/air-verse/air@latest
```

## Run

Run once (no hot reload):

```bash
go run .
```

Run with hot reload:

```bash
make dev
```

Note: do not use `go run main.go`. That runs only one file and skips the rest of the package.

Example with explicit browser origins:

```bash
CORS_ALLOWED_ORIGINS=http://localhost:5173,https://app.example.com go run .
```

## Test

```bash
make test
```

## Build

```bash
make build
./op-bot
```

## Docker

```bash
make docker-build
make docker-run
make stop
make clean
```

## API Documentation

Swagger UI available at: http://localhost:8080/swagger

## Troubleshooting

- `make dev` says Air is missing: install with `go install github.com/air-verse/air@latest`
- Add `$(go env GOPATH)/bin` to your shell `PATH` if Air is still not found.
- `listen tcp :8080: bind: address already in use`: stop the process using port `8080` or run with another port using `PORT=8081 go run .`
- Browser gets blocked by CORS: set `CORS_ALLOWED_ORIGINS` to your frontend origin(s).

## Production Notes

- CORS middleware is enabled with explicit origin allow-listing.
- Security headers are applied on all responses.
- HTTP server runs with read/write/idle/header timeouts for safer production behavior.

## API Endpoints

- `GET /auth/github/start`
- `GET /auth/github/callback`
- `GET /api/github/me`
- `POST /api/github/logout`
- `POST /api/resume/validate`
- `POST /api/github/deploy`
- `GET /swagger` - Swagger UI
- `GET /swagger/openapi.json` - OpenAPI spec
