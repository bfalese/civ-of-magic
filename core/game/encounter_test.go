package game

import (
    "testing"

    "github.com/bfalese/civ-of-magic/core/combat"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/maplib"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/core/units"
)

// a lost battle at a lair removes only the guards that died
func TestResolveEncounterKeepsSurvivors(test *testing.T) {
    model, _, _ := makeBattleTestModel()
    encounter := &maplib.ExtraEncounter{
        Type: maplib.EncounterTypeLair,
        Units: []units.Unit{units.HighMenSpearmen, units.GreatDrake},
    }

    defender, enemies, zone := model.EncounterDefenders(encounter, data.PlaneArcanus, 5, 5)
    if defender == nil || len(enemies) != 2 || zone.Encounter != combat.ZoneLair {
        test.Fatalf("lair defenders not set up: enemies=%v zone=%v", len(enemies), zone.Encounter)
    }

    enemies[0].AdjustHealth(-100)

    if model.ResolveEncounter(encounter, model.ArcanusMap, 5, 5, enemies, combat.CombatStateAttackerFlee) {
        test.Errorf("a lost battle should not clear the encounter")
    }
    if len(encounter.Units) != 1 || encounter.Units[0].Name != units.GreatDrake.Name {
        test.Errorf("only the surviving drake should guard the lair, got %v", encounter.Units)
    }
}

// an AI player takes gold, mana and retorts from a treasure without any screen
func TestApplyTreasureAi(test *testing.T) {
    model, _, ai := makeBattleTestModel()
    gold := ai.Gold
    mana := ai.Mana

    treasure := Treasure{
        Treasures: []TreasureItem{
            &TreasureGold{Amount: 100},
            &TreasureMana{Amount: 50},
            &TreasureRetort{Retort: data.RetortAlchemy},
        },
    }

    model.ApplyTreasure(nil, ai, treasure, spellbook.Spells{})

    if ai.Gold != gold + 100 || ai.Mana != mana + 50 {
        test.Errorf("gold/mana not applied: gold %v -> %v, mana %v -> %v", gold, ai.Gold, mana, ai.Mana)
    }
    if !ai.Wizard.RetortEnabled(data.RetortAlchemy) {
        test.Errorf("retort should be enabled")
    }
}
