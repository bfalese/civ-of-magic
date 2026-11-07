.PHONY: magic magic.wasm lbxdump wasm core-wasm test

magic:
	go mod tidy
	go build -o magic ./game/magic

wasm: magic.wasm

magic.wasm:
	env GOOS=js GOARCH=wasm go build -o magic.wasm ./game/magic

# the game core alone (no Ebitengine) for web frontends; test page: frontend/wasm/index.html
core-wasm:
	env GOOS=js GOARCH=wasm go build -o frontend/wasm/core.wasm ./frontend/wasm
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" frontend/wasm/

lbxdump:
	go build -o lbxdump ./util/lbxdump

update:
	go get -u ./game/magic
	go mod tidy

test:
	go test ./...
