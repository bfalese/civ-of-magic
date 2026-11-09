# Handoff 033: magic screen and spellbook, first-turn research

Date: 2026-11-08
Previous: [032](032-battle-walls.md). Start-here doc: [027](027-start-here.md) (this starts its step 4).

## State

- PR #39 (walls, siege scenario) is merged. Re-export the assets if you haven't since.
- This round: branch `magic-screen`, PR against `master`. **Don't merge without approval.**

## What changed

1. **First research at the start**: `MakeGameHost` emits `choose-research` when the human has
   nothing to research (the turn loop asks only from the second turn). The page already answers it
   with its question dialog.
2. **Player view, magic data** (`WizardView`): `power`, `power-distribution`, `research-per-turn`,
   `skill-per-turn`, `remaining-casting-skill`, `casting-cost`, `research-candidates`,
   `alchemy-full-rate`, and `spells` (overland spells: name, realm, section, effective cost,
   target kind from `SpellTargetType`, `now` = cast at once, `needs-design` for Create
   Artifact / Enchant Item).
3. **Magic screen** (`frontend/web/magic.js`, Magic button or M): sliders for the three power
   shares (changing one keeps the sum at 1, the others share the rest in their old proportion)
   with per-turn gains; mana, skill left this turn, the spell being cast; alchemy both ways; the
   spellbook by section, coloured by realm, cost and "over turns" when it can't be cast now.
   - No target: `cast-spell` (now, or started over the next turns).
   - Target, can't cast now: started without one; the core asks with `choose-spell-target` when
     it's finished (the page's existing path).
   - Target, can cast now: the screen closes and the target is picked on the map (or a wizard in a
     dialog); the click sends `cast-spell` with it.
   - Disabled, with a tooltip: item spells (need a design) and targets the page can't pick yet
     (`dead-hero`, `enemy-global-enchantment`).
4. **Scenario `battle`** also teaches Wall of Stone and raises the casting skill to about 60,
   available this turn (the turn had already set the remaining skill before the scenario).
5. **Smoke test**: checks the first research question at load (scenario loads answer it); opens
   the magic screen, moves the research share and checks the shares still add up to 1, casts or
   starts a spell without a target, does alchemy (2 gold to 1 mana), Esc closes. In the battle
   scenario: casts Wall of Stone from the spellbook on the capital by clicking it, and checks the
   city got City Walls.
6. `TestGameHost` checks the magic data and the first `choose-research`.

## Notes

- A smoke run is about 9.5 s; with `--shots DIR` about 18.5 s (each screenshot about 0.8 s with
  software rendering). Use `--shots` to debug, not for routine runs.
- Spells needing a choice (Spell Ward's ward, Spell Binding / Disjunction's enchantment,
  Resurrection's hero) can't be cast from the page yet: `cast-spell` takes `target.choice`, but
  the page has no dialog to pick one.

## Next (rest of 027 step 4)

- Choices for spells (above) and the item design screen for Create Artifact / Enchant Item.
- Armies and items: hero equipment, the vault, disbanding, splitting stacks.
- Rename city, city enchantments in the city screen; tax rate; diplomacy for known wizards.
- Soft fog edges; a start screen instead of URL parameters.
