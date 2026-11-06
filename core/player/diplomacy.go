package player

import (
    "errors"
    "fmt"
    "math/rand/v2"

    "github.com/bfalese/civ-of-magic/core/spellbook"
)

// What a wizard can do at the diplomacy table with another wizard (the Ebitengine diplomacy
// screen and the diplomacy commands use these). The other wizard currently agrees to every
// treaty proposed.

// propose a treaty to other: "pact", "alliance" or "peace"
func (player *Player) ProposeTreaty(other *Player, treaty string) error {
    switch treaty {
        case "pact":
            other.PactWithPlayer(player)
            player.PactWithPlayer(other)
        case "alliance":
            other.AllianceWithPlayer(player)
            player.AllianceWithPlayer(other)
        case "peace":
            relationship, ok := other.GetDiplomaticRelation(player)
            if !ok {
                return fmt.Errorf("%v doesn't know you", other.Wizard.Name)
            }
            relationship.PeaceCounter = 50
        default:
            return fmt.Errorf("unknown treaty %q", treaty)
    }
    return nil
}

// both wizards think better of each other after a tribute
func (player *Player) thanksForTribute(other *Player) {
    other.AdjustDiplomaticRelation(player, 5)
    player.AdjustDiplomaticRelation(other, 5)
}

// the gold a tribute of gold is worth: a tenth of the treasury, 0 if that's 5 or less
func (player *Player) TributeGoldAmount() int {
    gold := int(float64(player.Gold) * 0.1)
    if gold > 5 {
        return gold
    }
    return 0
}

// give other a tenth of the treasury; returns the amount
func (player *Player) TributeGold(other *Player) (int, error) {
    gold := player.TributeGoldAmount()
    if gold == 0 {
        return 0, errors.New("not enough gold for a tribute")
    }
    player.Gold -= gold
    other.Gold += gold
    player.thanksForTribute(other)
    return gold, nil
}

// teach other a spell the player knows and they don't, as a tribute
func (player *Player) TributeSpell(other *Player, spell spellbook.Spell) error {
    if !player.KnownSpells.Contains(spell) || other.KnownSpells.Contains(spell) {
        return fmt.Errorf("%v can't be given to %v", spell.Name, other.Wizard.Name)
    }
    other.KnownSpells.AddSpell(spell)
    player.thanksForTribute(other)
    return nil
}

// whether other is in the mood to trade spells right now (a roll against their trade interest)
func (player *Player) WillTradeSpells(other *Player) bool {
    relationship, ok := other.GetDiplomaticRelation(player)
    return ok && rand.N(100) < relationship.TradeInterest
}

// a spell other knows and the player doesn't, picked at random; false if there is none
func (player *Player) SpellOfferedBy(other *Player) (spellbook.Spell, bool) {
    var choices []spellbook.Spell
    for _, spell := range other.KnownSpells.Spells {
        if !player.KnownSpells.Contains(spell) {
            choices = append(choices, spell)
        }
    }
    if len(choices) == 0 {
        return spellbook.Spell{}, false
    }
    return choices[rand.N(len(choices))], true
}

// exchange give for received: both wizards learn the other's spell, and other is less keen on
// the next trade
func (player *Player) TradeSpells(other *Player, give spellbook.Spell, received spellbook.Spell) error {
    if !player.KnownSpells.Contains(give) || other.KnownSpells.Contains(give) {
        return fmt.Errorf("%v can't be traded to %v", give.Name, other.Wizard.Name)
    }
    if !other.KnownSpells.Contains(received) || player.KnownSpells.Contains(received) {
        return fmt.Errorf("%v can't trade you %v", other.Wizard.Name, received.Name)
    }

    player.KnownSpells.AddSpell(received)
    other.KnownSpells.AddSpell(give)

    if relationship, ok := other.GetDiplomaticRelation(player); ok {
        relationship.TradeInterest = max(-100, relationship.TradeInterest - 20)
    }
    return nil
}
