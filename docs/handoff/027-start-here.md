# Handoff 027: start here (state at end of 2026-11-07)

This is the entry point for the next session. It replaces [020](020-start-here.md).
Read `CLAUDE.md` first, then this file. Handoffs 021–026 have the details of each round.

## Where things stand

- **All PRs are merged** (#25–#33). No open PRs. `master` is the base for new work.
- **Core** (`core/`, pure Go): the whole game plays through JSON commands, events and views
  ([`docs/frontend-protocol.md`](../frontend-protocol.md)). `GameHost` (`core/game/host.go`) runs
  a game for one human player and owns the turn loop. Overland units have stable ids. Views name
  their sprites by asset id (`core/assets`).
- **Wasm core** (`frontend/wasm`): `GOOS=js` build of core only, exposing `civ.*` to JavaScript.
- **Web frontend** (`frontend/web`), playable in the browser:
  - overland map in Three.js from the user's exported sprites (animated terrain, overlays, cities,
    stacks, fog dimming, east-west wrap, pan/zoom);
  - HUD, stack selection and moves, next unit, end turn, event log;
  - dialogs: research, hero/mercenary/merchant offers, spell trades, lairs, raze-or-keep, spell
    targets;
  - city screen: farmers, production choices, buy, sell, garrison;
  - tactical battles: isometric battlefield, figures in banner colours, reachable tiles and
    targets, click to move/attack, Done/Wait/Auto/Flee.
- **Assets**: `util/export-assets` writes the overland and battle sprites as PNG sheets plus
  `assets.json` (4865 sprites). Battle units are banner-neutral; the page recolours them.
- **CI**: only the Linux x86-64 job runs on push. macOS, Windows and Linux arm64 run only when
  started by hand (Actions tab, `workflow_dispatch`).
- **Tools on this machine**: Playwright (Python, with its own Chromium) for browser tests;
  xdotool + Xvfb + ImageMagick to drive and screenshot the Ebitengine game.

## Running and checking

```sh
# data exports (derived from copyrighted data; kept outside the repo, never commit)
go run ./util/export-gamedata -data "/home/bernardo/ROMs/ms-dos/Master of Magic" -out ~/.local/share/civ-of-magic/gamedata
go run ./util/export-assets   -data "/home/bernardo/ROMs/ms-dos/Master of Magic"   # -> ~/.local/share/civ-of-magic/assets

# the web frontend: builds the wasm core, serves the page and the exports
go run ./util/serve-web                     # http://localhost:8080  (?opponents=1-4, ?difficulty=hard, ?log)
python3 frontend/web/smoke_test.py          # Playwright, against a running serve-web (also -addr ...; --shots DIR)

# the Ebitengine game
go run ./game/magic -data "/home/bernardo/ROMs/ms-dos/Master of Magic"
go run ./game/magic -data "/home/bernardo/ROMs/ms-dos/Master of Magic" -watch -music=false   # AI vs AI

# headless
go run ./util/headless-game -years 200
```

Checks for every round:

```sh
go build ./... && go vet ./...
go test ./core/... ./lib/... ./game/...            # CI runs this under xvfb-run
go list -deps ./core/... | grep -E 'ebiten|/lbx'   # must print nothing (CI guard)
GOOS=js GOARCH=wasm go build -o /dev/null ./frontend/wasm
CIV_GAMEDATA=~/.local/share/civ-of-magic/gamedata go test ./core/game -run 'TestHeadlessGame|TestGameHost'
MOM_DATA="/home/bernardo/ROMs/ms-dos/Master of Magic" go test ./util/export-gamedata ./util/export-assets
python3 frontend/web/smoke_test.py http://localhost:8080   # with serve-web running; for frontend changes
```

Tips: `frontend/web/*.js` are ES modules; syntax-check one by copying it to a `.mjs` file and
running `node --check`. Battles in tests: the smoke test finds a guarded site near a fresh
capital (retrying games) and marches there. `window.civGame` exposes the page state and
`civGame.execute(type, command)` for tests.

## Next steps, in order

### 1. Combat spells in the battle screen

`battle-cast` already works in core (wizard spells once per turn within the army's mana pool and
mana/range; unit spells within casting skill; item charges).
- Battle view: `spells` for the wizard (castable now: name, cost, whether it needs a unit or tile
  target and which side) plus `casted` (already cast this turn) and the mana pool; for the
  selected unit its spells and charges.
