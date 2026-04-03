BINARY  := skill/timetracker
GODIR   := go-src
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v1.0.0")

.PHONY: build test install release

## Build the timetracker binary into skill/
build:
	cd $(GODIR) && go build -o ../$(BINARY) .

## Run all Go tests
test:
	cd $(GODIR) && go test ./...

## Developer install: build + symlink skill/ into Claude Code (and opencode if installed)
install: build
	@bash install.sh --dev

## Tag and push a release — GitHub Actions builds and publishes it
## Usage: make release VERSION=v1.2.0
release:
	@[ -n "$(VERSION)" ] || (echo "Usage: make release VERSION=v1.x.x" && exit 1)
	git tag $(VERSION)
	git push origin $(VERSION)
	@echo "✓ Tagged $(VERSION) — GitHub Actions will build and publish the release."
