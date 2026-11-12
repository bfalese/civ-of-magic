# CLAUDE.md

Guidance for Claude (and humans) working in this repo.

## What this is

**civ-of-magic**: a Civilization-style 4X game, forked from
[kazzmir/master-of-magic](https://github.com/kazzmir/master-of-magic) (Go + Ebitengine, BSD-3).
The goal is to keep MoM's retro feel while growing it into a Civ-like game with new concepts.

Long-term direction:
- **Decouple** game logic (pure Go "core") from rendering (swappable frontends).
- **Drop LBX assets** — move to standard formats (PNG atlases + YAML data).
- **Visual style:** flat-polygon 3D in the spirit of *Another World* (faceted geometry, one colour per
  face from a small per-plane palette, light in a few flat tones, strong silhouettes), in the Three.js
  frontend talking to the Go core over wasm. 3D models come from a ComfyUI text/image-to-GLB
  workflow, normalised to match (decimated, flat colours).

The current work plan lives in `docs/handoff/` (latest handoff first). Read it before starting.

## Build / run / test

```sh
go build -o magic ./game/magic      # or: make
go run ./game/magic                 # needs MoM LBX data for now (see README.md)
go vet ./...
go test ./core/... ./lib/... ./game/...   # CI runs this under xvfb-run on Linux
make wasm                           # GOOS=js GOARCH=wasm build
```

- Go `>= 1.27.0` (see `go.mod`).
- Linux deps for Ebitengine: `libasound2-dev xorg-dev` (+ `xvfb` for tests headless).
- Module path: `github.com/bfalese/civ-of-magic`.

## Layout

- `core/` — rendering-free, LBX-free game logic (CI-enforced): `ai`, `artifact`, `building`, `city`,
  `combat`, `data`, `assets` (asset IDs), `game` (the game model), `gamedata` (data tables as JSON), `hero`, `maplib`, `neural`,
  `pathfinding`, `player`, `setup`, `spellbook`, `terrain`, `units`.
  - `core/game/command.go`: player actions as commands (`GameModel.Execute`, JSON via `EncodeCommand` /
    `DecodeCommand`). `core/game/headless.go`: an AI-vs-AI game without a frontend.
  - Import `core/game` as `gamecore`: `gamelib` is the established alias of the UI package `game/magic/game`.
  - LBX parsers live in `game/magic/lbxdata`; drawing/screens in `game/magic/*view`
    (`mapview`, `combatview`, `spellbookview`, `artifactview`, `unitview`, `setupview`, ...).
  - Frontend-owned state: `maplib.Map.RenderCache` (opaque to core, filled by `mapview`).
    `mapview.Renderer` wraps a `*maplib.Map` with draw methods for code that wants a map that draws itself.
- `game/magic/` — the main game. `main.go` + one package per subsystem.
  - `game/game.go` (~6k lines) is now mostly screens; the model it drives is `core/game`. UI steps the
    model needs (dialogs, animations, battles) are passed in as hooks/handler interfaces.
    Events: core defines `GameEvent` and the events the model emits; the UI package aliases
    `GameEvent` and adds its own (screen requests, UI commands) on the same channel.
  - `core/combat` (model, AI, networking, headless runner) vs `combatview` (screens) is the template
    for further splits.
  - Pure UI packages: `*view`, `ui`, `draw`, `mouse`, `inputmanager`, `shaders`, `camera`, etc.
- `game/arena`, `game/network-combat` — side programs.
- `lib/` — loaders (lbx, fonts, midi…), vendored libs.
- `ebitenui/` — vendored fork, wired via `replace` in `go.mod`.
- `util/` — dev tools (sprite viewer, lbxdump, combat-simulator).
  - `util/export-assets`: writes the overland sprites as PNG sheets + `assets.json` keyed by asset IDs
    (`core/assets`), for web frontends; output outside the repo like export-gamedata's.
  - `util/export-gamedata`: writes the data tables from an original install to JSON (keep the output
    out of the repo, it's derived from copyrighted data). `util/headless-game`: plays AI vs AI with
    that data, no Ebitengine (`go run ./util/headless-game -years 200`), a quick regression run.
- `frontend/` — web frontends over the wasm core: `frontend/wasm` (the `GOOS=js` entry point exposing
  `civ.*`, plus a bare protocol test page), `frontend/web` (the game in the browser: Three.js map,
  isometric 3D by default (`map3d.js`: the terrain on a ground plane with height by terrain type, cities, stacks and lairs as
  billboards; Q/E turn the view a quarter turn; `?terrain=simple|polygon` or the V key (cycles textured, simple, polygon) for plain colours per terrain with shaped hills and mountains, carved rivers, a water plane (`terrain-simple.js`) instanced trees (`vegetation.js`) and animated node crystals (`nodes3d.js`); `?view=2d` for the flat `map.js`), sprites from util/export-assets). Run with `go run ./util/serve-web` (builds the core, serves the
  page and the user's exported data) and open http://localhost:8080. Browser smoke test (Playwright):
  `python3 frontend/web/smoke_test.py` against a running serve-web.
- `test/` — small standalone test programs.

## Architecture rules (target)

- `core/` must never import Ebitengine, ebitenui, or LBX loaders. Enforce with:
  `go list -deps ./core/... | grep -E 'ebiten|/lbx' && exit 1`
- Core talks to frontends through **commands in / events out** (e.g. "move unit X to Y" → "unit moved"):
  `core/game/command.go` in, `Events chan GameEvent` (events in `core/game/gameevents.go`) out.
  The JSON form of commands, events and views is described in `docs/frontend-protocol.md`.
- Sprites/images referenced from core become **asset IDs** (strings), resolved by the frontend.
- New logic should be testable without an `lbx.LbxCache`.

## Code style (inherited, keep it)

- **No gofmt.** 4-space indentation; match surrounding code.
- Import groups: stdlib, then this module, then 3rd party (ebiten etc.).
- Spell variable names out; only `x, y, z, i, j, err` as short names.
- Return errors instead of `panic`.
- Avoid new dependencies; prefer vendoring small code into `lib/`.

## Workflow

- Branch per change, squash-merge into `master`, keep branches short-lived.
- Large refactors: do mechanical moves in their own commits (move only, no logic changes;
  helpers in `scripts/refactor/`),
  and make sure `go build ./... && go test ./core/... ./lib/... ./game/...` passes after each.
- Upstream (kazzmir) will diverge quickly once folders move; cherry-pick fixes if needed, don't merge.
- Write a new `docs/handoff/NNN-*.md` at the end of a work session: what changed, what's next, open questions.
