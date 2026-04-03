BINARY  := skill/timetracker
GODIR   := go-src
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v1.0.0")
TARBALL := time-entry-skill-darwin-arm64.tar.gz

.PHONY: build test install release

## Build the timetracker binary into skill/
build:
	cd $(GODIR) && go build -o ../$(BINARY) .

## Run all Go tests
test:
	cd $(GODIR) && go test ./...

## Developer install: build + symlink skill/ as the Claude Code skill dir
install: build
	@bash install.sh --dev

## Create a GitHub release with a tarball of skill files + binary
## Usage: make release VERSION=v1.2.0
release: build
	@[ -n "$(VERSION)" ] || (echo "Usage: make release VERSION=v1.x.x" && exit 1)
	@echo "→ Creating release tarball $(TARBALL)..."
	tar -czf $(TARBALL) \
		-C skill \
		SKILL.md export.py tt.py pull_calendar.swift timetracker
	@echo "→ Tagging $(VERSION)..."
	git tag $(VERSION)
	git push origin $(VERSION)
	@echo "→ Publishing GitHub release $(VERSION)..."
	gh release create $(VERSION) $(TARBALL) \
		--title "$(VERSION)" \
		--notes "Install: \`curl -fsSL https://raw.githubusercontent.com/kajigga/ai-ticktock/main/install.sh | bash\`"
	rm -f $(TARBALL)
	@echo "✓ Release $(VERSION) published."
