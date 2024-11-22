# Multi-stage build for ArchDSL
FROM openjdk:11-jre-slim as antlr-stage

# Install curl for downloading ANTLR
RUN apt-get update && apt-get install -y curl && rm -rf /var/lib/apt/lists/*

# Download ANTLR
WORKDIR /tools
RUN curl -L -o antlr-4.13.1-complete.jar https://www.antlr.org/download/antlr-4.13.1-complete.jar

FROM golang:1.22-alpine AS build-stage

# Install Java (needed for ANTLR generation)
RUN apk add --no-cache openjdk11-jre

# Set working directory
WORKDIR /app

# Copy Go modules files
COPY go.mod go.sum ./
RUN go mod download

# Copy ANTLR jar from previous stage
COPY --from=antlr-stage /tools/antlr-4.13.1-complete.jar /app/tools/

# Copy source code
COPY . .

# Generate ANTLR code and build
RUN make generate
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.Version=docker" -o archdsl ./cmd/archdsl

FROM alpine:latest AS runtime-stage

# Install PlantUML and Graphviz for diagram generation
RUN apk add --no-cache \
    plantuml \
    graphviz \
    font-bitstream-type1 \
    ttf-dejavu

# Create non-root user
RUN addgroup -g 1001 archdsl && \
    adduser -D -s /bin/sh -u 1001 -G archdsl archdsl

# Copy binary
COPY --from=build-stage /app/archdsl /usr/local/bin/archdsl

# Create directories
RUN mkdir -p /workspace && chown archdsl:archdsl /workspace

# Switch to non-root user
USER archdsl

# Set working directory
WORKDIR /workspace

# Default command
ENTRYPOINT ["archdsl"]