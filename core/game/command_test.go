package game

import (
    "errors"
    "testing"

    "github.com/bfalese/civ-of-magic/core/artifact"
    "github.com/bfalese/civ-of-magic/core/units"

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
        &CastSpellCommand{Spell: "Wall of Fire", Target: &CommandSpellTarget{Location: &Location{X: 5, Y: 5}}},
        &CastSpellCommand{Spell: "Spell Blast", Target: &CommandSpellTarget{Wizard: "green"}},
        &CastSpellTargetCommand{Target: CommandSpellTarget{Location: &Location{X: 3, Y: 3}, Choice: "Spell Ward"}},
        &AnswerOfferCommand{Accept: true},
        &PlanarTravelCommand{Stack: Location{X: 3, Y: 3}},
        &PatrolCommand{Stack: Location{X: 3, Y: 3}},
        &DisbandUnitCommand{Stack: Location{X: 3, Y: 3}, Unit: 1},
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

func TestCastSpellCommandNow(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    player.KnownSpells.AddSpell(spellbook.Spell{Name: "Bless", Magic: data.LifeMagic, CastCost: 10})
    player.Mana = 100
    player.RemainingCastingSkill = 100
    context := &CommandContext{Player: player}
    stackLocation := Location{Plane: data.PlaneArcanus, X: 3, Y: 3}

    if err := model.Execute(context, &CastSpellCommand{Spell: "Bless"}); !errors.Is(err, ErrSpellNeedsTarget) {
        test.Errorf("bless without a target: %v", err)
    }

    unit := 0
    if err := model.Execute(context, &CastSpellCommand{Spell: "Bless", Target: &CommandSpellTarget{Location: &stackLocation, Unit: &unit}}); err != nil {
        test.Fatalf("bless: %v", err)
    }
    if !player.FindStack(3, 3, data.PlaneArcanus).Units()[0].HasEnchantment(data.UnitEnchantmentBless) {
        test.Errorf("the first unit should be blessed")
    }
    if player.Mana >= 100 || player.RemainingCastingSkill >= 100 {
        test.Errorf("casting should cost mana and skill: mana=%v skill=%v", player.Mana, player.RemainingCastingSkill)
    }

    if err := model.Execute(context, &CastSpellCommand{Spell: "Armageddon"}); err == nil {
        test.Errorf("an unknown spell should be refused")
    }
}

func TestCastSpellCommandOverTurns(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    player.KnownSpells.AddSpell(spellbook.Spell{Name: "Bless", Magic: data.LifeMagic, CastCost: 10})
    player.Mana = 5
    player.RemainingCastingSkill = 5
    context := &CommandContext{Player: player}

    if err := model.Execute(context, &CastSpellCommand{Spell: "Bless"}); err != nil {
        test.Fatalf("starting a long cast: %v", err)
    }
    if player.CastingSpell.Name != "Bless" {
        test.Fatalf("bless should be the spell being cast, got %v", player.CastingSpell.Name)
    }

    // the casting finished (StartPlayerTurn calls the hook): the target is asked for
    model.FinishCast(player, player.CastingSpell)
    var chosen *GameEventChooseSpellTarget
    for len(model.Events) > 0 {
        if event, ok := (<-model.Events).(*GameEventChooseSpellTarget); ok {
            chosen = event
        }
    }
    if chosen == nil || chosen.Spell.Name != "Bless" || chosen.LocationType != LocationTypeFriendlyUnit {
        test.Fatalf("expected a choose-spell-target event, got %+v", chosen)
    }

    unit := 1
    target := CommandSpellTarget{Location: &Location{Plane: data.PlaneArcanus, X: 3, Y: 3}, Unit: &unit}
    if err := model.Execute(context, &CastSpellTargetCommand{Target: target}); err != nil {
        test.Fatalf("target: %v", err)
    }
    if !player.FindStack(3, 3, data.PlaneArcanus).Units()[1].HasEnchantment(data.UnitEnchantmentBless) {
        test.Errorf("the second unit should be blessed")
    }
    if err := model.Execute(context, &CastSpellTargetCommand{Target: target}); err == nil {
        test.Errorf("no spell should be pending any more")
    }
}

func TestAnswerOfferCommand(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    model.ArtifactPool = artifact.MakeCatalog(nil)
    context := &CommandContext{Player: player}

    if err := model.Execute(context, &AnswerOfferCommand{Accept: true}); err == nil {
        test.Errorf("answering without an offer should fail")
    }

    player.Gold = 300
    mercenary := units.MakeOverworldUnitFromUnit(units.HighMenSpearmen, 5, 5, data.PlaneArcanus, player.Wizard.Banner, player.MakeExperienceInfo(), player.MakeUnitEnchantmentProvider())
    model.SendOffer(&GameEventHireMercenaries{Units: []*units.OverworldUnit{mercenary}, Cost: 100, Player: player})
    if err := model.Execute(context, &AnswerOfferCommand{Accept: true}); err != nil {
        test.Fatalf("hiring mercenaries: %v", err)
    }
    if player.Gold != 200 || player.FindStackByUnit(mercenary) == nil {
        test.Errorf("the mercenary should be hired for 100 gold (gold %v)", player.Gold)
    }

    item := &artifact.Artifact{Name: "Sword", Cost: 500}
    model.SendOffer(&GameEventMerchant{Artifact: item, Cost: 500, Player: player})
    if err := model.Execute(context, &AnswerOfferCommand{Accept: true}); err == nil {
        test.Errorf("buying a 500 gold item with 200 gold should fail")
    }
    if err := model.Execute(context, &AnswerOfferCommand{Accept: false}); err != nil {
        test.Errorf("declining: %v", err)
    }
    if _, open := model.PendingOffers[player]; open {
        test.Errorf("the offer should be closed after declining")
    }
}

func TestUnitOrderCommands(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    context := &CommandContext{Player: player}
    here := Location{Plane: data.PlaneArcanus, X: 3, Y: 3}
    stack := player.FindStack(3, 3, data.PlaneArcanus)

    if err := model.Execute(context, &PatrolCommand{Stack: here}); err != nil {
        test.Fatalf("patrol: %v", err)
    }
    for _, unit := range stack.Units() {
        if unit.GetBusy() != units.BusyStatusPatrol {
            test.Errorf("%v should patrol", unit.GetName())
        }
    }

    before := len(stack.Units())
    if err := model.Execute(context, &DisbandUnitCommand{Stack: here, Unit: 0}); err != nil {
        test.Fatalf("disband: %v", err)
    }
    if len(stack.Units()) != before - 1 {
        test.Errorf("one unit should be gone: %v -> %v", before, len(stack.Units()))
    }
    if err := model.Execute(context, &DisbandUnitCommand{Stack: here, Unit: 99}); err == nil {
        test.Errorf("a unit index out of range should be refused")
    }

    // no tower, gate or plane shift here
    if err := model.Execute(context, &PlanarTravelCommand{Stack: here}); err == nil {
        test.Errorf("planar travel without a way to travel should fail")
    }
    if model.Plane != data.PlaneArcanus {
        test.Errorf("the viewed plane should be restored")
    }
}
