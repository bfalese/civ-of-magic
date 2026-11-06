# Handoff 021: the wasm core (game host, JS API, test page)

Date: 2026-11-06
Previous: [020](020-start-here.md) (still the start-here doc; read it first).

## State

- Branch `wasm-bridge` (local commits, not pushed yet). Steps 1–3 of the wasm plan in 020 are done;
  step 4 (the Three.js renderer) is next.

## What changed

1. **`core/game/host.go`: `GameHost`**, a whole game for one human player who plays through
   commands. `MakeGameHost(gameData, HostSettings)`: the human is player 0 (random wizard,
   optional name), 1–4 AI wizards, raiders. `Execute`/`ExecuteJSON` goes through the player's
   `CommandSession`; `end-turn` sets a flag and, once the command (and anything it paused for) is
   done, the host plays the AI turns (`DoNextTurn` + `DoAiTurn`) until it's the human's turn again.
   The human's spells finish through `FinishCast`; the AI's through the headless target picker.
   Every model event is encoded into an outbox (`TakeEvents`), handed to the AI handler (offers,
   treasure) and to `session.ProcessEvent`. `GameOver()`: `""`, `"won"`, `"lost"`.
   `DecodeHostSettings` reads the JSON settings (difficulty/magic by name).
2. **`headless.go` split** (own commit): `makeHeadlessModel` + `addPlayers` (setup without the
   observer step) and `handleAiEvent` (the AI part of `drainEvents`), reused by the host.
3. **`gamedata.Parse(bytes)`** for frontends that load `gamedata.json` themselves.
4. **`frontend/wasm`** (`//go:build js && wasm`, core only): `globalThis.civ` with `newGame`,
   `execute`, `events`, `view`, `map`, `battle`, `setLogging`; every function returns a JSON
   string; panics become errors. Go's log is off by default (the AI is chatty). `make core-wasm`
   writes `frontend/wasm/core.wasm` (12 MB, gitignored) and copies `wasm_exec.js`.
5. **`frontend/wasm/smoke.cjs`**: Node script that plays N turns through the wasm (30 turns ≈ 3 s).
6. **`frontend/wasm/index.html`**: bare test page: pick `gamedata.json` (or `?gamedata=<url>`),
   new game, summary of the player view, a clickable minimap of the explored map (click your
   stack, then a tile: `move-stack`), buttons for the pending question (lair, raze, battle auto/flee)
   and the last research candidate, a raw command box and the event log. Checked in headless Chrome.
7. CI (`build-linux.yml`): builds and vets `frontend/wasm` for js/wasm and guards its deps
   (`go build ./...` on Linux skips it because of the build tag).
8. `docs/frontend-protocol.md`: new section on `GameHost` and the JS API.

## Checks run

`go build ./... && go vet ./...`, `go test ./core/... ./lib/... ./game/...`, the core guard,
`TestHeadlessGame` + `TestGameHost` with `CIV_GAMEDATA`, the Node smoke run, a headless Chrome
screenshot of the test page.

## Known limits / open items

- **AI attacks on the human are auto-resolved**: the AI's turn runs synchronously and isn't paused
  for the human's battle orders. Doing that needs the AI turn loop inside a coroutine like the
  session's (pause in `DoCombat` when the defender is the human).
- `TakeEvents` returns every event, including the AI players' own (their offers, research);
  a per-player filter in the host would be cleaner for a network client.
- The page doesn't show battles graphically; "Auto battle"/"Flee" only.
- The open questions from 020 still stand: where the renderer lives (`frontend/web` in this repo
  or a separate repo); placeholder art vs. requiring the user's MoM data for sprites (sprites need
  asset IDs instead of LBX indexes, 001 step 2).

## Next

The Three.js retro renderer over `civ` (the map view first: terrain tiles as a height/colour grid,
cities and stacks as sprites/markers; low-res render target, ~32-colour palette, Bayer dither).
