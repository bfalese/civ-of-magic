package game

import (
    "encoding/json"
    "slices"
    "strings"
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
    if !strings.HasPrefix(view.Cities[0].Asset, "city/") {
        test.Errorf("city asset: %q", view.Cities[0].Asset)
    }
    if len(view.Stacks) == 1 {
        stack := view.Stacks[0]
        if stack.Background != "unit-background/" + player.GetBanner().String() || !strings.HasPrefix(stack.Units[0].Asset, "unit/") || !strings.HasSuffix(stack.Units[0].Asset, "/" + player.GetBanner().String()) {
            test.Errorf("stack assets: %q %q", stack.Background, stack.Units[0].Asset)
        }
    }
    if len(view.ForeignStacks) == 1 && !strings.HasSuffix(view.ForeignStacks[0].Asset, "/" + enemy.GetBanner().String()) {
        test.Errorf("foreign stack asset: %q", view.ForeignStacks[0].Asset)
    }

    encoded, err := model.EncodePlayerView(player)
    if err != nil || !json.Valid(encoded) {
        test.Errorf("player view JSON: %v", err)
    }
}

func TestMapView(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    player.LiftFogSquare(3, 3, 1, data.PlaneArcanus)
    model.ArcanusMap.SetRoad(2, 2, false)
    model.ArcanusMap.SetRoad(3, 2, false)
    model.ArcanusMap.SetBonus(3, 2, data.BonusGoldOre)
    model.ArcanusMap.SetCorruption(3, 2)

    view := model.MakeMapView(player, data.PlaneArcanus)
    if view.Width != 10 || view.Height != 10 || len(view.Tiles) != 10 {
        test.Fatalf("size: %vx%v rows=%v", view.Width, view.Height, len(view.Tiles))
    }
    if view.Tiles[3][3] == nil || view.Tiles[3][3].Terrain == "" || !view.Tiles[3][3].Visible {
        test.Errorf("explored tile: %+v", view.Tiles[3][3])
    }
    if !strings.HasPrefix(view.Tiles[3][3].Asset, "terrain/") {
        test.Errorf("terrain asset: %q", view.Tiles[3][3].Asset)
    }
    // corruption hides the bonus; the road connects west
    overlays := view.Tiles[2][3].Overlays
    if !slices.Equal(overlays, []string{"road/w", "corruption"}) {
        test.Errorf("overlays at 3,2: %v", overlays)
    }
    if view.Tiles[2][2] == nil || !slices.Contains(view.Tiles[2][2].Overlays, "road/e") {
        test.Errorf("overlays at 2,2: %+v", view.Tiles[2][2])
    }
    if view.Tiles[9][9] != nil {
        test.Errorf("unexplored tiles should be empty: %+v", view.Tiles[9][9])
    }

    encoded, err := model.EncodeMapView(player, data.PlaneArcanus)
    if err != nil || !json.Valid(encoded) {
        test.Errorf("map view JSON: %v", err)
    }
}
