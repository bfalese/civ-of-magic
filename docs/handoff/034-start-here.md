# Handoff 034: start here (state at end of 2026-11-08)

This is the entry point for the next session. It replaces [027](027-start-here.md).
Read `CLAUDE.md` first, then this file. Handoffs 028–033 have the details of each round.

## Where things stand

- **Merged today**: #34–#39 (combat spells in battle, AI battles pause for you, battle replay/log/
  unit details, faster smoke test + 400x faster battle view, walls/fortress + siege scenario).
- **Open: PR #40** (magic screen and spellbook, first-turn research; this doc is on that branch).
  CI ran on push; merge it first if the user approves. Branch `magic-screen`.
- **Web frontend** now plays: overland map, city screen, magic screen/spellbook, research
  questions from turn 1, tactical battles (your attacks and AI attacks on you) with spells, target
  picking, replayed moves/attacks with damage numbers, unit details, battle log, walls and fortress.
- **Re-export assets** if the local export predates #39 (walls): 4930 sprites expected.

## Running and checking

```sh
go run ./util/serve-web                    # http://localhost:8080; rebuilds the wasm core at start (~5 s)
python3 frontend/web/smoke_test.py         # ~9.5 s; --shots DIR doubles it (screenshots ~0.8 s each)
go run ./util/export-assets -data "/home/bernardo/ROMs/ms-dos/Master of Magic"
```

Checks every round (same as 027): `go build ./... && go vet ./...`, `go test ./core/... ./lib/...
./game/...` (CI under xvfb-run), the `ebiten|/lbx` guard on `go list -deps ./core/...`, the wasm
build, `CIV_GAMEDATA=... go test ./core/game -run 'TestHeadlessGame|TestGameHost|TestHost'`,
`MOM_DATA=... go test ./util/export-gamedata ./util/export-assets`, and the smoke test.

**Scenarios for tests** (`core/game/scenario.go`, `HostSettings.Scenario`, page `?scenario=`):
`battle` (skeleton lair next to the capital; Fire Bolt, Wall of Stone, 300 mana, skill ~60) and
`siege` (human swordsmen next to an AI capital with walls, wall of fire and darkness). Add a
scenario instead of making a test play random games.

## Gotchas learned today

- Port 8080: the user's Open WebUI docker container also uses it (stopped now). Before starting
  serve-web check `ss -ltnp | grep 8080`: a **leftover serve-web serves a stale wasm core**. Stop
  it by PID; never `pkill -f` a pattern that appears in your own command (kills the tool shell).
- Restart serve-web after any core change (the wasm is built at start).
- The map **wraps east-west**: adjacency checks need `min(dx, width - dx)`.
- Battle view `reach`: never search paths to every tile (that was 420 ms per view).
- Smoke test waits use `Smoke.wait` (polls, fails at the first page error, 5 s step timeout).
- `slices.DeleteFunc` on an army's units shifts the backing array: copy pointers before killing.
- `gh pr edit` fails on this repo; retarget with `gh api -X PATCH repos/bfalese/civ-of-magic/pulls/N -f base=master`.

## Next steps, in order

1. **Spell choices and item design** (finish the spellbook): a dialog for `target.choice`
   (Spell Ward's ward, Spell Binding / Disjunction's enchantment, Resurrection's dead hero:
   targets `enemy-global-enchantment`, `dead-hero`), and an item design screen for Create
   Artifact / Enchant Item (`cast-spell` takes `artifact`: see `artifact.Design`). The view may
   need the choices listed per spell.
2. **Armies and items**: hero equipment and the vault (commands exist, see
   `docs/frontend-protocol.md`), disband, split stacks (`set-unit-active`), unit details on the map.
3. **City screen extras**: rename city, city enchantments, tax rate in the top bar.
4. **Diplomacy** screen for known wizards (commands exist).
5. **Start screen** (wizard, opponents, difficulty) instead of URL parameters; soft fog edges
   (`mapback.lbx` 0-13).
6. **Battle**: spell animations (the replay only glows targets); `castable` doesn't know if a
   spell has a valid target (e.g. Disrupt outside a city).
7. Later: the 3D/retro look (027 step 5), Ebitengine xdotool smoke tests (027 step 6), the small
   open items listed in 027 step 7.

## Open issues

- **Rare hang in `TestHeadlessGame`** (AI vs AI): once in a combined batch of repeated runs right
  after another test failed; 400 isolated runs never hung. Maybe tests sharing state in one
  process. If CI ever times out, build the test binary and loop it with `-test.timeout 60s` to get
  the stuck goroutine's stack (031 has the recipe).
- The Ebitengine `DrawWall` now draws through `combat.Tile.WallPieces`; not yet checked by eye in
  the desktop game (a siege there would show it).

## Conventions (unchanged, see CLAUDE.md)

No gofmt, 4-space indent, spelled-out names, `core/game` imported as `gamecore`. Branch per change,
squash-merge, PR per round; the user merges (today they said "merge it and continue": wait for CI,
then `gh pr merge N --squash`). New rules go to `core/` with tests; every player action is a
command; views name sprites by asset id. End each session with a new `docs/handoff/NNN-*.md`.
