package game

import (
    "errors"
    "testing"

    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/combat"
    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/lib/fraction"
)

// the battle test model, with building infos and a city for the first player at 5,5
func makeCommandTestModel() (*GameModel, *playerlib.Player, *citylib.City) {
    model, player, _ := makeBattleTestModel()

    infos := make(buildinglib.BuildingInfos, 40)
    for building := buildinglib.BuildingBarracks; building < buildinglib.BuildingLast; building++ {
        infos[infos.GetBuildingIndex(building)].Name = "building " + string(rune('A' + int(building)))
    }
    barracks := infos.GetBuildingIndex(buildinglib.BuildingBarracks)
    infos[barracks].Name = "Barracks"
    infos[barracks].ConstructionCost = 30
    infos[infos.GetBuildingIndex(buildinglib.BuildingHousing)].Name = "Housing"
    model.BuildingInfo = infos

    city := citylib.MakeCity("Testville", 5, 5, data.RaceHighMen, infos, model.ArcanusMap, model, player)
    city.Plane = data.PlaneArcanus
    city.Population = 4000
    city.Farmers = 4
    city.ResetCitizens()
    player.AddCity(city)

    model.CurrentPlayer = 0
    return model, player, city
}

func TestCommandJSONRoundTrip(test *testing.T) {
    commands := []Command{
        &MoveStackCommand{From: Location{X: 1, Y: 2}, To: Location{X: 3, Y: 4}},
        &SetResearchCommand{Spell: "Fireball"},
        &SetTaxRateCommand{Numerator: 3, Denominator: 2},
        &SetCityProductionCommand{City: Location{X: 5, Y: 5}, Unit: "Spearmen"},
        &SetCityFarmersCommand{City: Location{X: 5, Y: 5}, Farmers: 2},
        &BuyProductionCommand{City: Location{X: 5, Y: 5, Plane: data.PlaneMyrror}},
        &SellBuildingCommand{City: Location{X: 5, Y: 5}, Building: "Barracks"},
        &BuildActionCommand{Stack: Location{X: 1, Y: 1}},
        &BuildRoadCommand{Stack: Location{X: 1, Y: 1}, To: Location{X: 4, Y: 1}},
        &EndTurnCommand{},
    }

    for _, command := range commands {
        encoded, err := EncodeCommand(command)
        if err != nil {
            test.Fatalf("encode %v: %v", command.CommandType(), err)
        }
        decoded, err := DecodeCommand(encoded)
        if err != nil {
            test.Fatalf("decode %s: %v", encoded, err)
        }
        reencoded, _ := EncodeCommand(decoded)
        if string(reencoded) != string(encoded) {
            test.Errorf("round trip changed %s into %s", encoded, reencoded)
        }
    }

    if _, err := DecodeCommand([]byte(`{"type": "launch-nukes"}`)); err == nil {
        test.Errorf("unknown command types should be rejected")
    }
}

func TestCommandNotYourTurn(test *testing.T) {
    model, _, _ := makeCommandTestModel()
    other := model.Players[1]

    err := model.Execute(&CommandContext{Player: other}, &SetTaxRateCommand{Numerator: 1, Denominator: 1})
    if !errors.Is(err, ErrNotYourTurn) {
        test.Errorf("expected not your turn, got %v", err)
    }
}

func TestSetTaxRateCommand(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    context := &CommandContext{Player: player}

    if err := model.Execute(context, &SetTaxRateCommand{Numerator: 5, Denominator: 2}); err != nil {
        test.Fatalf("5/2 should be allowed: %v", err)
    }
    if !player.TaxRate.Equals(fraction.Make(5, 2)) {
        test.Errorf("tax rate not set: %v", player.TaxRate)
    }
    if err := model.Execute(context, &SetTaxRateCommand{Numerator: 7, Denominator: 3}); err == nil {
        test.Errorf("7/3 should be rejected")
    }
}

