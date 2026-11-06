package game

import (
    "encoding/json"
    "testing"

    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    "github.com/bfalese/civ-of-magic/core/data"
)

func TestPlayerView(test *testing.T) {
    model, player, city := makeCommandTestModel()
    enemy := model.Players[1]
    city.Buildings.Insert(buildinglib.BuildingBarracks)
    city.ProducingBuilding = buildinglib.BuildingHousing

    // the enemy spearman at 4,3 is visible, the enemy far away is not
    player.LiftFogSquare(3, 3, 3, data.PlaneArcanus)

    view := model.MakePlayerView(player)
    if view.You.Player.Banner != player.GetBanner().String() || view.CurrentPlayer == nil {
        test.Errorf("wizard view: %+v", view.You)
    }
    if len(view.Cities) != 1 || view.Cities[0].Name != "Testville" || view.Cities[0].Producing != "Housing" {
        test.Fatalf("cities: %+v", view.Cities)
    }
    if len(view.Cities[0].Buildings) != 1 || view.Cities[0].Buildings[0] != "Barracks" {
        test.Errorf("buildings: %v", view.Cities[0].Buildings)
    }
    if len(view.Stacks) != 1 || len(view.Stacks[0].Units) != 4 {
        test.Errorf("own stacks: %+v", view.Stacks)
    }
    if len(view.ForeignStacks) != 1 || view.ForeignStacks[0].Owner.Banner != enemy.GetBanner().String() {
        test.Errorf("the visible enemy stack should be listed: %+v", view.ForeignStacks)
    }

    encoded, err := model.EncodePlayerView(player)
    if err != nil || !json.Valid(encoded) {
        test.Errorf("player view JSON: %v", err)
    }
}

func TestMapView(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    player.LiftFogSquare(3, 3, 1, data.PlaneArcanus)

    view := model.MakeMapView(player, data.PlaneArcanus)
    if view.Width != 10 || view.Height != 10 || len(view.Tiles) != 10 {
        test.Fatalf("size: %vx%v rows=%v", view.Width, view.Height, len(view.Tiles))
    }
    if view.Tiles[3][3] == nil || view.Tiles[3][3].Terrain == "" || !view.Tiles[3][3].Visible {
        test.Errorf("explored tile: %+v", view.Tiles[3][3])
    }
    if view.Tiles[9][9] != nil {
        test.Errorf("unexplored tiles should be empty: %+v", view.Tiles[9][9])
    }

    encoded, err := model.EncodeMapView(player, data.PlaneArcanus)
    if err != nil || !json.Valid(encoded) {
        test.Errorf("map view JSON: %v", err)
    }
}
