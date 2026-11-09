# Handoff 029: battles the AI starts against you

Date: 2026-11-08
Previous: [028](028-battle-spells-page.md). Start-here doc: [027](027-start-here.md) (this round did its step 2).

## State

- PRs #34 (handoff 027) and #35 (combat spells in the battle screen) are merged.
- This round: branch `ai-battles`, PR against `master`. **Don't merge without approval.**

## What changed

1. **AI turns inside the session's coroutine** (`core/game/host.go`). `CommandSession.EndTurn` is
   now `host.playOthers` itself, not a flag the host acted on after the command. `end-turn` is a
   command like any other, so the AI turns run inside the session's coroutine and can pause there.
2. **`hostAiHandlers`**: the `MovementHandler` for the AI players' moves. It embeds the
   `HeadlessGame` and overrides `DoCombat`: a battle with the human on either side goes to
   `CommandSession.DoCombat`, which waits for battle commands (`WaitingFor` = `battle`, a
   `battle-started` event with the human as defender). Other battles are still fought
   automatically. When the battle ends, the AI turns go on, and the command that ended it returns
   at the start of the human's next turn.
3. A captured city goes through `CommandSession.DefeatCity`, which already lets an AI attacker
   decide raze or keep.
4. **Page**: a log line "<wizard> attack you at x,y!" (the battle screen opens through the
   existing waiting-for-battle path). The smoke test's end-turn loops auto-finish any battle an AI
   starts (`settle`).
5. **Tests**: `TestHostAiBattle` (no gamedata needed). An attack during end-turn pauses before the
   rest of the turn, the view says you defend, other commands are refused, then Done a few times
   and Auto; the turn finishes and the drakes win. `TestGameHost` answers any AI battle with
   `battle-auto`.
6. `docs/frontend-protocol.md`: end-turn may wait for a battle.

Checked by hand with a throwaway test: 6 games × up to 250 turns at impossible difficulty against
4 AIs, with an idle human. AI attacks paused the turn 6 times; auto resolved them and the turn
loop carried on. Cities that fell without a battle were empty (the idle human's units were gone).

## Not yet / notes

- The browser smoke test doesn't start an AI attack (it would take many random turns). The Go
  tests cover the pause, and the page opens the battle screen through the same path as your own
  attacks.
- `HeadlessGame.Stats` doesn't count the human's battles (only used by headless runs).
- Overland spells an AI casts on the human (e.g. damaging a stack) don't involve a battle and
  aren't affected.

## Next

027's step 3 (battle screen polish: walls, animations or combat events, a battle log, unit
details), then step 4 (web frontend gaps).
