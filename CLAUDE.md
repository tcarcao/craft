# Craft — CLAUDE.md

> Agent instructions for the **Craft** repository (`github.com/tcarcao/craft`).

## Project Overview

Craft is a domain-specific language (DSL) for modeling business use cases and domain interactions. It generates visual diagrams (C4, domain-flow, sequence) from `.craft` source files.

**Tech Stack:** Go 1.23, Docker / Podman, GitHub Actions.

---

## Repository Layout

```
cmd/
  server/       HTTP server (diagram generation API)
  craft/        CLI entry point
internal/
  ast/          v2 AST — pure data types
  lexer/        Hand-written scanner
  syntax/       Hand-written recursive-descent parser → ast
  sema/         Symbol tables, ResolutionMap, validation + lint rules
  workspace/    Multi-file store, per-file parse cache, URI index
  lsp/          LSP protocol handlers
  visualizer/   PlantUML / C4 diagram generators
  processor/    .craft file processing pipeline
  parser_diff/  Differential harness (Harness A + Harness C)
pkg/
  craft/        Stable public Go API — CraftDoc canonical type
build/
  package/
    Dockerfile          Multi-stage server image
.github/
  workflows/
    deploy.yml    Deploy VitePress docs to GitHub Pages
    publish.yml   Build & push tiagocarcao/craft to Docker Hub on v* tags
docs/
  page/         VitePress documentation site
  decisions/    Architecture decision records
testdata/
  corpus/       Acceptance corpus (.craft + .craftjson pairs)
  broken/       Intentionally broken files + expected .diagnostics.json
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
| `context_map` | Strategic view classifying BC-to-BC relationships with DDD patterns (`customer_supplier`, `conformist`, `anticorruption_layer`, `open_host_service`, `published_language`, `partnership`, `shared_kernel`, `separate_ways`) |
| `glossary`    | Ubiquitous-language view relating terms across bounded contexts with three symmetric verbs (`same_as`, `contrasts`, `distinct_from`) |

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

### `context_map` Example

```craft
context_map Monetization {
  Wallet open_host_service WalletItemPurchase
  OrderManagement customer_supplier OrderFulfilment
}
```

Endpoints are bare bounded-context names (no `bc:` prefix); `LEFT` is always the upstream side. See `docs/page/language/context-map.md` for the full pattern catalog and validation rules.

### `glossary` Example

```craft
glossary Monetization {
  Wallet/balance same_as WalletItemPurchase/balance
  Wallet/balance distinct_from OrderManagement/balance
}
```

Term nodes are `<bc>/<term>` or `<domain>/<bc>/<term>` — the last `/`-segment is the term, the rest is the bounded-context reference. All three verbs are symmetric (no upstream/downstream). See `docs/page/language/glossary.md` for term-node identity and the full diagnostics table.

---

## Container Requirements

- **Docker or Podman**: Auto-detected — whichever is available is used (`CONTAINER_RUNTIME` in Makefile)

---

## Common Commands

```bash
# Run tests
make test

# Build and run the server container
make docker-build && make docker-run

# Multi-arch publish to Docker Hub (maintainers only, requires Docker + Buildx)
make docker-publish IMAGE_TAG=v2.0.0
```

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

---

## VS Code Extension

The Craft VS Code extension is a separate repository:
🔗 **[craft-vscode-extension](https://github.com/tcarcao/craft-vscode-extension)**

See `docs/decisions/lsp-migration-plan.md` and `docs/AGENT.md` for operating rules.
