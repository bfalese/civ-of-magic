# CLAUDE.md

Guidance for Claude (and humans) working in this repo.

## What this is

**civ-of-magic**: a Civilization-style 4X game, forked from
[kazzmir/master-of-magic](https://github.com/kazzmir/master-of-magic) (Go + Ebitengine, BSD-3).
The goal is to keep MoM's retro feel while growing it into a Civ-like game with new concepts.

Long-term direction:
- **Decouple** game logic (pure Go "core") from rendering (swappable frontends).
- **Drop LBX assets** — move to standard formats (PNG atlases + YAML data).
- **Visual style:** MoM retro look over 3D (low-res render, ~32-color palette, Bayer dither,
  depth outlines, sprite units), eventually via a Three.js frontend talking to the Go core over wasm.

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
  `combat`, `data`, `game` (the game model), `hero`, `maplib`, `neural`, `pathfinding`, `player`, `setup`,
  `spellbook`, `terrain`, `units`.
  - Import `core/game` as `gamecore`: `gamelib` is the established alias of the UI package `game/magic/game`.
  - LBX parsers live in `game/magic/lbxdata`; drawing/screens in `game/magic/*view`
    (`mapview`, `combatview`, `spellbookview`, `artifactview`, `unitview`, `setupview`, ...).
  - Frontend-owned state: `maplib.Map.RenderCache` (opaque to core, filled by `mapview`).
    `mapview.Renderer` wraps a `*maplib.Map` with draw methods for code that wants a map that draws itself.
- `game/magic/` — the main game. `main.go` + one package per subsystem.
  - `game/game.go` (~8.6k lines) is the main UI/state tangle; the model it drives is `core/game`.
    Events: core defines `GameEvent` and the events the model emits; the UI package aliases
    `GameEvent` and adds its own (screen requests, UI commands) on the same channel.
  - `core/combat` (model, AI, networking, headless runner) vs `combatview` (screens) is the template
    for further splits.
  - Pure UI packages: `*view`, `ui`, `draw`, `mouse`, `inputmanager`, `shaders`, `camera`, etc.
- `game/arena`, `game/network-combat` — side programs.
- `lib/` — loaders (lbx, fonts, midi…), vendored libs.
- `ebitenui/` — vendored fork, wired via `replace` in `go.mod`.
- `util/` — dev tools (sprite viewer, lbxdump, combat-simulator).
- `test/` — small standalone test programs.

## Architecture rules (target)

- `core/` must never import Ebitengine, ebitenui, or LBX loaders. Enforce with:
  `go list -deps ./core/... | grep -E 'ebiten|/lbx' && exit 1`
- Core talks to frontends through **commands in / events out** (e.g. "move unit X to Y" → "unit moved").
  Existing seam to build on: `Events chan GameEvent` in `game/game.go`.
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
