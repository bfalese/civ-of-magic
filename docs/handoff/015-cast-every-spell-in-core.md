# Handoff 015: almost every overland spell castable from core

Date: 2026-11-06
Previous: [014](014-spell-casting-in-core.md). Start-here doc: [009](009-start-here-next-session.md).

## State

- PRs #13–#18 are merged into `master`.
- This round: branch `cast-rest`, PR #19 against `master`. **Don't merge without explicit approval.**
- Wasm bridge stays last (user's request). Hand smoke tests: [`docs/smoke-test.md`](../smoke-test.md).

## What changed

`GameModel.CastSpell` now covers every overland spell except Spell of Mastery (victory screens)
and Create Artifact / Enchant Item without an item designed in the UI:

- **Map spells**: Earth Lore, Enchant Road, Change Terrain, Transmute, Raise Volcano, Corruption,
  Warp Node, Disenchant Area/True, Floating Island. Targets are checked with the existing
  `CheckSpellTarget` (terrain, explored, ...). These model functions work on the *viewed* plane
  (`model.Plane` via `CurrentMap()`), so `castOnTile` switches the view to the target's plane for
  the cast and restores it. FIXME later: give the `Apply*` map functions a plane parameter.
- **City spells**: Spell Ward (ward by name in `Choice`), Wall of Stone, Summoning Circle, Move
  Fortress, Call the Void.
- **Wizard spells**: Spell Blast, Cruel Unminding, Drain Power, Subversion (`SpellTarget.Wizard`;
  must be a known, active rival).
- **Choice spells**: Spell Binding and Disjunction (True) (`Wizard` = owner, `Choice` = global
  enchantment; same dispel strengths as the UI), Resurrection (`Choice` = hero name).
- **Word of Recall** is a `UnitSpellRule` now; the UI's generic branch handles it and its own case
  is gone. Visible difference: with no summoning city the cast shows a notice instead of silently
  doing nothing.
- New `LocationType`s for non-map targets: `wizard`, `enemy-global-enchantment`, `dead-hero`.
  `GameModel.GetUnitOwner` added.
- Command targets gain `wizard` (banner) and `choice`; `location` is optional now. Headless game
  also targets wizards (first known rival) and enemy cities (first explored).
- Protocol doc lists the target fields per spell.

The UI keeps its own flow for these spells (target screens, animations with effects applied at a
given animation frame); it calls the same model `Apply*` functions.

## Checks done

- `go build ./... && go vet ./...`, `go test ./core/... ./lib/... ./game/...` under Xvfb: pass;
  CI guard empty; `TestHeadlessGame` with data passes; headless 300 years without skipped spells.
- `-watch` 3 min (81 years), no errors.

## Next (wasm last)

1. **More commands**: answer offers (`answer-offer` accept/decline for hero, mercenaries,
   merchant; the events need an id or the model needs pending offers like `PendingSpells`),
   planar travel, unit orders (patrol, wait, disband, split/merge stacks).
2. **Route the Ebitengine UI through commands** for simple actions (tax, city screen buy/sell).
3. **Map spell functions with a plane parameter** (removes the view switch in `castOnTile`).
4. **Wasm bridge.**
