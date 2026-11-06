package game

import (
    "errors"
    "testing"

    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/spellbook"
)

func TestCastUnitSpell(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    stack := player.FindStack(3, 3, data.PlaneArcanus)
    unit := stack.Units()[0]
    bless := spellbook.Spell{Name: "Bless", Magic: data.LifeMagic}

    if err := model.CastSpell(player, bless, SpellTarget{}); !errors.Is(err, ErrSpellNeedsTarget) {
        test.Errorf("a unit spell without a target: %v", err)
    }
    if err := model.CastSpell(player, bless, SpellTarget{Unit: unit}); err != nil {
        test.Fatalf("bless: %v", err)
    }
    if !unit.HasEnchantment(data.UnitEnchantmentBless) {
        test.Errorf("the unit should be blessed")
    }
    if err := model.CastSpell(player, bless, SpellTarget{Unit: unit}); err == nil {
        test.Errorf("blessing twice should be refused")
    }

    enemyUnit := model.Players[1].Stacks[0].Units()[0]
    if err := model.CastSpell(player, bless, SpellTarget{Unit: enemyUnit}); err == nil {
        test.Errorf("blessing an enemy unit should be refused")
    }

    rule, ok := model.UnitSpellRule(spellbook.Spell{Name: "Lycanthropy"})
    if !ok || rule.Check(unit) == nil {
        test.Errorf("drakes (fantastic) can't get Lycanthropy")
    }
}

func TestCastCitySpell(test *testing.T) {
    model, player, city := makeCommandTestModel()
    here := &Location{Plane: data.PlaneArcanus, X: 5, Y: 5}

    wall := spellbook.Spell{Name: "Wall of Fire", Magic: data.ChaosMagic}
    if err := model.CastSpell(player, wall, SpellTarget{Location: here}); err != nil {
        test.Fatalf("wall of fire: %v", err)
    }
    if !city.HasEnchantment(data.CityEnchantmentWallOfFire) {
        test.Errorf("the city should have a wall of fire")
    }
    if err := model.CastSpell(player, wall, SpellTarget{Location: here}); err == nil {
        test.Errorf("a second wall of fire should be refused")
    }

    famine := spellbook.Spell{Name: "Famine", Magic: data.DeathMagic}
    if err := model.CastSpell(player, famine, SpellTarget{Location: here}); err == nil {
        test.Errorf("cursing your own city should be refused")
    }
}

func TestCastSpellTargetTypes(test *testing.T) {
    model, player, _ := makeCommandTestModel()

    cases := map[string]LocationType{
        "Bless": LocationTypeFriendlyUnit,
        "Lycanthropy": LocationTypeFriendlyUnit,
        "Wall of Fire": LocationTypeFriendlyCity,
        "Famine": LocationTypeEnemyCity,
        "Black Wind": LocationTypeEnemyUnit,
    }
    for name, expected := range cases {
        locationType, ok := model.SpellTargetType(player, spellbook.Spell{Name: name})
        if !ok || locationType != expected {
            test.Errorf("%v: got %v %v, expected %v", name, locationType, ok, expected)
        }
    }

    if _, ok := model.SpellTargetType(player, spellbook.Spell{Name: "Awareness"}); ok {
        test.Errorf("a global enchantment needs no target")
    }

    if err := model.CastSpell(player, spellbook.Spell{Name: "Spell Ward"}, SpellTarget{}); !errors.Is(err, ErrSpellNotSupported) {
        test.Errorf("spell ward still needs the UI: %v", err)
    }
}
