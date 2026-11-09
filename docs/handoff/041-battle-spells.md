# Handoff 041: spell animations in battle; spells that know if they have a target

Date: 2026-11-09
Previous: [040](040-fog-edges.md). Start-here doc: [034](034-start-here.md) (this is its step 6).

## State

- #46 (fog edges) is merged. This round: branch `battle-animations`, PR against `master`.
  (An old local branch `battle-spells` from round 023 exists; unrelated.)
- **Re-export the assets**: 5125 sprites expected (spell effects added).

## What changed

1. **`combat.SpellHasTarget`** (`core/combat/probe.go`): whether a combat spell has anything to
   be cast on now. `InvokeSpell` runs in a probe mode where `DoTargetUnitSpell` /
   `DoTargetTileSpell` only check their filters against the field and return, and the dispel roll
   is skipped. Only the spells in `probedSpells` (those whose case does nothing but pick a target)
   are probed, so a probe can't change the battle; `TestProbedSpellsList` scans `InvokeSpell`'s
   source and fails if a targeted spell is missing from the list or a listed one stops asking.
   Raise Dead and Animate Dead (they pick among dead units) have their own small checks.
2. **Battle view**: a spell with nothing to target is `castable: false` with `no-target: true`
   (wizard spells, unit spells and item charges); `battle-cast` refuses it before anything waits.
   The page's tooltip says "nothing to cast it on here".
3. **Spell animations**: the desktop battle screen's effect frames (`cmbtfx.lbx` / `specfx.lbx`,
   the explode part) exported as `spell-effect/<spell>` (table `spellEffects` in
   `util/export-assets`). A `spell` step now carries `target`, `tile` and `effect`; the battle
   page plays the animation on the target (or tile) and on every unit in `changes` (Flame Strike,
   Death Spell, ...), the step lasting as long as the animation (70 ms a frame, at least 520 ms).
   Without the sprite (old export) it falls back to the glow.
4. The battle scenario also teaches Disrupt (no walls at a lair: greyed out). Tests: probe tests
   in `core/combat`, `TestHostBattleScenario` (Disrupt refused, the Fire Bolt step's target, tile
   and effect), smoke checks for both.

## Notes

- The AI's spells still replay as anonymous `effect` steps (a glow): the recorder only sees
  health changes. Naming them needs the AI cast path (`doAiCast`) to report what it cast.
- Spells without a target that do nothing useful (e.g. a wall spell when attacking) are still
  castable; `shouldAICastSpell` has some of those rules and could be shared.

## Next

034 step 7: the 3D/retro look (027 step 5), Ebitengine xdotool smoke tests (027 step 6), the small
open items in 027 step 7. Also open: `docs/core-review.md` (diplomacy decisions), AI spell
animations (above).
