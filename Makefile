# sitecheck: build, test, install, release archives.
PREFIX     ?= $(HOME)/.local/bin
SKILLS_DIR ?= $(HOME)/.claude/skills
PLATFORMS  := darwin/arm64 darwin/amd64 linux/amd64 linux/arm64
LDFLAGS    := -s -w

.PHONY: build test install skill dist clean

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o build/sitecheck ./cmd/sitecheck

test:
	go vet ./...
	go test ./...

install: build
	mkdir -p $(PREFIX)
	cp build/sitecheck $(PREFIX)/

# For working on the skill: symlink it into ~/.claude/skills so edits apply
# at once. Users install the plugin instead (see README).
skill:
	mkdir -p $(SKILLS_DIR)
	ln -sfn "$(CURDIR)/skills/site-check" "$(SKILLS_DIR)/site-check"

# Release archives: dist/sitecheck-<os>-<arch>.tar.gz plus .sha256 files.
dist: clean
	mkdir -p dist
	for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; d=dist/sitecheck-$$os-$$arch; mkdir -p $$d; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" -o $$d/sitecheck ./cmd/sitecheck || exit 1; \
		cp LICENSE $$d/; tar -czf $$d.tar.gz -C $$d sitecheck LICENSE; \
		(cd dist && shasum -a 256 sitecheck-$$os-$$arch.tar.gz > sitecheck-$$os-$$arch.tar.gz.sha256); \
	done
	find dist -mindepth 1 -type d -exec rm -r {} +

clean:
	rm -rf build dist
