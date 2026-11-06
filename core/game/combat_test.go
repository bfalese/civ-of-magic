package game

import (
    "testing"

    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/combat"
    "github.com/bfalese/civ-of-magic/core/data"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/setup"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/core/units"
)

// two AI wizards, a strong stack next to a weak one
func makeBattleTestModel() (*GameModel, *playerlib.Player, *playerlib.Player) {
    names := make(map[herolib.HeroType]string)
    attacker := playerlib.MakePlayer(setup.WizardCustom{Name: "Jafar", Banner: data.BannerGreen}, false, 8, 8, names, nil)
    defender := playerlib.MakePlayer(setup.WizardCustom{Name: "Oberic", Banner: data.BannerBlue}, false, 8, 8, names, nil)

    model := makeSpellTestModel(10, 10)
    model.MyrrorMap = model.ArcanusMap
    model.Players = []*playerlib.Player{attacker, defender}
    model.Events = make(chan GameEvent, 100)
    attacker.GlobalEnchantmentsProvider = model
    defender.GlobalEnchantmentsProvider = model

    for range 4 {
        attacker.AddUnit(units.MakeOverworldUnitFromUnit(units.GreatDrake, 3, 3, data.PlaneArcanus, attacker.Wizard.Banner, attacker.MakeExperienceInfo(), attacker.MakeUnitEnchantmentProvider()))
    }
    defender.AddUnit(units.MakeOverworldUnitFromUnit(units.HighMenSpearmen, 4, 3, data.PlaneArcanus, defender.Wizard.Banner, defender.MakeExperienceInfo(), defender.MakeUnitEnchantmentProvider()))

    return model, attacker, defender
}

func runBattleHeadless(test *testing.T, strategic bool) {
    model, attacker, defender := makeBattleTestModel()
    attackerStack := attacker.FindStack(3, 3, data.PlaneArcanus)
    defenderStack := defender.FindStack(4, 3, data.PlaneArcanus)

    events := make(chan combat.CombatEvent, 1000)
    battle := model.MakeBattle(spellbook.Spells{}, attacker, attackerStack, defender, defenderStack, combat.ZoneType{}, events)
    if len(battle.Combat.AttackingArmy.GetUnits()) != 4 || len(battle.Combat.DefendingArmy.GetUnits()) != 1 {
        test.Fatalf("armies should hold the stacks' units")
    }

    result := battle.Run(strategic)
    if !result.AttackerWon() {
        test.Fatalf("four great drakes should beat one spearman, got state %v", result.State)
    }
    if result.DefeatedDefenders != 1 {
        test.Errorf("one defender should be defeated, got %v", result.DefeatedDefenders)
    }

    defeatCity := func(*playerlib.Player, *playerlib.UnitStack, *playerlib.Player, *citylib.City) (bool, int) {
        test.Errorf("no city was involved")
        return false, 0
    }
    model.ApplyBattleOutcome(battle, result, defeatCity)
    aftermath := model.ApplyBattleAftermath(battle, result)

    if len(defender.Stacks) != 0 {
        test.Errorf("the defeated stack should be removed, defender has %v stacks", len(defender.Stacks))
    }
    if attacker.FindStack(3, 3, data.PlaneArcanus) == nil {
        test.Errorf("the winning stack should still be on the map")
    }
    if aftermath.HumanHeroDied || aftermath.UndeadUnits != 0 {
        test.Errorf("unexpected aftermath %+v", aftermath)
    }
}

func TestBattleStrategicHeadless(test *testing.T) {
    runBattleHeadless(test, true)
}

func TestBattleTacticalHeadless(test *testing.T) {
    runBattleHeadless(test, false)
}
