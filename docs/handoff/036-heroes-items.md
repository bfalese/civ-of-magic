# Handoff 036: hero equipment, the vault, splitting and disbanding

Date: 2026-11-09
Previous: [035](035-spell-choices.md). Start-here doc: [034](034-start-here.md) (this is its step 2).

## State

- #40 is merged. **#41 (`spell-choices`, 035) passed CI but is not merged**: the session wasn't
  allowed to merge it. This round's branch `heroes-items` is stacked on it and its PR targets
  `spell-choices`. Merge #41, then retarget this one to `master`
  (`gh api -X PATCH repos/bfalese/civ-of-magic/pulls/N -f base=master`; merging `master` into the
  branch afterwards keeps the diff clean without a force push).

## What changed

1. **Player view `you.items`** (`core/game/vaultview.go`): the four vault slots, the newly
   arrived items (`model.PendingItems`), and each hero with index, unit id, level, location and
   its three slots (kind and the item types they take), items with picture, cost, destroy value
   and powers; the teleport cost and the fortress.
2. **Units in stacks** have `active` and `details` (attack, defense, resistance, hit points,
   figures, moves, level, upkeep, abilities, enchantments). Heroes show hero levels.
3. **Items screen** (`frontend/web/vault.js`, Items button or I): click an item to hold it (the
   slots it doesn't fit are greyed), click a slot to swap (`move-item`), Destroy for mana
   (`destroy-item`). New items are listed at the top; the `vault` event's log line points here.
4. **Stack panel**: a checkbox per unit for `set-unit-active` (unchecked units stay behind when
   the stack moves: that's how stacks split), `i` for the unit's details, `✕` to disband (asks).
5. **Scenario `items`**: a hero who can wield a sword and a shield at the fortress, a plain
   sword in the vault, a shield waiting as a new item. `TestHostItemsScenario` and smoke steps
   equip, destroy, split, inspect and disband (smoke run about 14.5 s).

## Notes

- The model lets new items wait for ever (no end-of-turn handling); the original game forces the
  vault screen. Fine for now.
- Item slot images (`itemisc.lbx` 19-24) aren't exported; empty slots show their kind as text.

## Next

034's list from step 3: rename city, city enchantments, tax rate; diplomacy; a start screen and
soft fog edges; battle spell animations and target-aware `castable`.
