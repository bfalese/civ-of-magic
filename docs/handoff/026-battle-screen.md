# Handoff 026: tactical battles in the web frontend

Date: 2026-11-07
Previous: [025](025-city-screen.md). Start-here doc: [020](020-start-here.md).

## State

- PRs #31 (CI Linux only) and #32 (city screen) are merged.
- This round: branch `battle-screen`, PR against `master`. **Don't merge without approval.**
- After `export-assets` changes, re-export: `go run ./util/export-assets -data "<MoM dir>"`.

## What changed

1. **Battle view** (`core/game/battle.go`): `tiles[y][x]` (ground asset, top + alignment, mud),
   `road`/`clouds` in towns; per unit `asset` (combat sprite for its facing), `banner`, `facing`,
   `figures` (offsets from `units.CombatPoints` for its visible figures), `flying`; for the
   selected unit `moves` (empty tiles `CanMoveTo`) and `targets` (`ranged` if in arrow range and
   `CanRangeAttack`, else `melee` if in melee range and `CanMeleeAttack`), the checks
   `CombatModel.Update` makes for a click. ~0.5 ms per view.
2. **Asset ids** (`core/assets`): `battle-tile/<lbx>/<i>`, `battle-top/<lbx>/<i>`, `battle-mud`,
   `battle-road`, `battle-clouds`, `battle-unit/<race>/<unit>/<facing>`.
3. **export-assets**: battlefield ground (terrain files 0-47, water), tops (48-57; town houses,
   lair/tower/temple/keep buildings, cropped like the screen), mud, road, clouds; all units' combat
   figures × 8 facings. Battle units are exported once with marker colours in the 4 banner palette
   entries (215-218); the manifest's `bannerMarker` / `bannerColors` let the page recolour them
   (cached per frame). 4865 sprites, 4 sheets, ~1.7 MB.
4. **Battle screen** (`frontend/web/battle.js`): the original's 320×200 window onto the field
   (tile centre = S·R(-45°)·(x,y) + (-220, 80), S = (w·3/4−2, h·3/4−1)), integer zoom to fit;
   ground, mud, town road/clouds, reachable tiles, targets (red melee / orange ranged, pulsing),
   the selected unit, mouse hover; tops and units painted back to front; figures in the owner's
   colour, flying units animate; small health bars. Click a tile → `battle-act`; Done/Wait/Auto/
   Flee buttons and D/W/A keys. Opens when the session waits for a battle and closes after.
5. Battle results in the log from your side ("You won the battle at x,y").
6. **Smoke test** has a battle scenario: finds a guarded site within 4 tiles of a fresh capital
   (retries up to 10 games), marches there, enters, clicks a reachable tile on the canvas (the
   unit walks there), then Auto until the screen closes.

## Not yet

- Combat spells from the page (`battle-cast` exists; needs a spell list in the view: the wizard's
  castable spells with costs, the selected unit's spells/charges, and target picking).
- City walls, wall of fire/darkness on the field; the chaos node river special top.
- Animations of moves and attacks (the core resolves instantly; the page jumps to the result),
  damage numbers, a battle log, unit details on hover.
- AI-initiated battles against the player are still auto-resolved in `GameHost` (021).
