IMAGE  ?= portfolio
TAG    ?= latest
PORT   ?= 8080
BINARY ?= op-bot
AIR    ?= $(shell command -v air 2>/dev/null || echo "$(shell go env GOPATH)/bin/air")
SWAG   ?= $(shell command -v swag 2>/dev/null || echo "$(shell go env GOPATH)/bin/swag")

.PHONY: help dev test build run stop clean env

help: ## Show backend targets
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*##"}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'

env: ## Copy .env.example to .env
	@if [ ! -f .env ]; then cp .env.example .env && echo "✓ Created .env from .env.example"; else echo "✗ .env already exists"; fi

dev: ## Run server with hot reload (requires air)
	@echo "Serving on http://localhost:$(PORT)"
	@test -x "$(AIR)" || (echo "Install Air: go install github.com/air-verse/air@latest" && exit 1)
	PORT=$(PORT) "$(AIR)"

test: ## Run tests
	go test -v ./...

build: ## Build the binary
	go build -o $(BINARY) .

swagger: ## Generate Swagger UI docs (requires comments in code)
	$(SWAG) init -g main.go --output docs && rm -f docs/docs.go docs/swagger.yaml openapi.json

docker-build: ## Build the Docker image
	docker build -t $(IMAGE):$(TAG) .

docker-run: ## Run the Docker container (builds first if image is absent)
	@docker image inspect $(IMAGE):$(TAG) > /dev/null 2>&1 || $(MAKE) docker-build
	docker run --rm -d \
	  --name $(IMAGE) \
	  -p $(PORT):8080 \
	  $(IMAGE):$(TAG)
	@echo "Running at http://localhost:$(PORT)"

stop: ## Stop the running container
	docker stop $(IMAGE) 2>/dev/null || true

clean: stop ## Stop container, remove image and binary
	docker rmi $(IMAGE):$(TAG) 2>/dev/null || true
	rm -f $(BINARY)
