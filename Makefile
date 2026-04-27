.PHONY: docker-build docker-run docker-publish docker-clean test vet test-release help

# Auto-detect container runtime (docker preferred, podman fallback)
CONTAINER_RUNTIME := $(shell command -v docker 2>/dev/null || command -v podman 2>/dev/null)

IMAGE_NAME        = tiagocarcao/craft
IMAGE_TAG        ?= latest
DOCKERFILE_PATH   = build/package/Dockerfile

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

vet:
	go vet \
		github.com/tcarcao/craft/internal/ast \
		github.com/tcarcao/craft/internal/lexer \
		github.com/tcarcao/craft/internal/syntax \
		github.com/tcarcao/craft/internal/sema \
		github.com/tcarcao/craft/internal/workspace \
		github.com/tcarcao/craft/internal/lsp \
		github.com/tcarcao/craft/pkg/craft

test-release: ## Download and smoke-test the latest GitHub release binary for the current platform
	./scripts/test-release.sh

# ── Help ──────────────────────────────────────────────────────────────────────

help:
	@echo ""
	@echo "Docker (server):"
	@echo "  docker-build             - Build craft server image locally"
	@echo "  docker-run               - Run craft server container"
	@echo "  docker-publish           - Build multi-arch server image and push to Docker Hub (maintainers only)"
	@echo "  docker-clean             - Remove local craft server image"
	@echo ""
	@echo "Other:"
	@echo "  test                     - Run tests"
	@echo "  vet                      - Run go vet on hand-written packages"
	@echo "  test-release             - Download and smoke-test the latest GitHub release (current platform)"
	@echo "                             Example: make test-release  (uses latest tag)"
	@echo "                             Example: ./scripts/test-release.sh 0.1.0  (specific version)"
	@echo ""
	@echo "Variables:"
	@echo "  IMAGE_TAG                - Image tag for docker-publish (default: latest)"
	@echo "                             Example: make docker-publish IMAGE_TAG=v2.0.0"
