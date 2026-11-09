# Handoff 031: a faster smoke test, and a fast battle view

Date: 2026-11-08
Previous: [030](030-battle-replay.md). Start-here doc: [027](027-start-here.md).

## State

- PR #37 (battle replay) is merged.
- This round: branch `faster-smoke-test`, PR against `master`. **Don't merge without approval.**

## What changed

1. **The battle view was slow**: about 420 ms natively and about 2.4 s in wasm, in a real game.
   `reach` ran a full path search (`CanMoveTo`) for every battlefield tile, and a search for an
   unreachable tile explores the whole field. Now it skips tiles farther (Chebyshev) than
   ceil(moves left): a step needs moves left and costs at least 1 (`CanFollowPath`), so those
   tiles can never pass anyway. The result is identical (a test compares it with `CanMoveTo` on
   every tile), and the view takes about 1 ms. Players see this too: entering a battle and each
   click used to wait 2+ s in the browser. The Go session tests went from about 6.5 s to 0.45 s.
2. **Scenarios** (`core/game/scenario.go`, `HostSettings.Scenario`, page `?scenario=`). The
   `battle` scenario puts a lair with three skeletons next to the human's capital, removes the
   game's own encounters next to it (some are unguarded, and a finder would pick them), and gives
   the wizard Fire Bolt, 300 mana and casting skill. `TestHostBattleScenario` checks ten games:
   the lair is there, entering starts a battle, and Fire Bolt is castable. 150 games passed after
   the encounter cleanup.
3. **Smoke test rewritten** (`frontend/web/smoke_test.py`):
   - The battle part loads `?scenario=battle` and walks into the lair. Before, it reloaded up to
     10 random games, ended turns to bank mana and marched up to 12 turns. It always casts Fire
     Bolt now (cancel with Esc, then on a skeleton), so the targeted-spell path is always covered.
   - It fails fast: a `Smoke.wait` helper polls and stops at the first page error, and steps time
     out after 5 s (`STEP_TIMEOUT`; loading 20 s, auto-battle 20 s). Playwright's default was 30 s.
     A failure saves `failure.png` (in `--shots DIR`, else the current directory).
   - The first-turn part uses `?opponents=1` (quicker AI turns). Fixed the `--shots DIR` parsing,
     which took DIR for the URL.

## Timings

- Full pass: about 5.4 s, down from 11-18 s. Most of what's left is the two page loads (about
  1.4 s and 0.9 s: wasm core plus a new game each).
- A page error fails in about 0.4 s; a step that never happens, or a missing button, in 5 s; no
  server in under 1 s.
- Start serve-web once and keep it running between runs (`go run ./util/serve-web` rebuilds the
  wasm core at start, about 5 s). Check `ss -ltnp | grep 8080` for a leftover one serving an old
  core.

## Open: a rare hang in TestHeadlessGame

In one batch of 30 repeated runs of the gamedata tests, `TestHeadlessGame` (AI vs AI, battles
fought automatically through `battle.Run`) hung until the 10-minute test timeout. About 210 other
runs passed (60 on master), so it's rare, and this branch doesn't touch that code. A hang in the AI
turns would freeze the browser too. To catch it: build the test binary and loop
`game.test -test.run 'TestHeadlessGame$' -test.timeout 60s` until one fails; the timeout dump
shows the stuck goroutine.

## Next

- Find the TestHeadlessGame hang (above).

- Walls on the battlefield (027 step 3).
- 027 step 4: web frontend gaps.
- More scenarios as tests need them (a city siege with walls, an AI attack on the human).
