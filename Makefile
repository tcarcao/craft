.PHONY: clean generate build test help

# Variables
ANTLR_VERSION = 4.13.1
ANTLR_JAR = tools/antlr-$(ANTLR_VERSION)-complete.jar
GRAMMAR_DIR = tools/antlr-grammar
OUTPUT_DIR = pkg/parser

# Default target
help:
	@echo "Available targets:"
	@echo "  generate  - Generate ANTLR parser code"
	@echo "  clean     - Clean generated files"
	@echo "  test      - Run tests"
	@echo "  build     - Build the project"

# Download ANTLR jar if not present
$(ANTLR_JAR):
	@echo "Downloading ANTLR $(ANTLR_VERSION)..."
	@mkdir -p tools
	@curl -o $(ANTLR_JAR) https://www.antlr.org/download/antlr-$(ANTLR_VERSION)-complete.jar

# Generate ANTLR parser code
generate: $(ANTLR_JAR)
	@echo "Generating ANTLR parser code..."
	@mkdir -p $(OUTPUT_DIR)
	@java -jar $(ANTLR_JAR) -Dlanguage=Go -o $(OUTPUT_DIR) $(GRAMMAR_DIR)/*.g4

clean:
	@echo "Cleaning generated files..."
	@rm -rf $(OUTPUT_DIR)
	@rm -rf build/

test:
	@echo "Running tests..."
	@go test ./...

build: generate
	@echo "Building project..."
	@go build -o build/archdsl ./cmd/archdsl