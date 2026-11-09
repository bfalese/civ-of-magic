# Handoff 045: start here (state at end of 2026-11-09)

The entry point for the next session; it replaces [034](034-start-here.md) as the start-here doc.
Read `CLAUDE.md` first, then this file. [044](044-small-features-done.md) has the details of the
first half of today.

## Where things stand

Everything from today is merged into `master` except this branch (`smoke-gpu`, the `--gpu` smoke
option and this file). Merge it first if the user approves.

- **043 backlog** (#50–#54, summary in 044): map orders, save/load, city and army lists,
  minimap / tile info / wizard info, small core fixes.
- **043 "Later" list** (#55–#58):
  - battle spell checks for Disenchant Area, Call Chaos and Summon Demon;
  - the Spell of Mastery wins the game (ending screen);
  - the custom wizard builder on the start screen (core check via `civ.checkWizard`);
  - AI diplomacy: AI wizards declare war and propose peace / pacts / alliances; treaties now stop
    AI attacks; attacking a treaty partner means war.
- **`run.sh`** (#59) and the plan change: the desktop smoke checklist is to be ported to tests,
  no xdotool suite.
- **3D map** (#60–#62), `frontend/web/map3d.js`, the default (`?view=2d` for the flat map):
  - isometric orthographic camera; the terrain texture painted as in 2D on a heightmap ground;
  - cities, stacks, lairs and nodes as billboards, node billboards animated;
  - grid painted on the ground texture; heights per terrain type with slope shading;
  - H flattens the ground or brings the height back.

The web game is now the target; the Ebitengine game is a legacy frontend that still builds.

## Running and checking

```sh
./run.sh                 # web game in the browser (3D); 8099 when 8080 is taken (Open WebUI)
./run.sh smoke --gpu     # smoke test on the Intel GPU (~20 s); without --gpu: SwiftShader (~34 s)
./run.sh export          # re-export gamedata.json and sprites (needed after #54: portraits, slots)
./run.sh desktop         # the Ebitengine game
```

Checks every round: `go build ./... && go vet ./...`; `go test ./core/... ./lib/... ./game/...`
under `xvfb-run`, with `CIV_GAMEDATA=~/.local/share/civ-of-magic/gamedata` for the host tests and
`MOM_DATA=...` for `./util/...`; the `ebiten|/lbx` guard on `go list -deps ./core/...`; the wasm
build; the smoke test against a fresh serve-web.

**Test scenarios** (`core/game/scenario.go`, page `?scenario=`): `battle`, `siege`, `spellbook`,
`items`, `orders`, `mastery`, `proposal`. Add one rather than making a test play random games.

## Gotchas learned today

- **Software rendering in tests.** Headless Chromium falls back to SwiftShader (CPU WebGL); CI has
  no GPU at all. Frame rates measured there mean nothing (33 fps vs 60 on the real GPUs).
  `--gpu` uses ANGLE on OpenGL (Intel HD 630); `--gpu=vulkan` uses the GTX 1050.
- **Picking in 3D.** Billboards reach over the tiles behind them. `tileAt` gives the ground tile
  when something stands on it, otherwise the frontmost billboard, otherwise the ground. Smoke
  steps that hover or click near a city use a tile in front (south-west).
- **Merging PRs made from the same base.** They conflict in the shared lists: protocol doc,
  `main.js` event handlers, `smoke_test.py` steps, `scenario.go`, `eventjson.go`. Merge `master`
  into the branch and keep both sides. Watch hunks cut mid-block: the shared closing lines sit
  after the marker. Then re-run the tests and the smoke test.
- **Retargeting PRs.** `gh pr edit` fails on this repo. Use
  `gh api -X PATCH repos/bfalese/civ-of-magic/pulls/N -f base=master`.
- **Restarting serve-web.** Restart it after any core change; a leftover one serves a stale core.
  Stop it by the PID from `ss -ltnp | grep 8099`.
- **Overland sprites.** Units, cities and lairs have one frame in the original data: nothing to
  animate without new art. Only node tiles (4 frames) and node zones (6) animate.
- **Tuning AI diplomacy.** Starting relations run from −48 to +14, so the thresholds in
  `core/game/aidiplomacy.go` are set against that. To check whether anything happens, play long
  AI-only games and count treaties (the probe was a throwaway test; see #58).

## Next

1. **The 3D look**, continuing 034 step 7. In rough order:
   - render to a low-resolution target and scale it up (pixel-art look at any zoom);
   - a ~32-colour palette with Bayer dithering;
   - depth outlines;
   - camera tilt or rotation control, if wanted;
   - water (flat surface or shimmer) and rivers following the land;
   - mountains and forests standing up (billboards like nodes, or modelled).

   The height table, light direction and subdivisions are constants at the top of `map3d.js`.
2. **Port the smoke-test checklist** (`docs/smoke-test.md`): each check about a game rule becomes
   a host test or a smoke step, marked with the test's name; checks only about Ebitengine screens
   are struck through.
3. **Open decisions:**
   - the human's treaty proposals are still always accepted (`docs/core-review.md`;
     `WillAcceptTreaty` could answer them);
   - tributes don't move the visible mood at once;
   - the custom wizard's starting spells are random (the desktop lets you pick them);
   - the desktop game: drop it, or keep it with a "starts under Xvfb" check;
   - the upstream side programs (`game/arena`, `android/`, `game/network-combat`, LBX tools).
4. **Not checked by hand:** the desktop surveyor (now drawn from core `MakeTileInfo`) and the
   desktop Plane button (`PlanarTravel` takes the stack); both are listed in `docs/smoke-test.md`.
