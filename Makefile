.PHONY: build run test clean

IMAGE_NAME=craft
IMAGE_TAG=latest
DOCKERFILE_PATH=build/package/Dockerfile
ANTLR_IMAGE_NAME=antlr/antlr4
ANTLR_IMAGE_TAG=4.13.2
ANTLR_GRAMMAR_PATH=tools/antlr-grammar
ANTLR_GRAMMAR_FILENAME=Craft.g4
GOLANG_GRAMMAR_PATH=pkg/parser/
VSCODE_EXTENSION=tools/vscode-extension/server

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
	cd $(shell pwd)/$(VSCODE_EXTENSION) && pnpm run generate

test:
	go test ./...

help:
	@echo "Makefile commands:"
	@echo "  docker-build   			- Build Docker image from Dockerfile"
	@echo "  docker-run     			- Run Docker container"
	@echo "  docker-clean   			- Remove Docker image"
	@echo "  docker-build-antlr-image	- Generate the ANTLR Docker image to generate the grammar"
	@echo "  docker-clean-antlr-image	- Remove the ANTLR Docker image"
	@echo "  generate-grammar			- Generate the golang and javascript versions of the grammar"
	@echo "  test   					- Run the tests"