- Page: a Spells button and list; picking a spell that needs a target enters target mode
  (highlight valid units/tiles, click to cast, Esc cancels); spells offering a choice of units
  (Raise/Animate Dead) show the choices.
- Smoke test: cast a spell in the battle scenario (give the wizard a known combat spell, or pick
  a game whose wizard has one).

### 2. Battles the AI starts against you

In `GameHost`, AI attacks on the human are fought automatically, because the AI turn runs
synchronously. Run the AI turns inside a coroutine (like `CommandSession`) and pause in
`DoCombat` when the defender is the human, so `end-turn` can return `waiting: "battle"`. The page
already opens the battle screen whenever the status says so.

### 3. Battle screen polish

- City walls, wall of fire and wall of darkness on the field (`combatview.DrawWall`; walls come
  from `citywall.lbx`, fire/darkness from `cmbtfx.lbx`); export those sprites and add wall/fire/
  darkness per tile to the battle view. The chaos node river top is also missing.
- Show moves and attacks: the core resolves instantly. Either animate between views in the page
  (walk along the path, attack frames 2-3 of the sprite) or have the core report what happened
  (combat events as JSON: moves, attacks, damage, deaths).
- A battle log and unit details (attack, defense, abilities) on hover or click.

### 4. Web frontend gaps

- First turn: ask for research at the start (the core sends `choose-research` only from turn 2;
  the view could carry the candidates, or the host could emit the event at game start).
- Magic screen (power distribution, alchemy) and the spellbook: cast overland spells from the page
  (commands exist: `cast-spell`, `set-power-distribution`, `alchemy`).
- Armies and items: hero equipment, the vault, disbanding, splitting stacks (`set-unit-active`).
- Rename city, city enchantments; tax rate; diplomacy screen for known wizards.
- Soft fog edges (export `mapback.lbx` 0-13 and draw like `Overworld.DrawFog`).
- A start screen (wizard, opponents, difficulty) instead of URL parameters.

### 5. The 3D / retro look

The map is a flat canvas texture under an orthographic camera. Next: terrain height and a tilted
camera; render to a low-resolution target with a ~32-colour palette, Bayer dither and depth
outlines; sprite units as billboards. A browser prototype ("Realm of Hexes") exists on claude.ai.

### 6. Ebitengine smoke tests with xdotool

[`docs/smoke-test.md`](../smoke-test.md) has 53 hand checks, none done since #13. With xdotool
under Xvfb most can now be driven: start the game with `-start`, screenshot with
`import -window root`, click by screen coordinates. Most important first: moving stacks with
mouse and keys, then tax, patrol, disband, unit toggle, alchemy, diplomacy, casting with target
picking. Consider a script in `test/` or `scripts/` so it can be rerun.

### 7. Smaller open items

- AI wizards never start diplomacy (no proposals, no war declarations): a game feature to design.
- `GameModel.PlanarTravel` reads the viewed plane (`model.Plane`).
- Offers declined in the Ebitengine screens don't clear `PendingOffers` (harmless).
- `TakeEvents` returns every event, including the AI players'; a per-player filter in the host.
- Unit ids aren't saved with a game (fine until save/load goes through the protocol).
- Decide on the upstream side programs `game/arena`, `android/`, `game/network-combat`, and later
  the LBX-only tools in `util/` (`mapeditor`, `fonteditor`, `make-lbx`, `lbxviewer`, ...).

## Conventions (see CLAUDE.md for the rest)

- No gofmt; 4-space indentation; spell out variable names. Import `core/game` as `gamecore`.
- Branch per change, squash-merge to `master`, PR per round. After a stacked PR's base is
  squash-merged, merge `master` into the branch (no force-push) and retarget the PR
  (`gh pr edit` fails on this repo with a Projects (classic) error; use
  `gh api -X PATCH repos/bfalese/civ-of-magic/pulls/N -f base=master`).
- The user merges PRs unless they ask otherwise.
- New rules go to `core/` with tests; every player action gets a command; events need a JSON form;
  views name sprites by asset id.
- End each session with a new `docs/handoff/NNN-*.md`.
