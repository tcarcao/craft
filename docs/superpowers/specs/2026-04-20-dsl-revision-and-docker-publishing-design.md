# Design: DSL Revision (Bounded Contexts) + Docker Publishing

**Date:** 2026-04-20  
**Status:** Approved

---

## Overview

Two independent improvements to the Craft project:

1. **DSL Revision** — Correct DDD terminology: items currently called "subdomains" are more accurately bounded contexts. Update the grammar, parser, visualizer, tree-sitter grammar, VS Code extension, examples, and docs accordingly.
2. **Docker Publishing** — Publish `tiagocarcao/craft` and `tiagocarcao/antlr4-craft` to Docker Hub with multi-arch support. Simplify the setup flow by pulling the ANTLR image instead of building it locally. Add CI/CD via GitHub Actions.

---

## Part 1: DSL Revision

### Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Outer container keyword | Keep `domain { }` | Correct DDD: a domain is the problem space container |
| Inner items (user-visible) | No keyword change — bare identifiers | Syntax is already clean; renaming is internal |
| Service property | `domains:` → `contexts:` | Bounded contexts, not domains, are what a service owns |
| Exposure property | `of:` → `contexts:` | Consistency with service change; `of:` was ambiguous |
| Multi-domain shorthand | Keep `domains { }` | Correctly named — it lists multiple domains |
| Diagram labels | `[Context]` | Short, meaningful, accessible to non-DDD practitioners |
| Backward compatibility | Hard break + major version bump | Pre-1.0 project; clean break beats deprecation complexity |

### DSL Change Examples

**Service block:**
```
// Before
services {
  PaymentService {
    domains: PaymentProcessing, FraudDetection
  }
}

// After
services {
  PaymentService {
    contexts: PaymentProcessing, FraudDetection
  }
}
```

**Exposure block:**
```
// Before
exposure default {
  to: Business_User
  of: Authentication, Profile
  through: APIGateway
}

// After
exposure default {
  to: Business_User
  contexts: Authentication, Profile
  through: APIGateway
}
```

**Domain block (unchanged):**
```
domain User {
    Authentication
    Profile
    Preferences
}
```

### Scope of Changes

#### `craft` repo

- **ANTLR grammar** (`tools/antlr-grammar/Craft.g4`):
  - Rename grammar rule `subdomain` → `bounded_context`
  - Rename grammar rules `subdomain_list` → `bounded_context_list`, `domain_list` → `context_list`, `domain_ref` → `context_ref`
  - Replace lexer token `DOMAINS: 'domains'` → `CONTEXTS: 'contexts'`
  - Update `service_property` to use `CONTEXTS` token
  - Update `exposure_property` `of:` → `contexts:`
  - Regenerate parser via `make generate-grammar`

- **Go parser/visualizer** (`internal/`, `pkg/parser/`):
  - Update all references to subdomain/domain terminology in visualizer labels
  - Render bounded contexts with `[Context]` stereotype in generated diagrams

- **Example files** (`examples/*.craft`):
  - Replace `domains:` → `contexts:` in all service blocks
  - Replace `of:` → `contexts:` in all exposure blocks

- **`CLAUDE.md`**: Update setup command from `make docker-build-antlr-image generate-grammar` → `make docker-pull-antlr-image generate-grammar`. Update Podman note to reflect runtime auto-detection.

- **VitePress docs** (`docs/page/`):
  - `guide/introduction.md` — replace "subdomains" with "bounded contexts" in conceptual explanations
  - `guide/quickstart.md` — update any code snippets using `domains:` or `of:`
  - `guide/installation.md` — update setup steps (see Part 2)
  - `extension/features.md` — update DSL syntax references
  - `extension/installation.md` — update server setup instructions

#### `tree-sitter-craft` repo

- **`grammar.js`**:
  - Rename rule `subdomain` → `bounded_context` (line 236)
  - Rename `domain_definition` inner `subdomain` refs → `bounded_context`
  - Rename `domain_block` inner `subdomain` refs → `bounded_context`
  - Rename `domains_property` → `service_contexts_property`, change keyword `'domains'` → `'contexts'`
  - Rename `of_property` → `exposure_contexts_property`, change keyword `'of'` → `'contexts'`
  - These two rules have distinct names to avoid collision — they live in different parent rules (`service_property` and `exposure_property`) so the parser has no ambiguity. Both parse the same `contexts: <identifier_list>` syntax.
  - Rebuild wasm + native bindings
  - Bump npm package version (`0.1.5` → `0.2.0`) and publish to npm (`npm publish`)

