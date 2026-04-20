# Docker Publishing Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish `tiagocarcao/craft` and `tiagocarcao/antlr4-craft` to Docker Hub with multi-arch support (amd64 + arm64), replace the fragile ANTLR local build with a Docker Hub pull, add runtime auto-detection for docker/podman, and add a GitHub Actions workflow that runs tests and publishes on git tags.

**Architecture:** The Makefile gains a `CONTAINER_RUNTIME` variable that auto-detects `docker` or `podman`. Multi-arch builds use Docker Buildx (`--platform linux/amd64,linux/arm64`). The ANTLR image is pushed once manually; the craft server image is published automatically via GitHub Actions on `v*` tags with a test gate.

**Tech Stack:** Make, Docker Buildx / Podman, GitHub Actions, Go 1.22.

**Spec:** `docs/superpowers/specs/2026-04-20-dsl-revision-and-docker-publishing-design.md`

---

## Chunk 1: Makefile Overhaul

### Task 1: Add container runtime detection and replace ANTLR build with pull

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Read the current Makefile**

Read `/Users/tiago.carcao/projects/poc/craft-project/craft/Makefile` in full.

- [ ] **Step 2: Add runtime detection variable at the top**

After the existing variable declarations, add:
```makefile
# Auto-detect container runtime (docker preferred, fall back to podman)
CONTAINER_RUNTIME := $(shell command -v docker 2>/dev/null || command -v podman 2>/dev/null)
```

Then replace every hardcoded `podman` command in the Makefile with `$(CONTAINER_RUNTIME)`. Affected targets: `docker-build`, `docker-run`, `docker-clean`, `docker-build-antlr-image`, `docker-clean-antlr-image`, `generate-grammar`, and any compose targets if present.

- [ ] **Step 3: Replace docker-build-antlr-image with docker-pull-antlr-image**

Remove the current `docker-build-antlr-image` target entirely and add:
```makefile
docker-pull-antlr-image:
	@echo "Pulling ANTLR builder image..."
	$(CONTAINER_RUNTIME) pull $(ANTLR_IMAGE_NAME):$(ANTLR_IMAGE_TAG)
	@echo "✅ ANTLR image ready."
```

Update `generate-grammar` to depend on `docker-pull-antlr-image` instead of `docker-build-antlr-image`:
```makefile
generate-grammar: docker-pull-antlr-image
	mkdir -p $(shell pwd)/$(ANTLR_GRAMMAR_PATH)
	mkdir -p $(shell pwd)/$(GOLANG_GRAMMAR_PATH)
	$(CONTAINER_RUNTIME) run --platform linux/amd64 --rm \
		-v $(shell pwd)/$(ANTLR_GRAMMAR_PATH):/work \
		-v $(shell pwd)/$(GOLANG_GRAMMAR_PATH):/output \
		-w /work $(ANTLR_IMAGE_NAME):$(ANTLR_IMAGE_TAG) \
		-Dlanguage=Go -visitor -o /output $(ANTLR_GRAMMAR_FILENAME)
```

Update `ANTLR_IMAGE_NAME` from `antlr/antlr4` to `tiagocarcao/antlr4-craft`:
```makefile
ANTLR_IMAGE_NAME=tiagocarcao/antlr4-craft
```

- [ ] **Step 4: Update fresh-setup to use docker-pull-antlr-image**

The `fresh-setup` target currently calls `generate-grammar` which now pulls automatically. Update `fresh-setup` output message to reflect that setup now pulls from Docker Hub rather than building locally. Remove any references to building the ANTLR image.

---

### Task 2: Add multi-arch publish targets

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add Docker Hub variables and make IMAGE_TAG overridable**

Add near the top of the Makefile:
```makefile
DOCKERHUB_USER=tiagocarcao
```

Verify `ANTLR_IMAGE_TAG` is set to `4.13.2` in the Makefile (it should already be there). If missing, add:
```makefile
ANTLR_IMAGE_TAG=4.13.2
```

Change `IMAGE_TAG=latest` to use `?=` so callers can override it:
```makefile
IMAGE_TAG ?= latest
```
This lets `make docker-publish IMAGE_TAG=v2.0.0` work correctly. The GitHub Actions workflow passes the tag via its own mechanism, but for manual local publishes the caller must set `IMAGE_TAG`.

- [ ] **Step 2: Add docker-publish target**

The `docker-publish` and `docker-publish-antlr` targets use Docker Buildx which **requires Docker, not Podman** (podman has no `buildx` subcommand). Add a guard at the top of each target:

