# Handoff 024: the web frontend (first playable map)

Date: 2026-11-06
Previous: [023](023-protocol-gaps.md). Start-here doc: [020](020-start-here.md).

## State

- Branch `frontend-web`, stacked on `protocol-gaps` (PR #29; it uses the unit ids).
  **Don't merge without explicit approval.**

## Running it

```sh
go run ./util/serve-web        # builds frontend/wasm, serves frontend/web + your exports
# open http://localhost:8080   (?opponents=1-4, ?difficulty=hard, ?log for the Go log)
```

`util/serve-web` serves `frontend/web`, the freshly built `core.wasm`, Go's `wasm_exec.js` from
GOROOT, `gamedata.json` from `~/.local/share/civ-of-magic/gamedata` and `/assets/` from
`~/.local/share/civ-of-magic/assets` (flags to change them), with caching off.
Three.js 0.160 comes from jsDelivr through an import map (no npm, no build step).

## What's there

- `frontend/web/core.js`: the wasm core as a JS class (`execute`, `events`, `view`, `map`, ...).
- `frontend/web/assets.js`: sprites by asset id from the manifest and sheets.
- `frontend/web/map.js`: `MapRenderer`. The map is drawn into a canvas at original pixel size
  (terrain with animated frames at 4 fps, overlays, cities, stacks, the selected stack's path,
  dimming for explored-but-not-visible tiles) and shown as a texture on three planes side by side
  (east-west wrap) under an orthographic camera. Integer zoom steps (wheel, +/-), drag or arrow
  keys to pan. Stacks inside cities aren't drawn unless selected (as the original); the selected
  stack blinks.
- `frontend/web/main.js`: the game screen. Top bar (date, gold, mana, food, research, plane
  switch, next unit, end turn), side panel (selected stack's units, or a city's details), event
  log, dialogs for research, hero/mercenary/merchant offers, spell trades, lairs, raze-or-keep and
  battles (auto or flee only), and spell targets (click a tile/stack, or pick a wizard).
  Selection follows the stack's first unit id. Keys: N next unit, Enter end turn, Esc cancel.

Checked in headless Chrome over the DevTools protocol (no automation tool installed; a small
Node CDP script in the session scratchpad): select the capital's stack, click two tiles east (it
walks one, path dot left), end turn (February, research dialog); no console errors.

## Not yet

- Tactical battles (the battle view exists in the protocol; only auto/flee here).
- City screen actions (production, buy, farmers), magic screen, spellbook/casting from the UI,
  armies/vault. All have commands already.
- Fog edges (soft edges, mapback 0-13; not exported yet), terrain height/tilt, the retro look
  (low-res target, palette, dither, outlines), sound.
- The first turn doesn't ask for research (the core sends `choose-research` from turn 2).
- No automated test for the page; consider a CDP smoke script in the repo if it grows.

## Next

Pick between: the city screen (production/buy, most-used actions), tactical battles in the page,
or the 3D/retro rendering step.
