package game

import (
    "testing"

    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
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

// the build action follows the stack: settle away from cities, purify corruption, road otherwise
func TestStackBuildAction(test *testing.T) {
    model, player, _ := makeBattleTestModel()
    add := func(unit units.Unit, x int, y int) *playerlib.UnitStack {
        player.AddUnit(units.MakeOverworldUnitFromUnit(unit, x, y, data.PlaneArcanus, player.Wizard.Banner, player.MakeExperienceInfo(), player.MakeUnitEnchantmentProvider()))
        return player.FindStack(x, y, data.PlaneArcanus)
    }

    if action := model.StackBuildAction(player.FindStack(3, 3, data.PlaneArcanus)); action != "" {
        test.Errorf("drakes have nothing to build, got %q", action)
    }

    settlers := add(units.HighMenSettlers, 7, 7)
    if action := model.StackBuildAction(settlers); action != "settle" {
        test.Errorf("settlers on open land should settle, got %q", action)
    }

    priests := add(units.HighMenPriest, 1, 8)
    if action := model.StackBuildAction(priests); action != "" {
        test.Errorf("priests on clean land have nothing to do, got %q", action)
    }
    model.ArcanusMap.SetCorruption(1, 8)
    if action := model.StackBuildAction(priests); action != "purify" {
        test.Errorf("priests on corruption should purify, got %q", action)
    }

    player.SelectedStack = priests
    if model.DoBuildAction(player, func(data.Race) string { return "x" }) {
        test.Errorf("purifying isn't a road")
    }
    for _, unit := range priests.Units() {
        if unit.GetBusy() != units.BusyStatusPurify {
            test.Errorf("the priests should be purifying, got %v", unit.GetBusy())
        }
    }

    view := model.MakePlayerView(player)
    found := false
    for _, stack := range view.Stacks {
        if stack.Location.X == 7 && stack.Location.Y == 7 {
            found = true
            if stack.Build != "settle" {
                test.Errorf("the view should offer settling, got %q", stack.Build)
            }
        }
        if stack.Location.X == 1 && stack.Location.Y == 8 && stack.Units[0].Busy != "purifying land" {
            test.Errorf("the view should show the priests purifying, got %q", stack.Units[0].Busy)
        }
    }
    if !found {
        test.Errorf("the settlers' stack should be in the view")
    }

    add(units.HighMenEngineer, 2, 1)
    // the test player's fog is 8x8; the map is 10x10
    player.ArcanusFog = make(data.FogMap, 10)
    for x := range player.ArcanusFog {
        player.ArcanusFog[x] = make([]data.FogType, 10)
    }
    player.ExploreFogSquare(2, 2, 2, data.PlaneArcanus)
    view = model.MakePlayerView(player)
    for _, stack := range view.Stacks {
        if stack.Location.X == 2 && stack.Location.Y == 1 && (stack.Build != "road" || !stack.Road) {
            test.Errorf("engineers should offer a road: build %q road %v", stack.Build, stack.Road)
        }
    }
    path, err := model.RoadPathFor(player, Location{Plane: data.PlaneArcanus, X: 2, Y: 1}, Location{Plane: data.PlaneArcanus, X: 2, Y: 4})
    if err != nil || len(path) == 0 || path[len(path) - 1] != (Location{Plane: data.PlaneArcanus, X: 2, Y: 4}) {
        test.Errorf("the road path should end at the destination: %v %v", path, err)
    }
    if _, err := model.RoadPathFor(player, Location{Plane: data.PlaneArcanus, X: 7, Y: 7}, Location{Plane: data.PlaneArcanus, X: 2, Y: 4}); err == nil {
        test.Errorf("settlers can't build roads")
    }
}