```makefile
docker-publish:
	@command -v docker >/dev/null 2>&1 || \
		(echo "❌ docker-publish requires Docker with Buildx. Podman is not supported for multi-arch publish." && exit 1)
	@echo "Building and pushing multi-arch craft server image..."
	@echo "Image: tiagocarcao/craft:$(IMAGE_TAG)"
	@echo "Tip: set IMAGE_TAG to override (e.g. make docker-publish IMAGE_TAG=v2.0.0)"
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--push \
		-t $(DOCKERHUB_USER)/craft:$(IMAGE_TAG) \
		-t $(DOCKERHUB_USER)/craft:latest \
		-f $(DOCKERFILE_PATH) .
	@echo "✅ Published $(DOCKERHUB_USER)/craft:$(IMAGE_TAG)"
```

Note: Buildx requires a multi-platform builder. Create one if not already done:
```bash
docker buildx create --use --name multiarch --driver docker-container
```

- [ ] **Step 3: Add docker-publish-antlr target**

The ANTLR image is published manually and rarely. Also Docker-only for the same reason:

```makefile
docker-publish-antlr:
	@command -v docker >/dev/null 2>&1 || \
		(echo "❌ docker-publish-antlr requires Docker with Buildx. Podman is not supported for multi-arch publish." && exit 1)
	@echo "Building and pushing multi-arch ANTLR builder image..."
	@echo "This is a manual step — only needed when ANTLR tooling changes."
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--push \
		-t $(DOCKERHUB_USER)/antlr4-craft:$(ANTLR_IMAGE_TAG) \
		-t $(DOCKERHUB_USER)/antlr4-craft:latest \
		-f build/package/antlr.Dockerfile .
	@echo "✅ Published $(DOCKERHUB_USER)/antlr4-craft:$(ANTLR_IMAGE_TAG)"
```

Note: This requires `build/package/antlr.Dockerfile` — see Task 3.

- [ ] **Step 4: Update .PHONY and help target**

First, remove `docker-build-antlr-image` from any existing `.PHONY` line (the target is being replaced by `docker-pull-antlr-image`):
```bash
grep -n "docker-build-antlr-image" /Users/tiago.carcao/projects/poc/craft-project/craft/Makefile
```
Delete it from whatever `.PHONY` line it appears in before writing the new one.

Replace `.PHONY` with the complete list including all new targets:
```makefile
.PHONY: build run test clean fresh-setup docker-build docker-run docker-clean \
        docker-pull-antlr-image docker-clean-antlr-image generate-grammar \
        docker-publish docker-publish-antlr compose-build compose-up \
        compose-up-detached compose-down compose-logs compose-restart help
```

Update the `help` target:
```makefile
help:
	@echo "Makefile commands:"
	@echo ""
	@echo "Setup:"
	@echo "  fresh-setup              - Complete setup for new repository clones"
	@echo ""
	@echo "Docker (server):"
	@echo "  docker-build             - Build craft server image locally"
	@echo "  docker-run               - Run craft server container"
	@echo "  docker-clean             - Remove local craft server image"
	@echo "  docker-publish           - Build multi-arch image and push to Docker Hub (Docker only)"
	@echo "                             Usage: make docker-publish IMAGE_TAG=v2.0.0"
	@echo ""
	@echo "Grammar:"
	@echo "  docker-pull-antlr-image  - Pull ANTLR builder image from Docker Hub"
	@echo "  docker-publish-antlr     - Build and push ANTLR builder image (Docker only, maintainers only)"
	@echo "  docker-clean-antlr-image - Remove local ANTLR builder image"
	@echo "  generate-grammar         - Generate Go parser from ANTLR grammar"
	@echo ""
	@echo "Testing:"
	@echo "  test                     - Run all tests"
```

