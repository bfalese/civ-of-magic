# Handoff 014: spell rules and casting in core, casting commands

Date: 2026-11-06
Previous: [013](013-events-and-views-json.md). Start-here doc: [009](009-start-here-next-session.md).

## State

- PRs #13–#17 are merged into `master`.
- This round: branch `cast-in-core`, PR #18 against `master`. **Don't merge without explicit approval.**
- Wasm bridge stays last (user's request). Hand smoke tests: [`docs/smoke-test.md`](../smoke-test.md).

## What changed

1. **Spell rules in core** (`core/game/castrules.go`), generated from the UI's case lists so names
   and enchantments match exactly (including spelldat's "Invisiblity"):
   - `UnitSpellRule(spell)`: animation index, `Check(unit) error`, `Apply(unit)` for 30 unit
     enchantments, Black Channels (+ undead), Chaos Channels (random of three), Lycanthropy.
   - `CitySpellRule(player, spell)`: friendly/enemy location type, enchantment, `Before` (curses:
     the city may dispel, fizzle event), `After` (Consecration, Nature's Eye) for 13 city
     enchantments + Consecration + Nature's Eye, and 5 curses.
2. **UI `doCastSpell`**: those 53 cases became two generic branches calling the rules; target
   picking, notices and animations are unchanged (664 → 482 lines; unused helpers removed).
3. **`GameModel.CastSpell(player, spell, target)`** (`core/game/castspell.go`): casts a finished
   spell with the target as data (unit or tile). Covers the rules above, summons, Summon
   Hero/Champion, Incarnation, global enchantments, Death Wish, Great Unsummoning, Black Wind,
   Stasis, Ice/Fire Storm, Nature's Cures, Plane Shift, Spell of Return, Earthquake. Others return
   `ErrSpellNotSupported`. `SpellTargetType` says what target a spell needs.
4. **Headless game** casts through `CastSpell` (AI's unit target, or the first city without the
   enchantment for city spells). 300 years: no spells skipped.
5. **Casting commands**: `cast-spell {spell, target?}` (cast now if mana and skill allow, else start
   a multi-turn cast) and `cast-spell-target {target}`. `GameModel.FinishCast` is the castSpell hook
   for command frontends: spells needing a target become `PendingSpells[player]` and emit
   `choose-spell-target` (`GameEventChooseSpellTarget`, with JSON). Targets: `{"location", "unit"}`
   with the unit's index in the stack. `LocationType.Name()` added. Protocol doc updated.

## Checks done

- `go build ./... && go vet ./...`, `go test ./core/... ./lib/... ./game/...` under Xvfb: pass;
  CI guard empty; `TestHeadlessGame` with data passes.
- `-watch` 5 min (94 years): AI overland unit enchantments go through the new branch, no errors.
- Not testable headless: the human target picking for unit/city spells (smoke-test.md §6 covers it:
  unit enchantment, city enchantment, curse).

## Next (wasm last)

1. **The rest of `doCastSpell`** into `CastSpell`: Spell Ward (ward choice), map spells (Change
   Terrain, Transmute, Raise Volcano, Corruption, Earth Lore, Floating Island, Disenchant Area,
   Warp Node...), and the list spells (Subversion, Spell Binding, Disjunction, Resurrection) with
   the choice as part of the target. Then the UI branches can call `CastSpell` too.
2. **More commands**: answer offers (hire hero / mercenaries / merchant accept-decline),
   planar travel, unit orders (patrol, wait, disband, split stack).
3. **Route the Ebitengine UI through commands** for simple actions.
4. **Wasm bridge.**
