.PHONY: build test check install install-hooks install-codex-hook install-claude-hook install-vicinae-extension check-vicinae-extension

build:
	mkdir -p bin
	go build -o bin/sesswitch ./cmd/sesswitch

test:
	go test ./...
	bash scripts/test-install-codex-hook.sh

check: test build
	go vet ./...
	./bin/sesswitch doctor

check-vicinae-extension:
	cd extensions/vicinae && npm run typecheck && npm run build -- --out "$(CURDIR)/bin/vicinae"

install-vicinae-extension:
	cd extensions/vicinae && npm ci && npm run typecheck && npm run build
	@echo 'If AI Sessions is missing, restart Vicinae: vicinae server --replace'

install: build
	mkdir -p "$(HOME)/.local/bin"
	ln -sf "$(CURDIR)/bin/sesswitch" "$(HOME)/.local/bin/sesswitch"

install-codex-hook: build
	bash scripts/install-codex-hook.sh

install-claude-hook: build
	bash scripts/install-claude-hook.sh

install-hooks: install-codex-hook install-claude-hook