- [ ] **Step 5: Verify Makefile syntax**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && make help
```
Expected: help output displays correctly with all new targets.

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && make --dry-run docker-build
```
Expected: shows the correct runtime command (docker or podman based on what's installed).

- [ ] **Step 6: Commit**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft
git add Makefile
git commit -m "feat(makefile): add runtime detection, pull-based ANTLR setup, and multi-arch publish targets

- Auto-detect docker or podman via CONTAINER_RUNTIME
- Replace fragile antlr image build with docker-pull-antlr-image
- Add docker-publish for multi-arch server image (amd64 + arm64)
- Add docker-publish-antlr for ANTLR builder image (manual, rarely needed)
- ANTLR image now pulled from tiagocarcao/antlr4-craft instead of built locally"
```

---

## Chunk 2: ANTLR Dockerfile + GitHub Actions

### Task 3: Create the ANTLR Dockerfile

**Files:**
- Create: `build/package/antlr.Dockerfile`

The ANTLR builder image needs to run the ANTLR tool. Currently the project clones the antlr4 repo and builds the Docker image from it. We need to create a standalone Dockerfile that packages the ANTLR jar.

- [ ] **Step 1: Check what the original antlr4 Docker image does**

Look at the existing `docker-build-antlr-image` target in git history to understand what it built:
```bash
git -C /Users/tiago.carcao/projects/poc/craft-project/craft log --oneline --all | head -5
git -C /Users/tiago.carcao/projects/poc/craft-project/craft show HEAD:Makefile | grep -A 10 "docker-build-antlr-image"
```

The original image cloned `https://github.com/antlr/antlr4.git` and built from `antlr4/docker/`. The official ANTLR4 image runs `java -jar antlr4.jar` on a grammar file.

- [ ] **Step 2: Create build/package/antlr.Dockerfile**

```dockerfile
FROM eclipse-temurin:11-jre-alpine

WORKDIR /work

# Download the ANTLR 4.13.2 jar
RUN apk add --no-cache wget && \
    wget -q "https://github.com/antlr/antlr4/releases/download/4.13.2/antlr-4.13.2-complete.jar" \
         -O /usr/local/lib/antlr.jar && \
    apk del wget

ENTRYPOINT ["java", "-jar", "/usr/local/lib/antlr.jar"]
```

This is smaller than the original (alpine + JRE only) and doesn't require cloning the antlr4 repo.

- [ ] **Step 3: Test the Dockerfile locally**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft
docker build -f build/package/antlr.Dockerfile -t tiagocarcao/antlr4-craft:test .
docker run --rm tiagocarcao/antlr4-craft:test -version
```
Expected: prints `ANTLR Tool version 4.13.2`.

- [ ] **Step 4: Test grammar generation with the new image**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft
docker run --platform linux/amd64 --rm \
  -v $(pwd)/tools/antlr-grammar:/work \
  -v $(pwd)/pkg/parser:/output \
  -w /work tiagocarcao/antlr4-craft:test \
  -Dlanguage=Go -visitor -o /output Craft.g4
```
Expected: Go parser files generated in `pkg/parser/`.

- [ ] **Step 5: Commit the Dockerfile**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft
git add build/package/antlr.Dockerfile
git commit -m "feat: add standalone ANTLR builder Dockerfile for tiagocarcao/antlr4-craft"
```

---

### Task 4: Publish the ANTLR image to Docker Hub

This is a one-time manual step. Requires Docker Hub login.

- [ ] **Step 1: Log in to Docker Hub**

```bash
docker login
```
Enter username `tiagocarcao` and your Docker Hub access token when prompted.

- [ ] **Step 2: Set up Buildx for multi-arch (if not already done)**

```bash
docker buildx create --use --name multiarch --driver docker-container
docker buildx inspect --bootstrap
```
Expected: builder shows `linux/amd64` and `linux/arm64` platforms.

- [ ] **Step 3: Publish the ANTLR image**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && make docker-publish-antlr
```
Expected: image pushed as `tiagocarcao/antlr4-craft:4.13.2` and `tiagocarcao/antlr4-craft:latest`.

- [ ] **Step 4: Verify the pull works**

```bash
docker pull tiagocarcao/antlr4-craft:4.13.2
docker run --rm tiagocarcao/antlr4-craft:4.13.2 -version
```
Expected: `ANTLR Tool version 4.13.2`.

---

### Task 5: Create GitHub Actions publish workflow

**Files:**
- Create: `.github/workflows/publish.yml`

- [ ] **Step 1: Check existing GitHub Actions workflows**

```bash
ls /Users/tiago.carcao/projects/poc/craft-project/craft/.github/workflows/
```

- [ ] **Step 2: Create .github/workflows/publish.yml**

```yaml
name: Publish Docker Image

on:
  push:
    tags:
      - 'v*'

jobs:
  test-and-publish:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Run tests
        run: go test ./...

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to Docker Hub
        uses: docker/login-action@v3
        with:
          username: ${{ secrets.DOCKERHUB_USERNAME }}
          password: ${{ secrets.DOCKERHUB_TOKEN }}

      - name: Extract tag name
        id: tag
        run: echo "TAG=${GITHUB_REF#refs/tags/}" >> $GITHUB_OUTPUT

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          file: build/package/Dockerfile
          platforms: linux/amd64,linux/arm64
          push: true
          tags: |
            tiagocarcao/craft:${{ steps.tag.outputs.TAG }}
            tiagocarcao/craft:latest
```

- [ ] **Step 3: Add DOCKERHUB_USERNAME and DOCKERHUB_TOKEN to GitHub repo secrets**

In the GitHub UI for the `craft` repo:
1. Go to Settings → Secrets and variables → Actions
2. Add `DOCKERHUB_USERNAME` = `tiagocarcao`
3. Add `DOCKERHUB_TOKEN` = your Docker Hub access token (create one at hub.docker.com → Account Settings → Security)

This must be done manually in the GitHub UI — it cannot be automated.

- [ ] **Step 4: Commit the workflow**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft
git add .github/workflows/publish.yml
git commit -m "ci: add GitHub Actions workflow to publish Docker image on version tags

Trigger: push of v* tags
Steps: go test → buildx multi-arch build → push to tiagocarcao/craft"
```

---

### Task 6: Update CLAUDE.md and installation docs

**Files:**
- Modify: `CLAUDE.md`
- Modify: `docs/page/guide/installation.md`

- [ ] **Step 1: Update CLAUDE.md setup command**

Read `CLAUDE.md`. In the "Grammar Generation" section, update:

```diff
-# Generate ANTLR parser code and VS Code extension grammar
-make docker-build-antlr-image generate-grammar
+# Generate ANTLR parser code and VS Code extension grammar
+make generate-grammar
+# (automatically pulls tiagocarcao/antlr4-craft:4.13.2 from Docker Hub)
```

Also update the "Container Requirements" section:
```diff
-## Container Requirements
-- **Podman**: Used instead of Docker for container operations
-- **ANTLR4 Docker Image**: Built locally for grammar generation
+## Container Requirements
+- **Docker or Podman**: Auto-detected — whichever is available is used
+- **ANTLR4 Docker Image**: Pulled from `tiagocarcao/antlr4-craft:4.13.2` (no local build needed)
```

- [ ] **Step 2: Update docs/page/guide/installation.md**

Read the file. Replace the ANTLR image build step:
```diff
-# Build the ANTLR Docker image (required once)
-make docker-build-antlr-image
+# No setup needed — the ANTLR image is pulled automatically
```

Add a section for running from Docker Hub directly (for non-developers):
```markdown
## Quick Start (Docker Hub)

Run the latest Craft server without building locally:

```bash
docker run --rm -p 8080:8080 tiagocarcao/craft:latest
```

Then configure your VS Code extension to point to `http://localhost:8080`.
```

- [ ] **Step 3: Commit**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft
git add CLAUDE.md docs/page/guide/installation.md
git commit -m "docs: update setup instructions for Docker Hub pull-based ANTLR and quick-start"
```

---

### Task 7: Trigger first release and verify end-to-end

- [ ] **Step 1: Push all commits**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft && git push
```

- [ ] **Step 2: Verify GitHub Actions secrets are set**

In the GitHub UI, confirm `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` are present under Settings → Secrets → Actions.

- [ ] **Step 3: Create and push the v2.0.0 tag**

```bash
cd /Users/tiago.carcao/projects/poc/craft-project/craft
git tag v2.0.0
git push origin v2.0.0
```

- [ ] **Step 4: Monitor the GitHub Actions run**

Go to the `craft` repo on GitHub → Actions tab. The `Publish Docker Image` workflow should start within seconds. Monitor all steps: tests → buildx → push.

Expected: workflow completes successfully. If tests fail, fix and retag.

- [ ] **Step 5: Verify the published image**

```bash
docker pull tiagocarcao/craft:v2.0.0
docker run --rm -p 8080:8080 tiagocarcao/craft:v2.0.0 &
curl -s http://localhost:8080/health || curl -s http://localhost:8080/
```
Expected: server responds. Kill the container after verification.

- [ ] **Step 6: Verify fresh setup works end-to-end**

```bash
cd /tmp && git clone https://github.com/tcarcao/craft.git craft-test && cd craft-test
make fresh-setup
```
Expected: pulls ANTLR image from Docker Hub, generates grammar — no git clone of antlr4 repo.

```bash
cd /tmp && rm -rf craft-test
```

---

*End of Plan 2: Docker Publishing*
