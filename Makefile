VAULT ?= $(shell echo $$MYMIND_VAULT)
PLUGIN_DIR = $(VAULT)/.obsidian/plugins/mymind

.PHONY: build install dev clean

build:
	npx esbuild src/main.ts --bundle --external:obsidian --external:electron --format=cjs --target=es2018 --outfile=main.js

install: build
	@test -n "$(VAULT)" || (echo "error: set MYMIND_VAULT"; exit 1)
	mkdir -p "$(PLUGIN_DIR)"
	cp main.js manifest.json "$(PLUGIN_DIR)/"
	@echo "Installed to $(PLUGIN_DIR)"

dev:
	npx esbuild src/main.ts --bundle --external:obsidian --external:electron --format=cjs --target=es2018 --outfile=main.js --sourcemap=inline --watch

clean:
	rm -f main.js
