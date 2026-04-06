# op-bot

Go backend API server for GitHub OAuth, portfolio deployment, and resume validation. Runs as a microservice separate from the frontend.

## Files

- `main.go`: API server
- `server.go`: HTTP route handlers
- `github.go`: GitHub API integration
- `config.go`: environment configuration
- `handlers.go`: request handlers
- `middleware.go`: HTTP middleware
- `main_test.go`: tests
- `openapi.json`: OpenAPI 3.0 specification
- `.env.example`: environment template
- `Makefile`: build commands

## Environment

Copy `.env.example` to `.env` and set:

- `APP_CLIENT_ID`
- `APP_CLIENT_SECRET`
- `APP_INSTALL_URL` (optional)
- `OAUTH_CALLBACK_URL` (optional, defaults to `http://localhost:8080/auth/github/callback` in non-production)
- `PORT` (optional, default `8080`)
- `CORS_ALLOWED_ORIGINS` (optional, comma-separated, required when frontend is on different origin)
- `THEME_SOURCE_REPO` (optional, default `oneclick-portfolio/awesome-github-portfolio`)
- `THEME_SOURCE_REF` (optional, default `main`)

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

Example with explicit browser origins (required for frontend on separate port/domain):

```bash
CORS_ALLOWED_ORIGINS=http://localhost:4173,http://localhost:5173,https://portfolios.example.com go run .
```

## Architecture

op-bot is a **microservice API backend** that runs separately from the frontend:

- **Backend (op-bot)**: Handles GitHub OAuth, resume validation, repository creation, and selected-theme file publishing
- **Frontend**: Deployed separately, manages theme selection and file uploads

The backend serves only API endpoints (`/auth/*`, `/api/*`) and Swagger documentation (`/swagger/*`). No static assets are served by the backend.

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

- **CORS is required**: Frontend and backend must communicate across origins. Set `CORS_ALLOWED_ORIGINS` to your frontend domain(s).
- **Security headers**: Applied on all responses via Helmet middleware.
- **Timeouts**: HTTP server runs with read/write/idle/header timeouts for safer production behavior.
- **No static assets**: Backend is API-only; frontend is deployed independently.

## API Endpoints

- `GET /auth/github/start`
- `GET /auth/github/callback`
- `GET /api/github/me`
- `POST /api/github/logout`
- `POST /api/resume/validate`
- `POST /api/github/deploy`
- `GET /swagger` - Swagger UI
- `GET /swagger/openapi.json` - OpenAPI spec
