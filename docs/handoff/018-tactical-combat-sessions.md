# Handoff 018: tactical battles and questions through command sessions

Date: 2026-11-06
Previous: [017](017-commands-group-2.md). Start-here doc: [009](009-start-here-next-session.md).

## State

- PRs #13–#21 are merged into `master`.
- This round: branch `tactical-combat`, PR #22 against `master`. **Don't merge without explicit approval.**
- Wasm bridge stays last (user's request) and is now the next big item.
- Hand smoke tests: [`docs/smoke-test.md`](../smoke-test.md) (updated with #16/#18/#20/#21 items).

## What changed

**`CommandSession`** (`core/game/session.go`): one human player's game driven by commands. A
command that needs the player midway (a lair to enter, a battle, raze-or-keep after taking a
city) runs as a coroutine (`lib/coroutine`) and pauses there; `Execute` returns, `WaitingFor()`
says what's needed (`confirm-encounter`, `battle`, `raze-city`), an event announces it, and the
next commands answer it. Then the paused command (e.g. the stack's walk) carries on. The session
implements `PlayerMovementHandler`, so `move-stack` battles and encounters go through it.
`ProcessEvent` applies treasure for the session's player (items → pending vault items, prisoners
→ hero offer, spells learned), which the Ebitengine UI does in its event loop.

**Battles** (`core/game/battle.go`): `BattleSession` runs the battle on `core/combat`: the AI acts
for its units (and for the player's when auto is on); when one of the player's units is up, the
session waits. Commands: `battle-act {x, y}` (move/attack like a click; refused if nothing
happens), `battle-done`, `battle-wait`, `battle-flee`, `battle-auto`. `BattleSession.View()` /
`EncodeView()`: units with ids, positions, health, moves; the selected unit; state. Battles
between two other players still auto-resolve.

**Questions**: `confirm-encounter {enter}`, `raze-city {raze}`.

**Events**: `battle-started`, `battle-ended` (with result), `choose-encounter`, `choose-raze`
(`GameEventBattle`, `GameEventChooseEncounter`, `GameEventChooseRaze`). Protocol doc has a new
"Sessions" section.

## Checks done

- `go build ./... && go vet ./...`, `go test ./core/... ./lib/... ./game/...` under Xvfb: pass;
  CI guard empty; `TestHeadlessGame` with data passes; `go test -race` on the session tests passes.
- Session tests: a battle paused and finished with wait + auto; declining a lair; a manual
  battle move then fleeing (the stack steps back); capturing a city (battle, then raze-or-keep).
- No Ebitengine code changed: nothing new to smoke test by hand.

## Not done / next

1. **Combat spells in battles** through commands (wizard and unit casting; needs targeting as data,
   like the overland `CastSpell`).
2. **Wasm bridge** (`GOOS=js`): a `CommandSession` per human player plus the turn loop (AI turns
   via `DoAiTurn`, like the headless game) behind `execute(json)`, `events()`, `view()`,
   `map(plane)`, `battle()`. Data from `core/gamedata` (the exported JSON, loaded by the page).
3. The diplomacy steps the other wizard starts (proposals to the player) have no events/answers yet.
