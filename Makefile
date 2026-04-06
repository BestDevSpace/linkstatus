# All Go builds run in Docker only (no host go/gcc). Cross-compile with GOOS/GOARCH.
GO_DOCKER := golang:1.23-bookworm
# Use a single Linux platform for the builder; pure Go cross-compiles to linux/darwin from any arch.
DOCKER_BUILD := docker run --rm -e CGO_ENABLED=0 -v $$PWD:/app -w /app --platform linux/amd64 $(GO_DOCKER)

.PHONY: bin bin-linux bin-darwin bin-simple clean mod-tidy test lint goreleaser-check

# One line so it expands safely inside sh -c.
APT_GIT := apt-get update -qq && apt-get install -y -qq git >/dev/null

mod-tidy:
	$(DOCKER_BUILD) sh -c '$(APT_GIT) && go mod tidy'

# Linux only (amd64 + arm64).
bin-linux:
	@mkdir -p output
	@$(DOCKER_BUILD) sh -c '$(APT_GIT) && \
		GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o output/linkstatus-linux-amd64 . && \
		GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o output/linkstatus-linux-arm64 .'

# macOS only (amd64 + arm64); built inside Linux container via cross-compile.
bin-darwin:
	@mkdir -p output
	@$(DOCKER_BUILD) sh -c '$(APT_GIT) && \
		GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o output/linkstatus-darwin-amd64 . && \
		GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o output/linkstatus-darwin-arm64 .'

# All four release binaries in one container (no host toolchain).
bin:
	@mkdir -p output
	@$(DOCKER_BUILD) sh -c '$(APT_GIT) && \
		GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o output/linkstatus-linux-amd64 . && \
		GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o output/linkstatus-linux-arm64 . && \
		GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o output/linkstatus-darwin-amd64 . && \
		GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o output/linkstatus-darwin-arm64 .'

bin-simple: bin

test:
	$(DOCKER_BUILD) sh -c '$(APT_GIT) && go mod download && go test ./...'

lint:
	docker run --rm -v $$PWD:/app -w /app golangci/golangci-lint golangci-lint run -v

clean:
	rm -rf dist output

# Validate .goreleaser.yaml (requires Docker; same image family as CI).
goreleaser-check:
	docker run --rm -v $$PWD:/src -w /src goreleaser/goreleaser:latest check
