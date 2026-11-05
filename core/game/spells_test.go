package game

import (
    "testing"

    "github.com/bfalese/civ-of-magic/core/data"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/spellbook"
)

func TestCheckInstantFizzleWithoutEnchantments(test *testing.T) {
    human, enemy := makeContactPlayers()
    model := &GameModel{Players: []*playerlib.Player{human, enemy}}

    spell := spellbook.Spell{Name: "Fire Bolt", Magic: data.ChaosMagic, CastCost: 10}
    fizzled, _ := model.CheckInstantFizzle(human, spell)
    if fizzled {
        test.Errorf("no enemy enchantments: the spell must not fizzle")
    }
}

func TestChooseSummonHero(test *testing.T) {
    human, _ := makeContactPlayers()
    model := &GameModel{}

    human.HeroPool = make(map[herolib.HeroType]*herolib.Hero)
    if hero := model.ChooseSummonHero(human, false); hero != nil {
        test.Errorf("empty pool: expected nil, got %v", hero.HeroType)
    }

    // Torin is never summonable this way
    human.HeroPool[herolib.HeroTorin] = &herolib.Hero{HeroType: herolib.HeroTorin, Status: herolib.StatusAvailable}
    if hero := model.ChooseSummonHero(human, true); hero != nil {
        test.Errorf("only Torin available: expected nil, got %v", hero.HeroType)
    }
}
