.PHONY: download-monaco-deps clean bump-patch bump-minor bump-major tag-release

# Xray validator runtime and schemas, served from /assets/ (see scripts/editor-assets.cjs).
download-monaco-deps:
	bun scripts/editor-assets.cjs download public/assets

clean:
	rm -f public/assets/main.wasm public/assets/wasm_exec.js public/assets/xray.schema.json public/assets/xray.schema.cn.json

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
