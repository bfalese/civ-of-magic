package game

import (
    "image"
    "testing"

    "github.com/bfalese/civ-of-magic/core/building"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/maplib"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/lib/set"
)

func TestApplySpellOfReturn(test *testing.T) {
    human, _ := makeContactPlayers()
    human.Banished = true
    city := &citylib.City{Buildings: set.MakeSet[building.Building](), Race: data.RaceDwarf}

    model := &GameModel{}
    model.ApplySpellOfReturn(human, city)

    if human.Banished {
        test.Errorf("wizard should no longer be banished")
    }
    if !city.Buildings.Contains(building.BuildingFortress) || !city.Buildings.Contains(building.BuildingSummoningCircle) {
        test.Errorf("city should get the fortress and summoning circle")
    }
    if human.Wizard.Race != data.RaceDwarf {
        test.Errorf("wizard race should become the city's race, got %v", human.Wizard.Race)
    }
}

func TestIsGlobalEnchantmentActive(test *testing.T) {
    human, enemy := makeContactPlayers()
    model := &GameModel{Players: []*playerlib.Player{human, enemy}}

    if model.IsGlobalEnchantmentActive(data.EnchantmentPlanarSeal) {
        test.Errorf("nobody has Planar Seal yet")
    }
    enemy.GlobalEnchantments.Insert(data.EnchantmentPlanarSeal)
    if !model.IsGlobalEnchantmentActive(data.EnchantmentPlanarSeal) {
        test.Errorf("enemy's Planar Seal should count")
    }
}

// Nature Awareness reveals both planes and introduces the wizards that become visible
func TestNatureAwarenessRevealsMap(test *testing.T) {
    human, enemy := makeContactPlayers()
    model := makeSpellTestModel(10, 10)
    model.MyrrorMap = model.ArcanusMap
    model.Players = []*playerlib.Player{human, enemy}
    model.Events = make(chan GameEvent, 10)

    if human.IsExplored(7, 7, data.PlaneMyrror) {
        test.Fatalf("map should start unexplored")
    }

    model.ApplyGlobalEnchantment(data.EnchantmentNatureAwareness, human)

    if !human.IsExplored(7, 7, data.PlaneMyrror) || !human.IsExplored(0, 0, data.PlaneArcanus) {
        test.Errorf("Nature Awareness should reveal both planes")
    }
}

// the stack's own plane counts, not the plane the frontend shows (model.Plane): a stack on a tower
// travels whichever plane is viewed, and the view says it can
func TestPlanarTravelUsesTheStacksPlane(test *testing.T) {
    model, player, _ := makeBattleTestModel()
    model.ArcanusMap.SetPlaneTower(3, 3)
    stack := player.FindStack(3, 3, data.PlaneArcanus)
    model.Plane = data.PlaneArcanus

    if !model.CanPlanarTravel(player, stack) {
        test.Fatalf("a stack on a tower should be able to travel")
    }
    view := model.MakePlayerView(player)
    if !view.Stacks[0].PlanarTravel {
        test.Errorf("the view should offer planar travel")
    }

    if err := model.PlanarTravel(player, stack); err != nil {
        test.Fatalf("planar travel from the viewed plane: %v", err)
    }
    if player.FindStack(3, 3, data.PlaneMyrror) == nil || player.FindStack(3, 3, data.PlaneArcanus) != nil {
        test.Errorf("the stack should be on Myrror now")
    }
    if player.SelectedStack == nil || player.SelectedStack.Plane() != data.PlaneMyrror {
        test.Errorf("the selected stack should be the one that arrived")
    }
    if model.Plane != data.PlaneArcanus {
        test.Errorf("travel shouldn't change the viewed plane")
    }

    // and back, through the command, still viewing Arcanus
    context := &CommandContext{Player: player}
    model.CurrentPlayer = 0
    if err := model.Execute(context, &PlanarTravelCommand{Stack: Location{Plane: data.PlaneMyrror, X: 3, Y: 3}}); err != nil {
        test.Fatalf("planar-travel back: %v", err)
    }
    if player.FindStack(3, 3, data.PlaneArcanus) == nil {
        test.Errorf("the stack should be back on Arcanus")
    }

    // no tower: nothing offered, nothing moves
    other := player.FindStack(3, 3, data.PlaneArcanus)
    delete(model.ArcanusMap.ExtraMap[image.Pt(3, 3)], maplib.ExtraKindOpenTower)
    if model.CanPlanarTravel(player, other) {
        test.Errorf("without a tower there is no way to travel")
    }
    if err := model.PlanarTravel(player, other); err == nil {
        test.Errorf("travel without a way should fail")
    }
}
