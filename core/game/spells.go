package game

import (
    "math/rand/v2"

    "github.com/bfalese/civ-of-magic/core/data"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/spellbook"
)

// why a spell fizzled when it was cast
type FizzleReason struct {
    // the owner of the enchantment
    Owner *playerlib.Player
    // the enchantment that caused the spell to fizzle
    Enchantment data.Enchantment
}

// checks whether another wizard's enchantment (Tranquility, Life Force, Suppress Magic) makes the
// spell fizzle right away
func (model *GameModel) CheckInstantFizzle(player *playerlib.Player, spell spellbook.Spell) (bool, FizzleReason) {
    var dispelChances []FizzleReason

    for _, checkingPlayer := range model.Players {

        // Tranquility effect: if it's a chaos spell, it should either resist a strength 500 dispel check or fizzle right away.
        if spell.IsOfRealm(data.ChaosMagic) {
            // FIXME: Not sure if multiple instances of Tranquility stack or are checked separately.
            if checkingPlayer != player && checkingPlayer.HasEnchantment(data.EnchantmentTranquility) {
                dispelChances = append(dispelChances, FizzleReason{Owner: checkingPlayer, Enchantment: data.EnchantmentTranquility})
            }
        }
        // Life Force effect: if it's a death spell, it should either resist a strength 500 dispel check or fizzle right away.
        if spell.IsOfRealm(data.DeathMagic) {
            // FIXME: Not sure if multiple instances of Life Force stack or are checked separately.
            if checkingPlayer != player && checkingPlayer.HasEnchantment(data.EnchantmentLifeForce) {
                dispelChances = append(dispelChances, FizzleReason{Owner: checkingPlayer, Enchantment: data.EnchantmentLifeForce})
            }
        }

        if checkingPlayer != player && checkingPlayer.HasEnchantment(data.EnchantmentSuppressMagic) {
            dispelChances = append(dispelChances, FizzleReason{Owner: checkingPlayer, Enchantment: data.EnchantmentSuppressMagic})
        }
    }

    for _, reason := range dispelChances {
        if spellbook.RollDispelChance(spellbook.ComputeDispelChance(500, spell.Cost(true), spell.Magic, &player.Wizard)) {
            return true, reason
        }
    }
    return false, FizzleReason{}
}

// picks a random hero (or champion) the player can summon; nil if there is none.
// Torin is not summonable this way.
func (model *GameModel) ChooseSummonHero(player *playerlib.Player, champion bool) *herolib.Hero {
    var choices []*herolib.Hero
    for _, hero := range player.HeroPool {
        // torin is not summonable through this method
        if hero.Status == herolib.StatusAvailable && hero.IsChampion() == champion && hero.HeroType != herolib.HeroTorin {
            choices = append(choices, hero)
        }
    }

    if len(choices) > 0 {
        return choices[rand.N(len(choices))]
    }

    return nil
}

// effects of a global enchantment taking hold. Awareness and Nature Awareness also reveal wizards,
// which the frontend handles (meeting dialogs).
func (model *GameModel) ApplyGlobalEnchantment(enchantment data.Enchantment, player *playerlib.Player) {
    switch enchantment {
        case data.EnchantmentGreatWasting:
            for _, player := range model.Players {
                player.UpdateUnrest()
            }
        case data.EnchantmentArmageddon:
            for _, player := range model.Players {
                player.UpdateUnrest()
            }
        case data.EnchantmentJustCause:
            player.UpdateUnrest()
        case data.EnchantmentHolyArms:
            // all units implicitly have holy weapon, so the actual enchantment is removed from all units
            for unit := range player.Units() {
                unit.RemoveEnchantment(data.UnitEnchantmentHolyWeapon)
            }
        case data.EnchantmentAuraOfMajesty:
            for _, other := range player.GetKnownPlayers() {
                other.AdjustDiplomaticRelation(player, 10)
                player.AdjustDiplomaticRelation(other, 10)
            }

    }
}
