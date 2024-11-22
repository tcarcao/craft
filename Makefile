.PHONY: clean generate build test help examples extension docker

# Variables
ANTLR_VERSION = 4.13.1
ANTLR_JAR = tools/antlr-$(ANTLR_VERSION)-complete.jar
GRAMMAR_DIR = tools/antlr-grammar
OUTPUT_DIR = pkg/parser
BINARY_NAME = archdsl
BUILD_DIR = build
VERSION ?= 1.0.0
LDFLAGS = -ldflags "-X main.Version=$(VERSION)"

# Default target
help:
	@echo "ArchDSL Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  generate    - Generate ANTLR parser code"
	@echo "  clean       - Clean generated files and build artifacts"
	@echo "  test        - Run all tests"
	@echo "  build       - Build the CLI binary"
	@echo "  examples    - Process example files and generate diagrams"
	@echo "  extension   - Build VS Code extension"
	@echo "  docker      - Build Docker image"
	@echo "  install     - Install binary to GOPATH/bin"
	@echo "  release     - Build release binaries for multiple platforms"
	@echo ""

# Download ANTLR jar if not present
$(ANTLR_JAR):
	@echo "Downloading ANTLR $(ANTLR_VERSION)..."
	@mkdir -p tools
	@curl -L -o $(ANTLR_JAR) https://www.antlr.org/download/antlr-$(ANTLR_VERSION)-complete.jar

# Generate ANTLR parser code
generate: $(ANTLR_JAR)
	@echo "Generating ANTLR parser code..."
	@mkdir -p $(OUTPUT_DIR)
	@java -jar $(ANTLR_JAR) -Dlanguage=Go -o $(OUTPUT_DIR) $(GRAMMAR_DIR)/*.g4
	@echo "Generated parser code in $(OUTPUT_DIR)/"

clean:
	@echo "Cleaning generated files and build artifacts..."
	@rm -rf $(OUTPUT_DIR)
	@rm -rf $(BUILD_DIR)
	@rm -rf output/
	@rm -rf client/out/
	@rm -rf server/out/
	@rm -rf node_modules/
	@rm -f *.vsix

test:
	@echo "Running Go tests..."
	@go test -v ./...
	@echo "All tests passed!"

build: generate
	@echo "Building $(BINARY_NAME) v$(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/archdsl
	@echo "Binary built: $(BUILD_DIR)/$(BINARY_NAME)"

install: build
	@echo "Installing $(BINARY_NAME) to GOPATH/bin..."
	@go install $(LDFLAGS) ./cmd/archdsl
	@echo "Installed successfully!"

examples: build
	@echo "Processing example files..."
	@mkdir -p output/simple output/ecommerce output/ecommerce-complex output/platform
	@./$(BUILD_DIR)/$(BINARY_NAME) -input examples/simple.dsl -output output/simple
	@./$(BUILD_DIR)/$(BINARY_NAME) -input examples/e-commerce.dsl -output output/ecommerce
	@./$(BUILD_DIR)/$(BINARY_NAME) -input examples/e-comerce-complex.dsl -output output/ecommerce-complex
	@./$(BUILD_DIR)/$(BINARY_NAME) -input examples/microservices-platform.dsl -output output/platform
	@echo "Example diagrams generated in output/ directory"

extension:
	@echo "Building VS Code extension..."
	@npm install
	@npm run compile
	@vsce package
	@echo "VS Code extension packaged as .vsix file"

docker:
	@echo "Building Docker image..."
	@docker build -t archdsl:$(VERSION) .
	@docker tag archdsl:$(VERSION) archdsl:latest
	@echo "Docker image built: archdsl:$(VERSION)"

release: clean
	@echo "Building release binaries..."
	@mkdir -p $(BUILD_DIR)/release
	# Linux AMD64
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-linux-amd64 ./cmd/archdsl
	# Linux ARM64
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-linux-arm64 ./cmd/archdsl
	# macOS AMD64
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-darwin-amd64 ./cmd/archdsl
	# macOS ARM64
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-darwin-arm64 ./cmd/archdsl
	# Windows AMD64
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/release/$(BINARY_NAME)-windows-amd64.exe ./cmd/archdsl
	@echo "Release binaries built in $(BUILD_DIR)/release/"

# Development helpers
fmt:
	@echo "Formatting Go code..."
	@go fmt ./...

lint:
	@echo "Running Go linter..."
	@golangci-lint run ./...

deps:
	@echo "Updating dependencies..."
	@go mod tidy
	@go mod download

.PHONY: fmt lint deps