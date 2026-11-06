# Handoff 010: hands-off watch mode, battles/movement/AI/encounters in core/game

Date: 2026-11-06
Previous: [009](009-start-here-next-session.md) (start-here doc; still valid for running the game and the workflow)

## State

- PR #13 is merged into `master`.
- This round: **PR #14** (`game-go-split-2` → `master`), waiting for the user's smoke test.
  **Don't merge without explicit approval.**

`game/magic/game/game.go`: 6.8k → 6.0k lines.

## What changed

1. **`-watch` runs hands-off** (`game/magic/game/watch.go`). In watch mode the "human" is only an
   observer, so `skipInWatchMode` drops dialogs addressed to it (diplomacy, research, notices,
   scrolls, new building, hero level-up, vault, outpost, banish, the human's hire/merchant offers).
   The treasure popup and first-meeting diplomacy (`playFirstMeeting`) are skipped too.
   Headless runs: 123 turns in 6 min before the refactor steps below, 156 turns in 7 min after them,
   with battles, AI spells and research. No errors.

2. **Model code moved out of `game.go`** (UI keeps screens and passes the UI steps in as hooks):

| core/game file | What moved | Frontend hooks |
|---|---|---|
| `combat.go` | `MakeBattle`, `Battle.Run` (headless strategic/tactical), `ApplyBattleOutcome` (capture, fame, side changes, collateral damage), `ApplyBattleAftermath` (recalls, dead units, undead/zombies, experience), `DefeatCity` | `defeatCity` func (raze dialog for humans) |
| `move.go` | `MoveSelectedStack` (was `doMoveSelectedUnit`) | `PlayerMovementHandler` = `MovementHandler` + `ConfirmEncounter`, `SteppedStack`, `NextUnit` |
| `ai.go` | `ApplyAiDecisions`, `MoveAiStacks`, `DoAiTurn` (all in one, headless), `CreateOutpost`, `AnnounceVisibleWizards` | `suggestCityName` (city names come from LBX), `MovementHandler` |
| `encounter.go` | `EncounterDefenders`, `ResolveEncounter`, `CreateTreasure`, `ApplyTreasure` | `TreasureHandler` (vault, prisoner hero, learn spell screens); spells/artifact powers passed in |
| `build.go` | `DoBuildAction` (returns true → UI opens road builder) | `suggestCityName` |
| `road.go` | `FindRoadPath` (promoted with `promote-methods.py`) | |

Events moved to `core/game/gameevents.go`: `GameEventShowBanish`, `GameEventNewOutpost`,
`GameEventTreasure` (`test/overworld` updated). `MovementHandler.DiscoverWizards` now returns bool.

Small ordering changes (no visible effect): after a battle, experience is now added before the
undead animation plays; a cleared plane tower is removed from the other plane before its treasure
is rolled. Both only queue events or touch state the screens don't read.

**Headless tests** (core/game): strategic and tactical battle, stack walking and attacking,
AI decisions + moves, a full `DoAiTurn`, encounter survivors, AI treasure, build action.

## Checks done

- `go build ./... && go vet ./...`, `go test ./core/... ./lib/... ./game/...` under Xvfb: pass.
- CI guard `go list -deps ./core/... | grep -E 'ebiten|/lbx'`: empty.
- `-watch` headless run (see above). Watch mode doesn't cover the human-only paths: the lair confirm
  dialog, the raze dialog, the combat end screen, treasure/vault screens, the build button.

## Smoke test (by hand, together with #13's list in 009)

Most valuable on this branch: attack a lair (confirm dialog, battle, treasure popup, vault if an
item drops), attack an enemy stack and a city (raze/capture dialog, end screen with fame), lose a
hero in battle (equipment notice), settle with settlers (B), build a road with engineers, meld a
node, walk a multi-turn path.

## Next

- `game.go` is now mostly screens. Remaining logic is in the nine smaller UI files and
  `cityview/city-screen.go` (see 008).
- Use `-watch` after each refactor step (`timeout 300 ./magic -data ... -watch -music=false` under
  Xvfb, check the log for `Year=` advancing and no panics).
- `DoAiTurn` + `Battle.Run` make a fully headless AI-vs-AI game loop possible: a small driver in
  core (or a test) that runs `DoNextTurn` with AI turns would make a regression test without
  Ebitengine. Needs building infos and city names without LBX.
- Later (001): command/event API on `core/game`, wasm bridge.
