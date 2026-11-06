# Handoff 016: offers and unit orders as commands, UI actions through commands, map spells by plane

Date: 2026-11-06
Previous: [015](015-cast-every-spell-in-core.md). Start-here doc: [009](009-start-here-next-session.md).

## State

- PRs #13–#19 are merged into `master`.
- This round: branch `more-commands`, PR #20 against `master`. **Don't merge without explicit approval.**
- Wasm bridge stays last (user's request). Hand smoke tests: [`docs/smoke-test.md`](../smoke-test.md).

## What changed

1. **Offers**: every hero / mercenaries / merchant offer now goes through `GameModel.SendOffer`,
   which also records it in `PendingOffers[player]`. `GameModel.AnswerOffer(player, accept)` and the
   `answer-offer {accept}` command accept (gold checked; a bought item is announced as a `vault`
   event) or decline (a declined hero gains a level, as in the UI). The Ebitengine UI still answers
   offers through its own screens.
2. **Unit commands**: `planar-travel {stack}` (wraps `PlanarTravel`, which needs the view on the
   destination plane; switched and restored inside the command), `patrol {stack}`,
   `disband-unit {stack, unit}`.
3. **Ebitengine UI through commands**: `Game.execute(player, command)` runs a command through
   `GameModel.Execute` and shows a refusal as a notice. Used by the tax collector, the Patrol button
   and disbanding a unit from the unit context menu. Same behavior as before (the Patrol command
   was kept identical: it doesn't clear the stack's path).
4. **Map spells take a plane**: `ApplyCorruption`, `ApplyRaiseVolcano(Terrain)`, `ApplyEnchantRoad`,
   `ApplyChangeTerrain`, `ApplyTransmute`, `ApplyWarpNode`, `ApplyFloatingIsland`,
   `ApplyDisenchantArea` and `CheckSpellTarget` get a `plane` parameter instead of reading the
   viewed plane. The UI passes `game.Model.Plane` (unchanged behavior); `CastSpell` passes the
   target's plane, so the view switch from 015 is gone.

## Checks done

- `go build ./... && go vet ./...`, `go test ./core/... ./lib/... ./game/...` under Xvfb: pass;
  CI guard empty; `TestHeadlessGame` with data passes.
- `-watch` 3 min (84 years), no errors.
- Not covered headless: the human UI actions now going through commands (smoke-test.md §3 / §5).

## Next (wasm last)

1. **City screen through commands** (buy, sell, farmers, production): `cityview` has no model, so it
   needs an executor callback from `game` (like `Game.execute`).
2. **Stack commands**: split / merge stacks, select active units; hero equipment (vault moves).
3. **Remaining `PlanarTravel` view dependency**: give it a plane/stack parameter like the map spells.
4. **Wasm bridge**: `GOOS=js` entry point with `execute(json)`, `events()`, `view()`, `map(plane)`.
