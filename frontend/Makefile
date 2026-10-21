.PHONY: download-monaco-deps clean bump-patch bump-minor bump-major tag-release

MONACO_FILES = \
	public/wasm_exec.js \
	public/xray.schema.json \
	public/main.wasm

MONACO_BASE_URL = https://remnawave.github.io/xray-monaco-editor

download-monaco-deps: $(MONACO_FILES)

public/wasm_exec.js:
	@mkdir -p public
	curl -fsSL -o $@ $(MONACO_BASE_URL)/wasm_exec.js

public/xray.schema.json:
	@mkdir -p public 
	curl -fsSL -o $@ $(MONACO_BASE_URL)/xray.schema.json

public/main.wasm:
	@mkdir -p public
	curl -fsSL -o $@ $(MONACO_BASE_URL)/main.wasm

clean:
	rm -f $(MONACO_FILES)

bump-patch:
	bun pm version patch --no-git-tag-version
	bun install

bump-minor:
	bun pm version minor --no-git-tag-version
	bun install

bump-major:
	bun pm version major --no-git-tag-version
	bun install

tag-release:
	@VERSION=$$(bun -p "require('./package.json').version") && \
	echo "Creating signed tag for version $$VERSION..." && \
	git tag -s "v$$VERSION" -m "Release v$$VERSION" && \
	git push origin --follow-tags && \
	echo "Signed tag v$$VERSION created and pushed"
