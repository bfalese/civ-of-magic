# Handoff 042: the AI's spells replayed by name; spells that would do nothing aren't offered

Date: 2026-11-09
Previous: [041](041-battle-spells.md). Start-here doc: [034](034-start-here.md).

## State

- #47 (spell animations, target-aware castable) is merged. This round: branch `ai-spells`, PR
  against `master`. These are the two open items 041 listed.

## What changed

1. **AI spells as spell steps**: the combat model notes what the AI cast and on what
   (`SpellCast`, filled in as `DoAITargetUnitSpell` / `DoAITargetTileSpell` pick), and tells the
   actions passed to `Update` if they implement `combat.SpellCastObserver` (`BeforeSpellCast`,
   then `SpellCast` once the projectiles landed), for the wizard's casts (`doAiCast`) and units'
   charges (`doAI`). Fizzles aren't reported. The battle recorder makes them `spell` steps with
   `target`, `tile` and `effect`, so the page animates them like yours (they used to be anonymous
   `effect` steps with a glow). `TestHostAISpellSteps` (siege, the defender knows only Fire Bolt).
2. **`combat.SpellCanWork`** (`core/combat/probe.go`) replaces the bare target check: why casting
   now would do nothing, or "". Besides "nothing to cast it on here": area spells with nobody to
   touch ("nothing it would affect here": Flame Strike, Holy Word, Death Spell, probed through
   `DoAllUnitsSpell`), Mass Healing with nobody wounded, combat enchantments already in effect
   (`enchantmentSpells`), walls of fire / darkness already up, no dead to raise.
   `TestProbedSpellsList` now also checks the area and enchantment lists against `InvokeSpell`.
3. **Battle view**: `unusable` (the reason) replaces 041's `no-target`; `castable` is false and
   `battle-cast` refuses with the reason. The page shows it in the spell's tooltip.
4. **The AI** skips spells `SpellCanWork` rejects, so it no longer wastes its cast chance on them.

## Notes

- Disenchant Area / True, Call Chaos and the summons are always offered (they can't be judged
  cheaply, or always do something).
- The wall spells are only offered to a defending city (their eligibility); when one is already
  up the other may still be cast.

## Next

034 step 7: the 3D/retro look (027 step 5), Ebitengine xdotool smoke tests (027 step 6), the small
open items from 027 step 7. Also open: `docs/core-review.md` (diplomacy decisions).
