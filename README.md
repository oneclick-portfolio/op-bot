# op-bot

Go backend API server for GitHub OAuth, portfolio deployment, and resume validation. Runs as a microservice separate from the frontend.

## Architecture

The codebase follows a minimal root structure with most runtime code organized in `internal` packages:

### Root Files (Configuration & Composition)
- `main.go`: Application entry point, composes AppContext and middleware chain
- `config.go`: Environment variable loading and AppContext construction
- `go.mod`, `go.sum`: Dependency management
- `Makefile`: Build and development commands
- `README.md`, `LICENSE`: Project documentation

### Root Files (Backwards Compatibility Wrappers)
These files provide backwards compatibility by delegating to internal packages:
- `middleware.go` → `internal/httpapi/middleware`
- `response.go` → `internal/httpapi/response`
- `appauth.go` → `internal/auth/service`
- `parse.go` → `internal/ai/pdfparser`
- `handlers.go`, `github.go`, `theme.go`, `resume.go`: Contain handler implementations (pending extraction)
- `logging.go`: Delegates to `internal/logging`

### Internal Packages (Business Logic)

**Core Infrastructure:**
- `internal/appctx/context.go`: AppContext holds all configuration and dependencies
- `internal/logging/logger.go`: Logger setup and request ID tracking
- `internal/models/`: API and domain models

**HTTP Layer:**
- `internal/httpapi/router.go`: Route registration
- `internal/httpapi/middleware/`: Request logging, CORS, security headers, panic recovery
- `internal/httpapi/response/`: HTTP response writers and error handlers

**Services & Domain:**
- `internal/auth/service.go`: GitHub App JWT and installation token operations
- `internal/ai/pdfparser/`: PDF parsing to JSON using Gemini AI
- `internal/services/`: Service-layer orchestration (GitHub API, theme building)
- `internal/repository/`: GitHub API client adapter
- `internal/utils/`: Utilities (env loading, logging, string helpers)

### Configuration

Create `.env` and `.env.production` files with:
- `APP_CLIENT_ID`, `APP_CLIENT_SECRET`: GitHub OAuth app credentials
- `APP_ID`, `APP_PRIVATE_KEY`: GitHub App credentials for deployment
- `OAUTH_CALLBACK_URL`: OAuth callback URL (auto-configured in non-production)
- `CORS_ALLOWED_ORIGINS`: CORS allowed origins (defaults to localhost in non-production)
- `APP_INSTALL_URL` (optional): Custom installation URL
- `PORT` (optional, default `8080`)
- `LOG_LEVEL` (optional, default `info`)
- `GOOGLE_API_KEY`, `GEMINI_MODEL`: AI credentials for PDF parsing

## Build & Development

- `make build`: Build release binary
- `make test`: Run all tests
- `make env`: Create development `.env` file
- `make env-prod`: Create production `.env` file

## Testing

Tests are co-located with source files:
- `internal/utils/*_test.go`
- `internal/models/*_test.go`
- `main_test.go`: Integration tests for the application

## Environment

Create `.env` and `.env.production` (or run `make env` and `make env-prod`) and set:

- `APP_CLIENT_ID`
- `APP_CLIENT_SECRET`
- `APP_ID` for repository deployment and bot commits
- `APP_PRIVATE_KEY` for repository deployment and bot commits
- `OAUTH_CALLBACK_URL` in production; optional in local development, defaults to `http://localhost:8080/auth/github/callback` in non-production
- `CORS_ALLOWED_ORIGINS` in production when frontend is on a different origin
- `NODE_ENV=production` when running outside Vercel in production
- `APP_INSTALL_URL` (optional)
- `PORT` (optional, default `8080`)

Notes:

- On Vercel, `VERCEL_ENV=production` is provided automatically, so you normally do not need to set `NODE_ENV` there.
- If you only need GitHub sign-in and `/api/github/me`, `APP_CLIENT_ID`, `APP_CLIENT_SECRET`, `OAUTH_CALLBACK_URL`, and `CORS_ALLOWED_ORIGINS` are the critical settings.
- If you also need `/api/github/deploy`, `APP_ID` and `APP_PRIVATE_KEY` are required.

```bash
make env
```

For production template setup:

```bash
make env-prod
make check-prod-env
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

Example production configuration for your current deployment:

```bash
APP_CLIENT_ID=...
APP_CLIENT_SECRET=...
APP_ID=...
APP_PRIVATE_KEY="-----BEGIN RSA PRIVATE KEY-----\n...\n-----END RSA PRIVATE KEY-----"
OAUTH_CALLBACK_URL=https://op-bot-mauve.vercel.app/auth/github/callback
CORS_ALLOWED_ORIGINS=https://oneclick-portfolio.github.io
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

Production binary build:

```bash
make build-prod
```

## Docker

```bash
make docker-build
make docker-run
make stop
make clean
```

Note: `make docker-run` expects a populated `.env.production` file.

## API Documentation

Swagger UI available at: http://localhost:8080/swagger

## Troubleshooting

- `make dev` says Air is missing: install with `go install github.com/air-verse/air@latest`
- Add `$(go env GOPATH)/bin` to your shell `PATH` if Air is still not found.
- `listen tcp :8080: bind: address already in use`: stop the process using port `8080` or run with another port using `PORT=8081 go run .`
- Browser gets blocked by CORS: set `CORS_ALLOWED_ORIGINS` to your frontend origin(s).

## Production Notes

- **CORS is required**: Frontend and backend must communicate across origins. Set `CORS_ALLOWED_ORIGINS` to your frontend domain(s).
- **Production mode matters**: secure cross-site auth cookies are enabled when `NODE_ENV=production` or `VERCEL_ENV=production`.
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

Deploy contract note:

- `POST /api/github/deploy` now requires `themeRepoLink` in the request body.
- `themeRepoLink` must be a `https://github.com/...` repository URL, optionally including `/tree/{ref}`.
