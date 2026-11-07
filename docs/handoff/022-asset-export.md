# Handoff 022: asset IDs and the asset export

Date: 2026-11-06
Previous: [021](021-wasm-core.md). Start-here doc: [020](020-start-here.md).

## State

- PR #25 (`wasm-bridge`, the wasm core) and PR #26 (`remove-itchio`) are open.
  **Don't merge without explicit approval.** #26 touches the Makefile next to #25's `core-wasm`
  target; a small conflict at most.
- This round: branch `export-assets` on top of `wasm-bridge` (its PR is stacked on #25).
- Decisions (see 021): the renderer goes in `frontend/web`; it starts with the existing MoM art
  (the user's own export) and replaces it over time.

## What changed

1. **`core/assets`**: the asset ID scheme (`terrain/<tile index>`, `unit/<race>/<unit>/<banner>`,
   `unit-background/<banner>`, `city/<size>/<open|walled>/<banner>`, `road/<dir>`,
   `road-enchanted/<dir>`, `bonus/<name>`, `encounter/<name>`, `encounter/open-tower`,
   `corruption`, `node-zone/<banner>`). Core only.
2. **Views carry asset IDs** (`core/game/view.go`): tile `asset` + `overlays` (in drawing order,
   as the Ebitengine map: roads, bonus, lair/tower, corruption, node zone), unit `asset`, stack
   `background`, city `asset`, foreign stack `asset`/`background`. The bonus overlay is hidden on
   corrupted tiles, like the `bonus` field (the Ebitengine map still draws it there).
3. **`util/export-assets`**: reads terrain.lbx, units1/2.lbx and mapback.lbx with the game's
   palettes and banner recolouring, shelf-packs everything into 1024-wide PNG sheets and writes
   `assets.json`. On the user's install: 2810 sprites, 3117 frames, one 1024x1062 sheet, ~3 s.
   Output in `~/.local/share/civ-of-magic/assets` (default; never commit it).
   Test (`MOM_DATA=...`): every ID the views can produce is in the manifest and inside its sheet.
4. Docs: `frontend-protocol.md` (sprites in views, the Assets section), CLAUDE.md layout.

Checked by rendering a wasm game's map view + player view from the sheets alone (terrain,
river, city with flag, tower, node all correct).

## Not exported yet

Fog edges (mapback 0-13), unit enchantment outlines, the warp effect, combat graphics, city
screen, UI and fonts. The map renderer needs fog first; the rest comes with the screens that use it.

## Next

`frontend/web`: a Three.js page over `civ` (wasm core) that loads `assets.json` + sheets and draws
the overland map: tiles from the manifest (animated frames), overlays, cities and stacks; camera
pan/zoom; clicks to select and move. Then the retro look (low-res target, palette, dither).