func TestCityCommands(test *testing.T) {
    model, player, city := makeCommandTestModel()
    context := &CommandContext{Player: player}
    here := Location{Plane: data.PlaneArcanus, X: 5, Y: 5}

    if err := model.Execute(context, &SetCityFarmersCommand{City: here, Farmers: city.Citizens() + 1}); err == nil {
        test.Errorf("more farmers than citizens should be rejected")
    }
    if err := model.Execute(context, &SetCityFarmersCommand{City: here, Farmers: city.Citizens() - city.Rebels}); err != nil {
        test.Errorf("all citizens farming should be allowed: %v", err)
    }

    if err := model.Execute(context, &SetCityProductionCommand{City: here, Building: "Housing"}); err != nil || city.ProducingBuilding != buildinglib.BuildingHousing {
        test.Errorf("housing should be allowed: %v", err)
    }
    if err := model.Execute(context, &SetCityProductionCommand{City: here, Building: "Death Star"}); err == nil {
        test.Errorf("unknown buildings should be rejected")
    }
    if err := model.Execute(context, &SetCityProductionCommand{City: Location{X: 9, Y: 9}, Building: "Housing"}); err == nil {
        test.Errorf("a city that isn't yours should be rejected")
    }

    city.Buildings.Insert(buildinglib.BuildingBarracks)
    gold := player.Gold
    if err := model.Execute(context, &SellBuildingCommand{City: here, Building: "Barracks"}); err != nil {
        test.Fatalf("selling barracks: %v", err)
    }
    if player.Gold != gold + 10 || city.Buildings.Contains(buildinglib.BuildingBarracks) {
        test.Errorf("barracks should be sold for 10 gold")
    }
    city.Buildings.Insert(buildinglib.BuildingBarracks)
    if err := model.Execute(context, &SellBuildingCommand{City: here, Building: "Barracks"}); err == nil {
        test.Errorf("a second sale in one turn should be rejected")
    }

    city.ProducingBuilding = buildinglib.BuildingBarracks
    city.Production = 0
    player.Gold = 50
    if err := model.Execute(context, &BuyProductionCommand{City: here}); err == nil {
        test.Errorf("buying for 120 gold with 50 should be rejected")
    }
    player.Gold = 500
    if err := model.Execute(context, &BuyProductionCommand{City: here}); err != nil || player.Gold != 380 {
        test.Errorf("buying barracks should cost 120 gold: %v, gold %v", err, player.Gold)
    }
}

func TestMoveStackCommand(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    player.LiftFogSquare(3, 3, 5, data.PlaneArcanus)
    handlers := &fakeMoveHandlers{battleResult: combat.CombatStateAttackerFlee}
    context := &CommandContext{Player: player, Handlers: handlers}

    // the drakes at 3,3 attack the spearman at 4,3
    err := model.Execute(context, &MoveStackCommand{From: Location{Plane: data.PlaneArcanus, X: 3, Y: 3}, To: Location{Plane: data.PlaneArcanus, X: 4, Y: 3}})
    if err != nil {
        test.Fatalf("move: %v", err)
    }
    if handlers.battles != 1 {
        test.Errorf("moving onto the enemy should start a battle, got %v", handlers.battles)
    }

    if err := model.Execute(context, &MoveStackCommand{From: Location{X: 8, Y: 8}, To: Location{X: 9, Y: 9}}); err == nil {
        test.Errorf("moving a stack that isn't there should be rejected")
    }
}

func TestSetResearchCommand(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    context := &CommandContext{Player: player}
    player.ResearchCandidateSpells.AddSpell(spellbook.Spell{Name: "Fireball", ResearchCost: 100})

    if err := model.Execute(context, &SetResearchCommand{Spell: "Fireball"}); err != nil || player.ResearchingSpell.Name != "Fireball" {
        test.Errorf("Fireball should be researchable: %v", err)
    }
    if err := model.Execute(context, &SetResearchCommand{Spell: "Spell of Mastery"}); err == nil {
        test.Errorf("a spell that isn't a candidate should be rejected")
    }
}
