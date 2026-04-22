.PHONY: fresh-setup docker-pull-antlr-image docker-publish-antlr generate-grammar \
        docker-build docker-run docker-publish docker-clean test help

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

# ── Setup ─────────────────────────────────────────────────────────────────────

fresh-setup:
	@echo "Setting up Craft development environment..."
	@echo "1. Pulling ANTLR image and generating grammar..."
	$(MAKE) docker-pull-antlr-image generate-grammar
	@echo "✅ Fresh setup complete! You can now:"
	@echo "   - Use 'make docker-build && make docker-run' to start the server"
	@echo "   - Visit the standalone VS Code extension at: https://github.com/tcarcao/craft-vscode-extension"

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

# go vet scopes only to hand-written packages that do NOT transitively import
# pkg/parser (ANTLR-generated). pkg/parser has pre-existing unreachable-code
# warnings in generated code; we must not edit it per AGENT.md never-do rule #1,
# and go vet reports all transitive packages, making full ./... exclusion
# impractical without a linter config. Migration packages (S3+) are clean.
vet:
	go vet \
		github.com/tcarcao/craft/internal/ast \
		github.com/tcarcao/craft/internal/lexer \
		github.com/tcarcao/craft/internal/syntax \
		github.com/tcarcao/craft/internal/sema \
		github.com/tcarcao/craft/internal/workspace \
		github.com/tcarcao/craft/internal/lsp \
		github.com/tcarcao/craft/pkg/craft

# ── Help ──────────────────────────────────────────────────────────────────────

help:
	@echo ""
	@echo "Setup:"
	@echo "  fresh-setup              - Complete setup for new repository clones"
	@echo ""
	@echo "Grammar:"
	@echo "  docker-pull-antlr-image  - Pull ANTLR builder image from Docker Hub"
	@echo "  docker-publish-antlr     - Build multi-arch ANTLR image and push to Docker Hub (maintainers only, rarely needed)"
	@echo "  generate-grammar         - Generate Go parser from ANTLR grammar"
	@echo ""
	@echo "Docker (server):"
	@echo "  docker-build             - Build craft server image locally"
	@echo "  docker-run               - Run craft server container"
	@echo "  docker-publish           - Build multi-arch server image and push to Docker Hub (maintainers only)"
	@echo "  docker-clean             - Remove local craft server image"
	@echo ""
	@echo "Other:"
	@echo "  test                     - Run tests"
	@echo ""
	@echo "Variables:"
	@echo "  IMAGE_TAG                - Image tag for docker-publish (default: latest)"
	@echo "                             Example: make docker-publish IMAGE_TAG=v2.0.0"
