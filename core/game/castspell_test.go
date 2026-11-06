package game

import (
    "errors"
    "testing"

    buildinglib "github.com/bfalese/civ-of-magic/core/building"

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

    if err := model.CastSpell(player, spellbook.Spell{Name: "Spell of Mastery"}, SpellTarget{}); !errors.Is(err, ErrSpellNotSupported) {
        test.Errorf("spell of mastery still needs the UI: %v", err)
    }
    if err := model.CastSpell(player, spellbook.Spell{Name: "Spell Ward"}, SpellTarget{}); !errors.Is(err, ErrSpellNeedsTarget) {
        test.Errorf("spell ward needs a city: %v", err)
    }
}

func TestCastWizardSpell(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    enemy := model.Players[1]
    drain := spellbook.Spell{Name: "Drain Power", Magic: data.ChaosMagic}

    if err := model.CastSpell(player, drain, SpellTarget{Wizard: enemy}); err == nil {
        test.Errorf("a wizard you haven't met can't be targeted")
    }

    player.AwarePlayer(enemy)
    enemy.Mana = 500
    if err := model.CastSpell(player, drain, SpellTarget{Wizard: enemy}); err != nil {
        test.Fatalf("drain power: %v", err)
    }
    if enemy.Mana >= 500 {
        test.Errorf("the enemy should lose mana, has %v", enemy.Mana)
    }
    if err := model.CastSpell(player, drain, SpellTarget{Wizard: player}); err == nil {
        test.Errorf("you can't target yourself")
    }
}

func TestCastMapSpell(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    player.LiftFogSquare(7, 7, 1, data.PlaneArcanus)
    lore := spellbook.Spell{Name: "Earth Lore", Magic: data.NatureMagic}

    // the view is on the other plane: the cast still uses the target's plane, and restores the view
    model.Plane = data.PlaneMyrror
    if err := model.CastSpell(player, lore, SpellTarget{Location: &Location{Plane: data.PlaneArcanus, X: 2, Y: 2}}); err != nil {
        test.Fatalf("earth lore: %v", err)
    }
    if !player.IsExplored(0, 0, data.PlaneArcanus) {
        test.Errorf("earth lore should reveal the area")
    }
    if model.Plane != data.PlaneMyrror {
        test.Errorf("the viewed plane should be restored")
    }

    corruption := spellbook.Spell{Name: "Corruption", Magic: data.ChaosMagic}
    if err := model.CastSpell(player, corruption, SpellTarget{Location: &Location{Plane: data.PlaneArcanus, X: 7, Y: 7}}); err != nil {
        test.Fatalf("corruption: %v", err)
    }
    tile := model.ArcanusMap.GetTile(7, 7)
    if !tile.Corrupted() {
        test.Errorf("the tile should be corrupted")
    }
}

func TestCastSpellWardAndResurrection(test *testing.T) {
    model, player, city := makeCommandTestModel()
    here := &Location{Plane: data.PlaneArcanus, X: 5, Y: 5}
    ward := spellbook.Spell{Name: "Spell Ward", Magic: data.SorceryMagic}

    if err := model.CastSpell(player, ward, SpellTarget{Location: here, Choice: "No Such Ward"}); err == nil {
        test.Errorf("an unknown ward should be refused")
    }

    choices := model.AvailableSpellWards(city).Values()
    if len(choices) > 0 {
        if err := model.CastSpell(player, ward, SpellTarget{Location: here, Choice: choices[0].Name()}); err != nil {
            test.Fatalf("spell ward: %v", err)
        }
        if !city.HasEnchantment(choices[0]) {
            test.Errorf("the city should have %v", choices[0].Name())
        }
    }

    if err := model.CastSpell(player, spellbook.Spell{Name: "Resurrection", Magic: data.LifeMagic}, SpellTarget{Choice: "Nobody"}); err == nil {
        test.Errorf("resurrecting a hero who isn't dead should be refused")
    }
}

func TestWordOfRecallRule(test *testing.T) {
    model, player, city := makeCommandTestModel()
    rule, ok := model.UnitSpellRule(spellbook.Spell{Name: "Word of Recall"})
    if !ok {
        test.Fatalf("word of recall should be a unit spell")
    }
    unit := player.FindStack(3, 3, data.PlaneArcanus).Units()[0]

    if rule.Check(unit) == nil {
        test.Errorf("without a summoning circle the unit can't be recalled")
    }

    city.Buildings.Insert(buildinglib.BuildingSummoningCircle)
    if err := rule.Check(unit); err != nil {
        test.Fatalf("recall: %v", err)
    }
    rule.Apply(unit)
    if unit.GetX() != 5 || unit.GetY() != 5 {
        test.Errorf("the unit should be at the summoning circle, is at %v,%v", unit.GetX(), unit.GetY())
    }
}
