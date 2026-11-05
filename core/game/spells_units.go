package game

import (
    "math/rand/v2"

    "github.com/bfalese/civ-of-magic/core/combat"
    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/core/units"
)

// Effects of spells that hit units on the overworld. The frontend handles target selection, fizzle
// checks and animations, then calls these at the same point as before.

// Death Wish (global, when the enchantment animation finishes): non-fantastic enemy units must
// resist death magic or die; units in cities the spell can't target are spared
func (model *GameModel) ApplyDeathWish(player *playerlib.Player, spell spellbook.Spell) {
    cityStackInfo := model.ComputeCityStackInfo()

    for _, owner := range model.Players {
        if owner == player {
            continue
        }

        for _, stack := range owner.Stacks {

            city := cityStackInfo.FindCity(stack.X(), stack.Y(), stack.Plane())
            if city != nil && !city.CanTarget(spell) {
                continue
            }

            for _, unit := range stack.Units() {
                ignore := unit.GetRace() == data.RaceFantastic

                if unit.HasEnchantment(data.UnitEnchantmentChaosChannelsDemonWings) ||
                   unit.HasEnchantment(data.UnitEnchantmentChaosChannelsDemonSkin) ||
                   unit.HasEnchantment(data.UnitEnchantmentChaosChannelsFireBreath) {
                       ignore = false
                }

                if ignore {
                    continue
                }

                resistance := combat.GetResistanceFor(unit, data.DeathMagic)
                if rand.N(10) + 1 > resistance {
                    owner.RemoveUnit(unit)
                }
            }
        }
    }
}

// Great Unsummoning (global): fantastic units must resist sorcery at -3 or are unsummoned
func (model *GameModel) ApplyGreatUnsummoning(spell spellbook.Spell) {
    cityStackInfo := model.ComputeCityStackInfo()

    for _, player := range model.Players {
        for _, stack := range player.Stacks {

            city := cityStackInfo.FindCity(stack.X(), stack.Y(), stack.Plane())
            if city != nil && !city.CanTarget(spell) {
                continue
            }

            for _, unit := range stack.Units() {
                if unit.GetRace() == data.RaceFantastic {

                    if unit.HasEnchantment(data.UnitEnchantmentSpellLock) ||
                    unit.HasEnchantment(data.UnitEnchantmentChaosChannelsDemonWings) ||
                    unit.HasEnchantment(data.UnitEnchantmentChaosChannelsDemonSkin) ||
                    unit.HasEnchantment(data.UnitEnchantmentChaosChannelsFireBreath) ||
                    unit.HasAbility(data.AbilityMagicImmunity) {
                        continue
                    }

                    resistance := combat.GetResistanceFor(unit, data.SorceryMagic)
                    if rand.N(10) + 1 > resistance - 3 {
                        player.RemoveUnit(unit)
                    }
                }
            }
        }
    }
}

// Black Wind (after the animation): each unit in the stack must resist death magic at -1 or die
func (model *GameModel) ApplyBlackWind(stack *playerlib.UnitStack, owner *playerlib.Player) {
    for _, unit := range stack.Units() {
        if rand.N(10) + 1 > combat.GetResistanceFor(unit, data.DeathMagic) - 1 {
            owner.RemoveUnit(unit)
        }
    }
}

// Stasis (after the animation)
func (model *GameModel) ApplyStasis(stack *playerlib.UnitStack) {
    // FIXME: maybe apply a Stasis unit enchantment so the user can see the unit is under the stasis effect?
    for _, unit := range stack.Units() {
        unit.SetBusy(units.BusyStatusStasis)
    }
}

// Ice Storm (after the animation): strength 6 cold damage to each unit in the stack
func (model *GameModel) ApplyIceStorm(stack *playerlib.UnitStack, owner *playerlib.Player) {
    model.applyAreaDamageToStack(stack, owner, 6, units.DamageCold)
}

// Fire Storm (after the animation): strength 8 immolation damage to each unit in the stack
func (model *GameModel) ApplyFireStorm(stack *playerlib.UnitStack, owner *playerlib.Player) {
    model.applyAreaDamageToStack(stack, owner, 8, units.DamageImmolation)
}

func (model *GameModel) applyAreaDamageToStack(enemyStack *playerlib.UnitStack, enemy *playerlib.Player, strength int, damage units.Damage) {
    for _, unit := range enemyStack.Units() {
        combat.ApplyAreaDamage(&UnitDamageWrapper{StackUnit: unit}, strength, damage, 0)
        if unit.GetHealth() <= 0 {
            enemy.RemoveUnit(unit)
        }
    }
}
