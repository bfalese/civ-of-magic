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

func TestApplyDrainPower(test *testing.T) {
    human, enemy := makeContactPlayers()
    model := &GameModel{}

    enemy.Mana = 30
    drained, ok := model.ApplyDrainPower(enemy)
    if !ok || drained != 30 || enemy.Mana != 0 {
        test.Errorf("drain capped at the target's mana: drained=%v ok=%v mana=%v", drained, ok, enemy.Mana)
    }

    enemy.Mana = 1000
    drained, _ = model.ApplyDrainPower(enemy)
    if drained < 50 || drained > 150 || enemy.Mana != 1000 - drained {
        test.Errorf("drain must be 50-150: drained=%v mana=%v", drained, enemy.Mana)
    }

    human.Banished = true
    if _, ok := model.ApplyDrainPower(human); ok {
        test.Errorf("a banished wizard can't be targeted")
    }
}

func TestApplySpellBlastNeedsASpellInProgress(test *testing.T) {
    human, enemy := makeContactPlayers()
    model := &GameModel{}

    human.Mana = 500
    if model.ApplySpellBlast(human, enemy) {
        test.Errorf("target is not casting anything: spell blast must not apply")
    }
    if human.Mana != 500 {
        test.Errorf("caster mana changed: %v", human.Mana)
    }
}
