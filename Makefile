.PHONY: build run test clean fresh-setup compose-build compose-up compose-down compose-logs

IMAGE_NAME=craft
IMAGE_TAG=latest
DOCKERFILE_PATH=build/package/Dockerfile
ANTLR_IMAGE_NAME=antlr/antlr4
ANTLR_IMAGE_TAG=4.13.2
ANTLR_GRAMMAR_PATH=tools/antlr-grammar
ANTLR_GRAMMAR_FILENAME=Craft.g4
GOLANG_GRAMMAR_PATH=pkg/parser/
COMPOSE_FILE=docker-compose.yml

docker-build:
	@echo "Building Docker image..."
	podman build -t $(IMAGE_NAME):$(IMAGE_TAG) -f $(DOCKERFILE_PATH) .

docker-run:
	@echo "Running Docker container..."
	podman run --rm -it -p 8080:8080 $(IMAGE_NAME):$(IMAGE_TAG)

docker-clean:
	@echo "Removing Docker image..."
	podman rmi -f $(IMAGE_NAME):$(IMAGE_TAG)

docker-build-antlr-image:
	@if [ -z "$$(podman images -q $(ANTLR_IMAGE_NAME):$(ANTLR_IMAGE_TAG))" ]; then \
		echo "Building ANTLR Docker image..."; \
		git clone https://github.com/antlr/antlr4.git; \
		cd antlr4/docker && podman build -t $(ANTLR_IMAGE_NAME):$(ANTLR_IMAGE_TAG) --platform linux/amd64 .; \
		cd ../../ && rm -rf antlr4; \
	else \
		echo "ANTLR Docker image already exists."; \
	fi

docker-clean-antlr-image:
	podman rmi $(ANTLR_IMAGE_NAME):$(ANTLR_IMAGE_TAG)

generate-grammar: docker-build-antlr-image
	mkdir -p $(shell pwd)/$(ANTLR_GRAMMAR_PATH)
	mkdir -p $(shell pwd)/$(GOLANG_GRAMMAR_PATH)
	podman run --platform linux/amd64 --rm -v $(shell pwd)/$(ANTLR_GRAMMAR_PATH):/work -v $(shell pwd)/$(GOLANG_GRAMMAR_PATH):/output -w /work $(ANTLR_IMAGE_NAME):$(ANTLR_IMAGE_TAG) -Dlanguage=Go -visitor -o /output $(ANTLR_GRAMMAR_FILENAME)

test:
	go test ./...

# Docker Compose commands for running Craft with web IDE
compose-build:
	@echo "Building services with Docker Compose..."
	podman compose -f $(COMPOSE_FILE) build

compose-up:
	@echo "Starting Craft server and IDE..."
	@echo "Server will be available at: http://localhost:8080"
	@echo "Web IDE will be available at: http://localhost:3000"
	podman compose -f $(COMPOSE_FILE) up

compose-up-detached:
	@echo "Starting Craft server and IDE in detached mode..."
	podman compose -f $(COMPOSE_FILE) up -d
	@echo "✅ Services started!"
	@echo "   - Server: http://localhost:8080"
	@echo "   - Web IDE: http://localhost:3000"

compose-down:
	@echo "Stopping all services..."
	podman compose -f $(COMPOSE_FILE) down

compose-logs:
	podman compose -f $(COMPOSE_FILE) logs -f

compose-restart:
	@echo "Restarting services..."
	podman compose -f $(COMPOSE_FILE) restart

fresh-setup:
	@echo "Setting up Craft development environment..."
	@echo "1. Generating ANTLR grammar..."
	$(MAKE) generate-grammar
	@echo ""
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

help:
	@echo "Makefile commands:"
	@echo ""
	@echo "Setup:"
	@echo "  fresh-setup              - Complete setup for new repository clones"
	@echo ""
	@echo "Docker (single service):"
	@echo "  docker-build             - Build Docker image from Dockerfile"
	@echo "  docker-run               - Run Docker container"
	@echo "  docker-clean             - Remove Docker image"
	@echo ""
	@echo "Docker Compose (with Web IDE):"
	@echo "  compose-build            - Build all services (server + IDE)"
	@echo "  compose-up               - Start all services (foreground)"
	@echo "  compose-up-detached      - Start all services (background)"
	@echo "  compose-down             - Stop all services"
	@echo "  compose-logs             - View logs from all services"
	@echo "  compose-restart          - Restart all services"
	@echo ""
	@echo "Grammar:"
	@echo "  docker-build-antlr-image - Generate the ANTLR Docker image"
	@echo "  docker-clean-antlr-image - Remove the ANTLR Docker image"
	@echo "  generate-grammar         - Generate the golang version of the grammar"
	@echo ""
	@echo "Testing:"
	@echo "  test                     - Run the tests"

