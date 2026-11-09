# Handoff 035: spell choices and the item designer

Date: 2026-11-09
Previous: [034](034-start-here.md) (still the start-here doc; this is its step 1).

## State

- **PR #40 (`magic-screen`) is still open**: the session couldn't merge it without your approval.
  This round's branch `spell-choices` is stacked on it, and its PR targets `magic-screen`. Merge
  #40 first, then retarget this one to `master`
  (`gh api -X PATCH repos/bfalese/civ-of-magic/pulls/N -f base=master`). The squash will leave
  #40's commits in the diff until you rebase: `git rebase --onto master magic-screen spell-choices`.
- **Re-export the assets** (`util/export-assets`): item pictures were added (5046 sprites expected).

## What changed

1. **Spells cast with a choice** (Spell Ward, Spell Binding, Disjunction / True, Resurrection):
   each `SpellView` has `choice-spell` and `choices`, the targets `cast-spell` accepts now (Spell
   Ward: every ward each of your cities can take, with the city; Binding / Disjunction: the global
   enchantments of wizards you know, minus the ones you already have for Binding; Resurrection:
   your dead heroes, none without a free hero slot). `core/game/spellchoices.go`.
   - The magic screen asks in the dialog (`City: Ward`, `Wizard's Enchantment`, hero name); with
     no choices the spell is disabled ("nothing to cast it on now").
   - Cast over several turns: when `choose-spell-target` comes, the page asks the same way and
     answers with `cast-spell-target`.
2. **Item designer** (`frontend/web/items.js`) for Create Artifact and Enchant Item, opened from
   the spellbook: type, picture (sprites `item/N`), name, one power per group from a dropdown (the
   ones the type can't have or that need spellbooks are disabled, Enchant Item's cost limit too),
   the Ability 3 group as checkboxes, staff/wand spell charges, 4 powers at most, the cost and
   whether it's cast now. The player view's `item-design` holds the options and prices and
   `cost-factor` the retort/spellbook discount; the cost the page shows is the cost the core charges
   (the test checks to within 1).
3. **Assets**: `assets.Item(index)` and `items.lbx` 0-115 in `util/export-assets`.
4. `artifact.DesignImages`, `artifact.IsAbility` (was unexported), `artifact.SpellChargeCost`.
5. **Scenario `spellbook`**: the four choice spells and the two item spells, 5000 mana and plenty of
   casting skill this turn, a dead hero, a known rival with Detect Magic.
   `TestHostSpellbookScenario` casts each from the view's own choices and makes a sword from the
   view's options; the smoke test does Spell Ward, Resurrection and a sword through the page
   (about 12 s now, 22 s with `--shots`).

## Notes

- Empty `choices` are left out of the JSON (`omitempty`): read `choices || []` together with
  `choice-spell`.
- The core lets Enchant Item take abilities (only plain powers have the cost limit), as
  `artifact.BuildDesign` already did; the original game may differ.
- The made item goes to the vault (`vault` event). The page has no vault/equipment screen yet, so
  the item just waits there (next step).

## Next

The rest of 034's list, from step 2: hero equipment and the vault (the new item pictures help),
disband, split stacks, unit details on the map; then city screen extras, diplomacy, start screen,
battle spell animations.
