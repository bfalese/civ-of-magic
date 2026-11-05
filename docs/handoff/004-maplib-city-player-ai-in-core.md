# Handoff 004 — all of 001's "model" packages are in core/

Date: 2026-11-05
Previous: [003](003-spellbook-artifact-units-hero.md)

## What changed

- PRs #4–#6 merged into `master`.
- Open, stacked (merge in order, see 003 for the no-force-push procedure):

| PR | Branch | Base | Content |
|---|---|---|---|
| #7 | `refactor-scripts` | `master` | `scripts/refactor/` helpers + restored a comment lost in #6 |
| #8 | `maplib-split` | `refactor-scripts` | map drawing → `mapview`; `maplib`, `city`, `setup`, `player`, `ai` → core |

`core/` now: `ai`, `artifact`, `building`, `city`, `data`, `hero`, `maplib`, `neural`, `pathfinding`,
`player`, `setup`, `spellbook`, `terrain`, `units`. CI guard clean; build, vet, all tests pass.

## Design decisions made in #8 (worth knowing)

- **Draw methods → functions.** Go can't define methods across packages, so `Map.DrawLayer1(...)`
  became `mapview.DrawLayer1(m, ...)`; extra-tile drawing is `mapview.DrawExtraLayer1/2` with a type
  switch over the seven `Extra*` types. `maplib.ExtraTile` now only has `Serialize()`.
  **If you add a new `Extra*` type, add a case to both switches in `mapview/extra.go`**, or it won't draw.
- **Render state lives behind `maplib.Map.RenderCache any`.** `mapview` lazily stores its tile
  cache and minimap pixels there. Core never touches it. (Alternatives considered: global cache
  keyed by `*Map` — leaks and hidden state; renderer objects held by every caller — much bigger change.)
- **`mapview.Renderer{Map: m}`** embeds `*maplib.Map` and adds the draw methods, so code that
  wants "a map that draws itself" keeps working: the overworld's `OverworldMap` interface, and
  `game.RoadMap` (embeds `Renderer`, overrides `DrawLayer2` to draw the planned road).
- `setup` kept its name for the model (`WizardCustom`, `NewGameSettings`: ~375 references);
  the screens moved to `setupview` (~20 references).

## Not verified visually

There is no MoM data on this machine, so the game was never launched. The moved drawing code is
identical apart from receiver→argument and qualifiers (checked by normalizing and diffing), and the
only new logic is the lazy cache and the `Renderer`/dispatch glue. Still: **run the game once and look
at the overworld map, minimap, road building, cartographer and map editor** before building on this.

## Next steps

1. Merge #7 → #8. Smoke-test the game (see above).
2. **`combat` split** (001 step 3 starts here): `combat/model.go` (8.8k lines) has only 3 rendering
   refs: `type TileDraw func(*ebiten.Image, *util.ImageCache, *ebiten.DrawImageOptions, uint64)` and a
   `Drawer:` closure in the model. Replace with data (e.g. an enum/asset ID) drawn by the screen, then
   move model files (`model.go`, `remote.go`, `observer.go`, ...) to `core/combat`, screens stay.
3. **`game/model.go`** (2k lines) has no rendering imports except `combat`; after step 2 it can move
   to `core/game` (check the rest of the `game` package's files first).
4. **`game/game.go`** (8.6k lines, 181 refs, imports ~30 UI packages): the real tangle. Follow
   001: split state/logic into core, keep screens; seam is `Events chan GameEvent`.
5. Command/event API, then wasm bridge for the Three.js renderer.

## Tools

`scripts/refactor/` (README there): `movefuncs.py`, `requalify.py`, `fixcallers.py`,
`prune-imports.sh`, `check-move.sh`, `tidy-imports.py`. Known gotchas: map-literal keys aren't
qualified automatically; code using the package name as a variable needs an alias (`artifactlib`);
Go reports ≤10 errors per package, so loop the build.

## Open questions

- Frontend layout: the `*view` packages are effectively `frontend/ebiten`. Move them under
  `frontend/ebiten/` at some point, or leave as is?
- How long to keep LBX loading working?
- Which Civ concepts come first after the split?
