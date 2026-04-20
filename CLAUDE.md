# Craft — CLAUDE.md

> Agent instructions for the **Craft** repository (`github.com/tcarcao/craft`).

## Project Overview

Craft is a domain-specific language (DSL) for modeling business use cases and domain interactions. It generates visual diagrams (C4, domain-flow, sequence) from `.craft` source files.

**Tech Stack:** Go 1.22, ANTLR4 4.13.2, Docker / Podman, GitHub Actions.

---

## Repository Layout

```
cmd/
  server/       HTTP server (diagram generation API)
  craft/        CLI entry point
internal/
  parser/       ANTLR visitor + Go types (Services, Domains, Exposures, UseCases)
  visualizer/   PlantUML / C4 diagram generators
  processor/    .craft file processing pipeline
pkg/
  parser/       ANTLR-generated Go parser (auto-generated — do not edit manually)
tools/
  antlr-grammar/  Craft.g4 — the ANTLR grammar source
build/
  package/
    Dockerfile          Multi-stage server image
    antlr.Dockerfile    ANTLR builder image (tiagocarcao/antlr4-craft)
.github/
  workflows/
    deploy.yml    Deploy VitePress docs to GitHub Pages
    publish.yml   Build & push tiagocarcao/craft to Docker Hub on v* tags
docs/
  page/         VitePress documentation site
  superpowers/  Agent plans and specs
examples/       Example .craft files
```

---

## DSL Language Features

| Keyword       | Description |
|---------------|-------------|
| `services`    | Groups related bounded contexts into deployable service units |
| `contexts:`   | Bounded contexts — solution-space units grouped within a service |
| `data-stores:` | Data stores associated with a service |
| `language:`   | Implementation language of a service |
| `use_case`    | Business scenario with triggers and domain actions |
| `when`        | Scenario trigger (actor, event, domain listener, CRON) |
| `asks … to`   | Synchronous domain-to-domain action |
| `notifies`    | Asynchronous event publication |
| `domain`      | A top-level domain grouping bounded contexts |
| `expose`      | API exposure definition |

### Key DSL Example

```craft
services {
  UserService: {
    contexts: Authentication, Profile
    data-stores: user_db
    language: golang
  }
}

use_case "User Registration" {
  when Business_User creates Account
    Authentication validates email format
    Authentication asks Database to check email uniqueness
    Profile creates user profile
    Authentication notifies "User Registered"
}
```

> **Breaking change (v2.0):** `domains:` was renamed to `contexts:` in service blocks.
> `of:` was renamed to `contexts:` in exposure blocks.

---

## Container Requirements

- **Docker or Podman**: Auto-detected — whichever is available is used (`CONTAINER_RUNTIME` in Makefile)
- **ANTLR4 Docker Image**: Pulled from `tiagocarcao/antlr4-craft:4.13.2` (no local build required)

---

## Common Commands

```bash
# First-time setup — pulls ANTLR image and regenerates parser
make fresh-setup

# Grammar generation only (pulls ANTLR image automatically)
make generate-grammar

# Build and run the server container
make docker-build && make docker-run

# Run tests
make test

# Multi-arch publish to Docker Hub (maintainers only, requires Docker + Buildx)
make docker-publish IMAGE_TAG=v2.0.0

# Publish ANTLR builder image (manual, rarely needed)
make docker-publish-antlr
```

---

## Grammar Generation

The ANTLR grammar lives in `tools/antlr-grammar/Craft.g4`. Generated Go parser code lives in `pkg/parser/` (gitignored — regenerated on demand).

To regenerate after grammar changes:

```bash
make generate-grammar
# (automatically pulls tiagocarcao/antlr4-craft:4.13.2 from Docker Hub)
```

Do **not** edit files in `pkg/parser/` manually.

---

## GitHub Actions

| Workflow       | Trigger                | Purpose |
|----------------|------------------------|---------|
| `deploy.yml`   | Push to `main` (docs/) | Deploy VitePress docs to GitHub Pages |
| `publish.yml`  | Push `v*` tag          | Run tests → build multi-arch Docker image → push to Docker Hub |

### Required GitHub Secrets (for `publish.yml`)

| Secret              | Value |
|---------------------|-------|
| `DOCKERHUB_USERNAME` | `tiagocarcao` |
| `DOCKERHUB_TOKEN`    | Docker Hub access token |

Set these at: GitHub repo → Settings → Secrets and variables → Actions.

---

## Testing

```bash
go test ./...
```

Parser tests live in `internal/parser/`. After grammar changes, run tests to confirm visitors handle new types correctly.

---

## VS Code Extension

The Craft VS Code extension is a separate repository:
🔗 **[craft-vscode-extension](https://github.com/tcarcao/craft-vscode-extension)**

It uses `tree-sitter-craft` for syntax highlighting and communicates with the Craft HTTP server for diagram generation.
