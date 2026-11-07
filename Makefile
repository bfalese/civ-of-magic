.PHONY: magic magic.wasm lbxdump wasm core-wasm itch.io test

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

itch.io: wasm
	cp magic.wasm itch.io
	butler push itch.io kazzmir/magic:html

combat-simulator-itch.io:
	env GOOS=js GOARCH=wasm go build -o ./util/combat-simulator/itch.io/combat-simulator.wasm ./util/combat-simulator
	butler push ./util/combat-simulator/itch.io kazzmir/magic-combat-simulator:html

arena-itch.io:
	env GOOS=js GOARCH=wasm go build -o ./game/arena/itch.io/arena.wasm ./game/arena
	butler push ./game/arena/itch.io kazzmir/battle-arena:html

network-combat-itch.io:
	env GOOS=js GOARCH=wasm go build -o ./game/network-combat/itch.io/network-combat.wasm ./game/network-combat
	butler push ./game/network-combat/itch.io kazzmir/magic-combat-multiplayer:html

update:
	go get -u ./game/magic
	go mod tidy

test:
	go test ./...
