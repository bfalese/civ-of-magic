# Handoff 013: events and views as JSON

Date: 2026-11-06
Previous: [012](012-headless-game-and-commands.md). Start-here doc: [009](009-start-here-next-session.md).

## State

- PRs #13–#16 are merged into `master`.
- This round: branch `events-json`, PR #17 against `master`. **Don't merge without explicit approval.**
- The user asked to leave the wasm bridge for last.
- Hand smoke tests still pending: [`docs/smoke-test.md`](../smoke-test.md).

## What changed

- **Events as JSON** (`core/game/eventjson.go`): `GameModel.EncodeEvent(event)` →
  `{"type": ..., "event": {...}}` for all 16 model events. Players by banner, cities/units by
  location, heroes as name/title/level/location, artifacts in their save-game form, treasure items
  with a `kind`. UI-only events on the shared channel return `ok=false`.
  `RandomEventType.Name()` added for stable names.
- **Views** (`core/game/view.go`): `MakePlayerView(player)` (wizard stats, own cities and stacks in
  full, foreign cities/stacks where the fog shows them, known wizards) and `MakeMapView(player,
  plane)` (explored tiles only). Encode helpers for both. Lists are `[]`, never `null`.
- **Headless game** encodes every event it drains (`OnEventJSON` hook, `EventErrors`);
  `TestHeadlessGame` fails on any event that doesn't encode and encodes each wizard's views.
  `util/headless-game -events` prints the event stream, `-view` a wizard's view.
- **`docs/frontend-protocol.md`**: commands, events and views in one place, for the Three.js side.
  Linked from `CLAUDE.md`.

## Checks done

- `go build ./... && go vet ./...`, `go test ./core/... ./lib/... ./game/...` under Xvfb: pass.
- CI guard empty.
- Headless 250 years with `-events`: hire-hero, hire-mercenaries, merchant, new-outpost, refresh,
  treasure, vault all encoded, no errors. Most other events only fire for a human player (the
  headless observer gets none), so they're covered by unit tests only.

## Next (wasm last, as asked)

1. **Spell casting in core**: move the targeting/dispatch of `cast.go`'s `doCastSpell` into the
   model (target as data: unit, city, tile, wizard), so that (a) a `cast-spell` command works,
   (b) the headless game casts every spell, (c) the UI keeps only target picking and animations.
   Biggest remaining piece of logic in the UI.
2. **More commands**: answer offers (`hire-hero`, `hire-mercenaries`, `buy-item` with accept/decline),
   planar travel, unit orders (patrol, wait, disband, split stack).
3. **Route the Ebitengine UI through commands** for simple actions (tax, city screen) so the
   real game exercises the same path.
4. **Wasm bridge**: `GOOS=js` entry point exposing `execute(json)`, `events()`, `view()`, `map()`.
