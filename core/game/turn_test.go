package game

import (
    "testing"

    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/spellbook"
)

func TestStartPlayerTurnHeadless(test *testing.T) {
    human, enemy := makeContactPlayers()
    model := makeSpellTestModel(10, 10)
    model.MyrrorMap = model.ArcanusMap
    model.Players = []*playerlib.Player{human, enemy}
    model.Events = make(chan GameEvent, 100)
    // the real game creates players with the model as their global enchantment provider
    human.GlobalEnchantmentsProvider = model
    enemy.GlobalEnchantmentsProvider = model

    var cast []spellbook.Spell
    castSpell := func(player *playerlib.Player, spell spellbook.Spell) {
        cast = append(cast, spell)
    }

    enemy.Skip = true
    if model.StartPlayerTurn(enemy, castSpell) {
        test.Errorf("a skipping player's turn should not start")
    }

    // a spell that is fully paid for completes and is handed to the frontend
    spell := spellbook.Spell{Name: "Test Summon", Magic: data.LifeMagic, CastCost: 1}
    human.CastingSpell = spell
    human.CastingSpellProgress = 1
    human.Mana = 100
    human.RemainingCastingSkill = 100

    if !model.StartPlayerTurn(human, castSpell) {
        test.Fatalf("turn should start")
    }
    if len(cast) != 1 || cast[0].Name != "Test Summon" {
        test.Errorf("completed spell should be cast once, got %v", cast)
    }
    if human.CastingSpell.Valid() {
        test.Errorf("casting spell should be cleared after it completes")
    }
}
