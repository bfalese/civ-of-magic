# Handoff 044: the small-features backlog (043) done

Date: 2026-11-09
Previous: [043](043-small-features-backlog.md) (the backlog; on PR #49), [042](042-ai-spells.md).
Start-here doc: [034](034-start-here.md).

## State

#48 (AI spells) is merged. The five groups of 043 are one PR each, **stacked**: each branch
starts from the one before. Merge them in order, and retarget each to `master` once the one
below it is in:

| group | PR | branch (base) |
|---|---|---|
| 1. map orders | #50 | `map-orders` (`master`) |
| 2. save and load | #51 | `save-load` (`map-orders`) |
| 3. lists | #52 | `lists` (`save-load`) |
| 4. minimap, tile info, wizard info | #53 | `map-overview` (`lists`) |
| 5. small core fixes | #54 | `core-fixes` (`map-overview`) |

This file is on #54. #49 (the 043 doc itself) is still open.

## What changed

1. **Map orders** (#50). Stack panel buttons and keys: Build (B; Settle / Meld / Purify, as the
   stack view's `build` says), Road (R; the path shows while you pick, `civ.roadPath`), Plane (T,
   `planar-travel` in the view), Patrol (P; units show `busy`, Next unit skips them, checking a
   unit again ends its order). Hero rename (✎) on the items screen. Core: `StackBuildAction`,
   `CanPlanarTravel`, `RoadPathFor`. The 027 bug is fixed: `PlanarTravel(player, stack)` uses the
   stack's plane, not `model.Plane`. Test scenario `orders`.
2. **Save and load** (#51). `GameHost.Save` / `LoadGameHost` (`core/game/hostsave.go`): the
   model as the desktop saves it, plus the human's banner, a spell waiting for its target and
   unplaced new items. Open offers and trades aren't kept. Unit ids are reassigned on load.
   `civ.save` / `civ.load`. The page gzips an autosave (every turn's start) and a save slot into
   `localStorage`, downloads and loads save files, and has a ☰ game menu (G), **Continue** on the
   start screen and `?load=autosave|save`.
3. **Lists** (#52): Lists in the top bar, C for cities and A for armies (desktop citylistview /
   armyview). A click opens the city or selects and centres the stack.
4. **Map overview** (#53). Minimap in the bottom-right corner (click or drag to move the view).
   Tile info on hover, from the new core `MakeTileInfo`; the **desktop surveyor now draws from
   it** too. Wizard info (the mirror) in the Wizards screen: `MirrorView` for you and each known
   wizard.
5. **Core fixes** (#54):
   - `TakeEvents` passes on only what concerns the human (`GameHost.concernsHuman`). The "can not
     enter the city" notices are sent only for the human's moves now.
   - New items: **the strict rule**. `end-turn` is refused while new items wait
     (`ErrNewItemsWaiting`), and the page opens the items screen.
   - Wizard portraits (`wizard-portrait/<base>`) and item slot pictures (`item-slot/<kind>`) are
     exported and used on the start, wizards, magic and items screens. **Re-export the assets**
     (`util/export-assets`) to see them; without them the screens fall back to text.
   - `CloseOffer`: offers answered by the desktop's screens or an AI's own decision are no longer
     left in `PendingOffers`.
   - The `orders` scenario searches the whole map for its tower, since about 1 map in 200 had no
     spot within 12 tiles of the capital (a fix for #50, carried here).

Every PR passed: build, vet, the core guard, the wasm build, `go test ./core/... ./lib/...
./game/...` under xvfb, the gamedata host tests, the exporter tests with `MOM_DATA`, and the
browser smoke test against serve-web.

## Not checked by hand

- The desktop **surveyor** (rewritten on `MakeTileInfo`) and the desktop **plane button** with a
  stack on a tower (`PlanarTravel` takes the stack now). Both are listed in `docs/smoke-test.md`.

## Notes

- The top bar is full at 1200 px: the smoke test checks that it stays on one line. Cities and
  Armies share one Lists button, and Next unit is "Next". Anything new there needs room first.
- AI players' new items still pile up in `PendingItems`, unused (saves keep only the human's).
- Saves are versioned (`HostSaveVersion` 1). The model part is the desktop's `SerializedGame`, so
  a change there changes both.

## Next

The "Later" list of 043: the diplomacy decisions in `docs/core-review.md`, the custom wizard
builder, Spell of Mastery, and `SpellCanWork` for Disenchant Area / Call Chaos / summons. Then 034
step 7: the 3D/retro look, Ebitengine xdotool smoke tests, the upstream side programs.
