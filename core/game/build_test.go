package game

import (
    "testing"

    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/units"
)

// engineers have nothing to settle, meld or purify: the frontend should ask where to build a road
func TestDoBuildActionRoad(test *testing.T) {
    model, player, _ := makeBattleTestModel()
    player.AddUnit(units.MakeOverworldUnitFromUnit(units.HighMenEngineer, 6, 6, data.PlaneArcanus, player.Wizard.Banner, player.MakeExperienceInfo(), player.MakeUnitEnchantmentProvider()))
    player.SelectedStack = player.FindStack(6, 6, data.PlaneArcanus)

    names := func(race data.Race) string {
        return "Test Town"
    }

    if !model.DoBuildAction(player, names) {
        test.Errorf("engineers should ask for a road")
    }

    player.SelectedStack = nil
    if model.DoBuildAction(player, names) {
        test.Errorf("no selected stack should do nothing")
    }
}