#### `craft-vscode-extension` repo

- **Language server**: update any references to `subdomain` node type → `bounded_context`
- **Syntax queries** (`syntaxes/`): update node type references if querying `subdomain` or `domains_property`/`of_property` nodes
- **Update grammar dependency** to `tree-sitter-craft@0.2.0`

---

## Part 2: Docker Publishing

### Container Runtime Detection

The Makefile detects `docker` or `podman` at invocation time:

```makefile
CONTAINER_RUNTIME := $(shell command -v docker 2>/dev/null || command -v podman 2>/dev/null)
```

Both runtimes share identical CLI syntax for all operations used in this project (build, run, push, manifest). No branching needed beyond the binary name.

### Images

| Image | Registry | Purpose | Publish trigger |
|-------|----------|---------|-----------------|
| `tiagocarcao/craft` | Docker Hub | Craft server (runtime) | Git tag via GitHub Actions |
| `tiagocarcao/antlr4-craft:4.13.2` | Docker Hub | ANTLR grammar builder | Manual (`make docker-publish-antlr`) |

Both images target `linux/amd64` and `linux/arm64` using Docker Buildx with `--platform linux/amd64,linux/arm64`.

The ANTLR image has no automated CI publish — it is intentionally manual-only. It rarely changes (only when the ANTLR version or grammar tooling changes) and does not need to be part of the release cycle.

### ANTLR Setup Simplification

Current flow (fragile):
1. Clone `antlr4` repo from GitHub
2. Build ANTLR Docker image locally
3. Use local image to generate grammar

New flow:
1. Pull `tiagocarcao/antlr4-craft:4.13.2` from Docker Hub
2. Use it to generate grammar

The `docker-build-antlr-image` target is replaced with `docker-pull-antlr-image`. The git clone step is eliminated.

### Makefile Targets

Multi-arch builds in the Makefile use `docker buildx build --platform linux/amd64,linux/arm64 --push`. Buildx must be configured with a builder that supports multi-platform output (the default `docker-container` driver works).

```
Setup:
  fresh-setup              - Complete setup for new repository clones

Grammar:
  docker-pull-antlr-image  - Pull ANTLR builder image from Docker Hub
  docker-publish-antlr     - Build multi-arch ANTLR image and push to Docker Hub (maintainers only, rarely needed)
  generate-grammar         - Generate Go parser from ANTLR grammar

Docker (server):
  docker-build             - Build craft server image locally
  docker-run               - Run craft server container
  docker-publish           - Build multi-arch server image and push to Docker Hub (maintainers only)
  docker-clean             - Remove local craft server image
```

### GitHub Actions Workflow

File: `.github/workflows/publish.yml`

Trigger: push of a `v*` tag (e.g., `v2.0.0`)

Steps:
1. Checkout
2. Set up Go
3. Run `go test ./...` — publish aborted if tests fail
4. Set up Docker Buildx (multi-arch builder)
5. Login to Docker Hub using `DOCKERHUB_USERNAME` + `DOCKERHUB_TOKEN` secrets
6. Build and push `tiagocarcao/craft:$TAG` + `tiagocarcao/craft:latest` for `linux/amd64,linux/arm64`

### Required GitHub Secrets

| Secret | Value |
|--------|-------|
| `DOCKERHUB_USERNAME` | `tiagocarcao` |
| `DOCKERHUB_TOKEN` | Docker Hub access token (read/write) |

### VitePress Docs Updates (`guide/installation.md`)

Replace the ANTLR build step with a pull step. Update the quick-start Docker command to reference `tiagocarcao/craft:latest` for users who want to run without building locally:

```bash
docker run --rm -p 8080:8080 tiagocarcao/craft:latest
```

---

## Version Bump

The project is currently in active v2.0 development (no stable release yet). Both changes together warrant tagging the first stable release as **`v2.0.0`**. The breaking DSL change (`domains:` → `contexts:`, `of:` → `contexts:`) makes this a clean v2 baseline.

Pushing the `v2.0.0` git tag to `craft` triggers the GitHub Actions workflow and publishes the first `tiagocarcao/craft:v2.0.0` + `tiagocarcao/craft:latest` images to Docker Hub.

The `tree-sitter-craft` npm package bumps from `0.1.5` → `0.2.0` (breaking node type rename). The VS Code extension version bumps to match its grammar dependency update.
