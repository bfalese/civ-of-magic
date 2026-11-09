# Handoff 038: the diplomacy screen

Date: 2026-11-09
Previous: [037](037-city-extras.md). Start-here doc: [034](034-start-here.md) (this is its step 4).

## State

- #41, #42 and #43 are merged. This round: branch `diplomacy`, PR against `master`.

## What changed

1. **Player view `diplomacy`** (`core/game/diplomacyview.go`): per known wizard the treaty, how
   they feel about you (relation and mood name), peace turns left, the gold a tribute would give,
   your spells they lack (for tributes and trades), whether they know a spell you don't, the
   pending trade offer, their global enchantments, fame and city count.
2. **Wizards screen** (`frontend/web/diplomacy.js`, Wizards button or W): a card per wizard with
   the mood on a red-to-green meter, propose pact / alliance / peace, a gold tribute, a spell to
   give or trade; a trade offer shows on the card with accept / decline (the dialog for the
   `trade-offer` event is skipped while the screen is open).
3. Top bar: buttons and gaps a bit tighter, research cut at 11em, so it still fits one line at
   1200 px with the new button.
4. `TestHostDiplomacy`; smoke steps in the spellbook run (pact, spell tribute, trade).

## Notes

- A tribute raises `StartingRelation` (the long-term attitude); the visible mood drifts toward it
  over the next turns, so the meter doesn't jump right away.
- The other wizard still always agrees to treaties (core `ProposeTreaty`); trades depend on their
  trade interest. Making AI diplomacy answer for real is core work for later.
- No wizard portraits yet (the names are in the banner colour).

## Next

034's list from step 5: a start screen (wizard, opponents, difficulty) instead of URL
parameters, soft fog edges (`mapback.lbx` 0-13); then battle spell animations and a target-aware
`castable`.
