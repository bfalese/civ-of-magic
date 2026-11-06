# Handoff 012: game data as JSON, headless AI-vs-AI game, command API

Date: 2026-11-06
Previous: [011](011-city-economy-offers-planar-travel.md). Start-here doc: [009](009-start-here-next-session.md).

## State

- PRs #13, #14, #15 are merged into `master`.
- This round: branch `headless-game`, PR #16 against `master`. Waiting for review.
  **Don't merge without explicit approval.**
- Smoke tests for everything since #7 are collected in [`docs/smoke-test.md`](../smoke-test.md),
  ordered as one playthrough.

## What changed

1. **Game setup in core** (`core/game/setup.go`): `InitializePlayer` (capital, starting buildings
   and units), `FindCityLocation`, `InitializeNeutralPlayer` (raiders), `StartingUnits`,
   `SuggestCityName` (from a name list). `game/magic/main.go` calls them; it still fires the capital
   naming prompt and centers the camera.

2. **`core/gamedata`**: spells, buildings, hero and city names, premade artifacts and the artifact
   power table as one JSON file; `TerrainData()` builds map tiles from the rules in `core/terrain`
   (no images). Spells are written with all fields (`spellbook.Spell` marshals as its name only,
   which save games rely on).

3. **`util/export-gamedata`** writes `gamedata.json` from an original install:
   ```sh
   go run ./util/export-gamedata -data "/home/bernardo/ROMs/ms-dos/Master of Magic" -out ~/.local/share/civ-of-magic/gamedata
   ```
   The output is derived from copyrighted data: it lives outside the repo. Its round-trip test
   compares against the LBX readers when `MOM_DATA` is set. Found on the way: `lbxdata.ReadArtifacts`
   listed item abilities in map iteration order (random); now sorted by mask.

4. **Headless AI-vs-AI game** (`core/game/headless.go`, `util/headless-game`):
   ```sh
   go run ./util/headless-game -years 200 -opponents 4      # ~9 s
   CIV_GAMEDATA=~/.local/share/civ-of-magic/gamedata go test ./core/game -run TestHeadlessGame
   ```
   Battles (`Battle.Run`), encounters and treasure, AI offers (heroes, mercenaries, merchant),
   raze/capture by the AI, summons, global enchantments, plain unit enchantments on the AI's target.
   Other spells (city spells, curses, map spells, Black/Chaos Channels) are counted in
   `Stats.SkippedSpells`: their targeting still lives in the UI's `cast.go`. Player 0 is a passive
   observer, as in watch mode (parts of the model assume a human exists).

5. **Command API** (`core/game/command.go`): `move-stack`, `set-research`, `set-tax-rate`,
   `set-city-production`, `set-city-farmers`, `buy-production`, `sell-building`, `build-action`,
   `build-road`, `end-turn`. `GameModel.Execute(context, command)` checks it's the player's turn;
   each command validates ownership, gold and legal targets (commands may come from an untrusted
   client). Things are referred to by map location (cities are unique per tile, a player has one
   stack per tile). JSON: `{"type": "move-stack", "command": {"from": {...}, "to": {...}}}`.
   Not used by the Ebitengine UI yet (it calls the model methods directly).

`CLAUDE.md` updated (layout, tools, commands/events).

## Checks done

- `go build ./... && go vet ./...`, `go test ./core/... ./lib/... ./game/...` under Xvfb: pass.
- CI guard (`go list -deps ./core/... | grep -E 'ebiten|/lbx'`): empty.
- Real game: `-start` (capital prompt, camera, units OK), `-watch` 150 s (71 years), no errors.
- Headless: 250 years with 4 wizards in 15 s; wizards defeated/banished, cities razed/captured.

## Next

1. **Events out as JSON**: the counterpart of commands for the wasm bridge. Event types in
   `core/game/gameevents.go` reference players/cities/units by pointer; give them a JSON form
   with locations/banners (same approach as commands).
2. **More commands**: casting a spell (needs the targeting from `cast.go` moved to core: the same
   work would let the headless game cast every spell), planar travel, hire/decline offers,
   unit actions (patrol, wait, disband).
3. **Route the Ebitengine UI through commands** where it's simple (city screen, tax), so the same
   path is exercised by the real game.
4. **wasm bridge** (001): `GOOS=js` entry point that takes JSON commands and returns JSON events.
5. Asset IDs for sprites (001 step 2) once a second frontend needs them.
