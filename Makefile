.PHONY: fresh-setup docker-pull-antlr-image docker-publish-antlr generate-grammar \
        docker-build docker-run docker-publish docker-clean test help \
        compose-build compose-up compose-up-detached compose-down compose-logs compose-restart

# Auto-detect container runtime (docker preferred, podman fallback)
CONTAINER_RUNTIME := $(shell command -v docker 2>/dev/null || command -v podman 2>/dev/null)

IMAGE_NAME        = tiagocarcao/craft
IMAGE_TAG        ?= latest
DOCKERFILE_PATH   = build/package/Dockerfile

ANTLR_IMAGE_NAME  = tiagocarcao/antlr4-craft
ANTLR_IMAGE_TAG   = 4.13.2
ANTLR_GRAMMAR_PATH    = tools/antlr-grammar
ANTLR_GRAMMAR_FILENAME = Craft.g4
GOLANG_GRAMMAR_PATH   = pkg/parser/
COMPOSE_FILE=docker-compose.yml

# ── Setup ─────────────────────────────────────────────────────────────────────

# Docker Compose commands for running Craft with web IDE
compose-build:
	@echo "Building services with Docker Compose..."
	$(CONTAINER_RUNTIME) compose -f $(COMPOSE_FILE) build

compose-up:
	@echo "Starting Craft server and IDE..."
	@echo "Server will be available at: http://localhost:8080"
	@echo "Web IDE will be available at: http://localhost:3000"
	$(CONTAINER_RUNTIME) compose -f $(COMPOSE_FILE) up

compose-up-detached:
	@echo "Starting Craft server and IDE in detached mode..."
	$(CONTAINER_RUNTIME) compose -f $(COMPOSE_FILE) up -d
	@echo "✅ Services started!"
	@echo "   - Server: http://localhost:8080"
	@echo "   - Web IDE: http://localhost:3000"

compose-down:
	@echo "Stopping all services..."
	$(CONTAINER_RUNTIME) compose -f $(COMPOSE_FILE) down

compose-logs:
	$(CONTAINER_RUNTIME) compose -f $(COMPOSE_FILE) logs -f

compose-restart:
	@echo "Restarting services..."
	$(CONTAINER_RUNTIME) compose -f $(COMPOSE_FILE) restart

fresh-setup:
	@echo "Setting up Craft development environment..."
	@echo "1. Pulling ANTLR image and generating grammar..."
	$(MAKE) docker-pull-antlr-image generate-grammar
	@echo "✅ Fresh setup complete! You can now:"
	@echo ""
	@echo "Option 1: Run with Web IDE (Recommended)"
	@echo "   make compose-build && make compose-up-detached"
	@echo "   Then visit: http://localhost:3000"
	@echo ""
	@echo "Option 2: Run server only"
	@echo "   make docker-build && make docker-run"
	@echo "   Then visit: http://localhost:8080"
	@echo ""
	@echo "For more info:"
	@echo "   - Quick start guide: QUICKSTART_WEB_IDE.md"
	@echo "   - Web IDE docs: WEB_IDE.md"
	@echo "   - VS Code extension: https://github.com/tcarcao/craft-vscode-extension"

# ── Grammar ───────────────────────────────────────────────────────────────────

docker-pull-antlr-image:
	@echo "Pulling ANTLR builder image from Docker Hub..."
	$(CONTAINER_RUNTIME) pull $(ANTLR_IMAGE_NAME):$(ANTLR_IMAGE_TAG)

docker-publish-antlr: ## Build multi-arch ANTLR image and push to Docker Hub (maintainers only, rarely needed)
	@command -v docker >/dev/null 2>&1 || { echo "docker is required for multi-arch publish (not podman)"; exit 1; }
	@echo "Building and pushing multi-arch ANTLR image..."
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--push \
		-t $(ANTLR_IMAGE_NAME):$(ANTLR_IMAGE_TAG) \
		-f build/package/antlr.Dockerfile .

generate-grammar: docker-pull-antlr-image
	mkdir -p $(shell pwd)/$(ANTLR_GRAMMAR_PATH)
	mkdir -p $(shell pwd)/$(GOLANG_GRAMMAR_PATH)
	$(CONTAINER_RUNTIME) run --platform linux/amd64 --rm \
		-v $(shell pwd)/$(ANTLR_GRAMMAR_PATH):/work \
		-v $(shell pwd)/$(GOLANG_GRAMMAR_PATH):/output \
		-w /work \
		$(ANTLR_IMAGE_NAME):$(ANTLR_IMAGE_TAG) \
		-Dlanguage=Go -visitor -o /output $(ANTLR_GRAMMAR_FILENAME)

# ── Docker (server) ───────────────────────────────────────────────────────────

docker-build:
	@echo "Building craft server image..."
	$(CONTAINER_RUNTIME) build -t $(IMAGE_NAME):$(IMAGE_TAG) -f $(DOCKERFILE_PATH) .

docker-run:
	@echo "Running craft server container..."
	$(CONTAINER_RUNTIME) run --rm -it -p 8080:8080 $(IMAGE_NAME):$(IMAGE_TAG)

docker-publish: ## Build multi-arch server image and push to Docker Hub (maintainers only)
	@command -v docker >/dev/null 2>&1 || { echo "docker is required for multi-arch publish (not podman)"; exit 1; }
	@echo "Building and pushing multi-arch craft image (IMAGE_TAG=$(IMAGE_TAG))..."
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--push \
		-t $(IMAGE_NAME):$(IMAGE_TAG) \
		-t $(IMAGE_NAME):latest \
		-f $(DOCKERFILE_PATH) .

docker-clean:
	@echo "Removing craft server image..."
	$(CONTAINER_RUNTIME) rmi -f $(IMAGE_NAME):$(IMAGE_TAG)

# ── Tests ─────────────────────────────────────────────────────────────────────

test:
	go test ./...

# ── Help ──────────────────────────────────────────────────────────────────────

help:
	@echo ""
	@echo "Setup:"
	@echo "  fresh-setup              - Complete setup for new repository clones"
	@echo ""
	@echo "Docker Compose (with Web IDE):"
	@echo "  compose-build            - Build all services (server + IDE)"
	@echo "  compose-up               - Start all services (foreground)"
	@echo "  compose-up-detached      - Start all services (background)"
	@echo "  compose-down             - Stop all services"
	@echo "  compose-logs             - View logs from all services"
	@echo "  compose-restart          - Restart all services"
	@echo ""
	@echo "Docker (server only):"
	@echo "  docker-build             - Build craft server image locally"
	@echo "  docker-run               - Run craft server container"
	@echo "  docker-publish           - Build multi-arch server image and push to Docker Hub (maintainers only)"
	@echo "  docker-clean             - Remove local craft server image"
	@echo ""
	@echo "Grammar:"
	@echo "  docker-pull-antlr-image  - Pull ANTLR builder image from Docker Hub"
	@echo "  docker-publish-antlr     - Build multi-arch ANTLR image and push to Docker Hub (maintainers only, rarely needed)"
	@echo "  generate-grammar         - Generate Go parser from ANTLR grammar"
	@echo ""
	@echo "Testing:"
	@echo "  test                     - Run tests"
	@echo ""
	@echo "Variables:"
	@echo "  IMAGE_TAG                - Image tag for docker-publish (default: latest)"
	@echo "                             Example: make docker-publish IMAGE_TAG=v2.0.0"
