# Handoff 020: start here (state at end of 2026-11-06)

> **Superseded by [027](027-start-here.md)**, the current start-here doc.

This is the entry point for the next session. It replaces [009](009-start-here-next-session.md).
Read `CLAUDE.md` first, then this file, then the newer handoffs ([021](021-wasm-core.md): the
wasm core, steps 1–3 of the plan below, are done). The numbered handoffs 010–019 have the details of each
round if you need them.

## Where things stand

- **Goal** (plan in [001](001-fork-and-decoupling-plan.md)): pure-Go game logic in `core/` (no
  Ebitengine, no LBX), swappable frontends: the Ebitengine game now, a Three.js retro renderer over
  wasm later, headless for tests and AI.
- **All PRs are merged** (#7–#23). No open PRs. `master` is the base for new work.
- **Decoupling is essentially done.** The Ebitengine UI (`game/magic/game/game.go` 5.9k lines,
  `cast.go` 2.0k, `cityview/city-screen.go` 2.6k) is now mostly screens; the rules live in
  `core/`, and the UI passes its dialogs/animations to the model as hooks (handler interfaces).
- **A frontend outside the process can play the whole game through JSON**:
  - **Commands in** (`core/game/command.go`, 34 types): movement, cities, research, tax, casting
    (every overland spell except Spell of Mastery; Create Artifact with an item design), offers,
    unit orders, magic screen, vault, renames, diplomacy, and in battles move/attack/cast/flee/auto.
  - **Events out** (`core/game/eventjson.go`, 20 types) and **views** (`core/game/view.go`: player
    view, map view; `BattleSession.View()` for battles).
  - **`CommandSession`** (`core/game/session.go`): commands that need the player midway (enter a
    lair?, a battle, raze or keep?) pause as a coroutine until answered, then carry on.
  - Full reference: [`docs/frontend-protocol.md`](../frontend-protocol.md).
- **Game data without LBX**: `core/gamedata` (spells, buildings, names, artifacts, power table as
  JSON; terrain tiles from code). Export once with `util/export-gamedata`.
- **Headless AI-vs-AI game**: `go run ./util/headless-game -years 200` (≈10 s, core only).

## Running and checking

```sh
# the Ebitengine game (MoM data is copyrighted: never copy it into the repo)
go run ./game/magic -data "/home/bernardo/ROMs/ms-dos/Master of Magic"
go run ./game/magic -data "/home/bernardo/ROMs/ms-dos/Master of Magic" -start -music=false   # random wizard
go run ./game/magic -data "/home/bernardo/ROMs/ms-dos/Master of Magic" -watch -music=false   # AI vs AI, hands-off

# exported data tables (also derived from copyrighted data; kept outside the repo)
go run ./util/export-gamedata -data "/home/bernardo/ROMs/ms-dos/Master of Magic" -out ~/.local/share/civ-of-magic/gamedata

# headless game, event stream, a wizard's view
go run ./util/headless-game -years 200
go run ./util/headless-game -years 100 -events
go run ./util/headless-game -years 50 -view
```

Checks for every round:

```sh
go build ./... && go vet ./...
go test ./core/... ./lib/... ./game/...            # CI runs this under xvfb-run on Linux
go list -deps ./core/... | grep -E 'ebiten|/lbx'   # must print nothing (CI guard)
CIV_GAMEDATA=~/.local/share/civ-of-magic/gamedata go test ./core/game -run TestHeadlessGame   # with data
MOM_DATA="/home/bernardo/ROMs/ms-dos/Master of Magic" go test ./util/export-gamedata            # export round trip
```

Real-game regression run: build `./game/magic`, run `-watch` under `Xvfb :NN` with `DISPLAY=:NN`
for a few minutes, check the log for `Year=` advancing and no panics; screenshot with
`import -window root shot.png`. There is no input automation tool on this machine, so human-only
paths (mouse/keys) can't be driven headless: they go on the smoke-test list.

CI noise to ignore: the macOS M1 job often waits for a runner ("pending"); Linux and arm64 are the
real checks.

## Hand smoke tests still to do

[`docs/smoke-test.md`](../smoke-test.md): 53 items ordered as one playthrough. **Most important**:
moving your own stacks with the mouse/keys (now goes through `GameModel.OrderStackMove`), then the
other human-only paths (tax, patrol, disband, unit toggle, alchemy, diplomacy, casting with target
picking). Nothing there has been checked by hand since #13.

## Next: the wasm bridge (the user asked to leave it for last; it is now next)

Goal: a `GOOS=js GOARCH=wasm` build of **core only** (not the Ebitengine game; the existing
`make wasm` builds the Ebitengine game) that a Three.js page drives with JSON. Sketch:

1. **A game host in core** (e.g. `core/game/host.go`), reusable by wasm and tests: creates the game
   from `gamedata` (like `MakeHeadlessGame`, but with a human player who has a `CommandSession`),
   runs the turn loop: AI turns via `DoAiTurn` + draining their events (as `HeadlessGame` does),
   the human's turn waits for commands; `end-turn` (`CommandContext.EndTurn`) advances with
   `DoNextTurn(..., startPlayerTurn)` using `FinishCast` as the castSpell hook. Collects the human's
   events (passing each through `session.ProcessEvent`) into an outbox.
2. **The wasm entry point** (e.g. `cmd/wasm` or `frontend/wasm`, `//go:build js`): `syscall/js`
   functions `newGame(gamedataJSON, settingsJSON)`, `execute(commandJSON) → {error, waiting}`,
   `events() → [eventJSON]`, `view()`, `map(plane)`, `battle()`. The page loads `gamedata.json`
   itself (the user's own export; never shipped).
3. **A tiny test page** (plain JS, no Three.js yet) that starts a game, lists events, sends a few
   commands: proves the round trip before the renderer work.
4. Then the Three.js retro renderer (MoM look over 3D: low-res, ~32 colours, Bayer dither, depth
   outlines, sprite units; a browser prototype "Realm of Hexes" exists on claude.ai). Sprites will
   need asset IDs (001 step 2) instead of LBX indexes.

Open questions for the user: where the page/renderer lives (this repo under `frontend/web`, or a
separate repo); whether to ship placeholder art or require the user's MoM data for sprites.

## Smaller open items

- Diplomacy initiated by other wizards (proposals to the player) has no events/answers.
- `GameModel.PlanarTravel` still reads the viewed plane (`model.Plane`); the command switches it.
- Offers declined in the Ebitengine screens don't clear `PendingOffers` (harmless).
- Spell exchange through `trade-spells` is one step (no "see their offer, then decline").
- In the protocol, units in a stack are referred to by index; real ids would be sturdier for a
  network client.

## Conventions (see CLAUDE.md for the rest)

- No gofmt; 4-space indentation; spell out variable names. Import `core/game` as `gamecore`.
- Branch per change, squash-merge to `master`, PR per round. Merging stacked PRs without force-push:
  see [003](003-spellbook-artifact-units-hero.md).
- Mechanical moves in their own commits; tools in `scripts/refactor/` (README there).
- New rules go to `core/` with tests; UI steps become hooks/handlers; every player action should
  also get a command, and events the model emits need a JSON form (`eventjson.go`).
- End each session with a new `docs/handoff/NNN-*.md`.
