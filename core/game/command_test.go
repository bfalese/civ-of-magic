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
    "github.com/bfalese/civ-of-magic/lib/set"
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
        &SetPowerDistributionCommand{Mana: 0.5, Research: 0.25, Skill: 0.25},
        &AlchemyCommand{Direction: "gold-to-mana", Amount: 10},
        &SetUnitActiveCommand{Stack: Location{X: 3, Y: 3}, Unit: 0, Active: true},
        &RenameCityCommand{City: Location{X: 5, Y: 5}, Name: "Newtown"},
        &RenameHeroCommand{Hero: 1, Name: "Bob"},
        &MoveItemCommand{From: ItemSlot{Vault: new(int)}, To: ItemSlot{Hero: new(int), Slot: 2}},
        &DestroyItemCommand{Item: ItemSlot{New: new(int)}},
        &ProposeTreatyCommand{Wizard: "green", Treaty: "alliance"},
        &OfferTributeCommand{Wizard: "green", Gold: true},
        &TradeSpellsCommand{Wizard: "green", Spell: "Fireball"},
        &CastSpellCommand{Spell: "Create Artifact", Artifact: &artifact.Design{Type: "Staff", Powers: []string{"+1 Attack"}, Spell: "Fire Bolt", Charges: 2}},
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

func TestMagicCommands(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    context := &CommandContext{Player: player}

    if err := model.Execute(context, &SetPowerDistributionCommand{Mana: 0.5, Research: 0.3, Skill: 0.2}); err != nil {
        test.Fatalf("power distribution: %v", err)
    }
    if player.PowerDistribution.Research != 0.3 {
        test.Errorf("research share not set: %+v", player.PowerDistribution)
    }
    if err := model.Execute(context, &SetPowerDistributionCommand{Mana: 0.5, Research: 0.5, Skill: 0.5}); err == nil {
        test.Errorf("shares over 1 should be refused")
    }

    player.Gold = 100
    player.Mana = 0
    if err := model.Execute(context, &AlchemyCommand{Direction: "gold-to-mana", Amount: 40}); err != nil {
        test.Fatalf("alchemy: %v", err)
    }
    if player.Gold != 60 || player.Mana != 20 {
        test.Errorf("40 gold should become 20 mana: gold %v mana %v", player.Gold, player.Mana)
    }
    if err := model.Execute(context, &AlchemyCommand{Direction: "mana-to-gold", Amount: 500}); err == nil {
        test.Errorf("converting more mana than you have should fail")
    }
}

func TestSetUnitActiveAndSplitMove(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    player.LiftFogSquare(3, 3, 4, data.PlaneArcanus)
    here := Location{Plane: data.PlaneArcanus, X: 3, Y: 3}
    context := &CommandContext{Player: player, Handlers: &fakeMoveHandlers{}}
    stack := player.FindStack(3, 3, data.PlaneArcanus)
    for _, unit := range stack.Units() {
        stack.SetActive(unit, true)
    }

    // the first two drakes stay home
    for index := range 2 {
        if err := model.Execute(context, &SetUnitActiveCommand{Stack: here, Unit: index, Active: false}); err != nil {
            test.Fatalf("set inactive: %v", err)
        }
    }

    if err := model.Execute(context, &MoveStackCommand{From: here, To: Location{Plane: data.PlaneArcanus, X: 3, Y: 5}}); err != nil {
        test.Fatalf("move: %v", err)
    }

    home := player.FindStack(3, 3, data.PlaneArcanus)
    if home == nil || len(home.Units()) != 2 {
        test.Errorf("two inactive drakes should stay at 3,3")
    }
    if len(player.Stacks) != 2 {
        test.Errorf("the stack should have split in two, has %v stacks", len(player.Stacks))
    }
}

func TestRenameCommands(test *testing.T) {
    model, player, city := makeCommandTestModel()
    context := &CommandContext{Player: player}

    if err := model.Execute(context, &RenameCityCommand{City: Location{Plane: data.PlaneArcanus, X: 5, Y: 5}, Name: "Newtown"}); err != nil || city.Name != "Newtown" {
        test.Errorf("rename city: %v (%v)", err, city.Name)
    }
    if err := model.Execute(context, &RenameHeroCommand{Hero: 0, Name: "Bob"}); err == nil {
        test.Errorf("renaming a hero you don't have should fail")
    }
}

func TestItemCommands(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    context := &CommandContext{Player: player}
    sword := &artifact.Artifact{Name: "Sword", Type: artifact.ArtifactTypeSword, Cost: 200}

    model.SendVaultItem(&GameEventVault{CreatedArtifact: sword, Player: player})
    if len(model.PendingItems[player]) != 1 {
        test.Fatalf("the new item should be pending: %v", model.PendingItems[player])
    }

    zero := 0
    slot2 := 2
    if err := model.Execute(context, &MoveItemCommand{From: ItemSlot{New: &zero}, To: ItemSlot{Vault: &slot2}}); err != nil {
        test.Fatalf("place the new item: %v", err)
    }
    if player.VaultEquipment[2] != sword || len(model.PendingItems[player]) != 0 {
        test.Errorf("the sword should be in vault slot 2")
    }

    player.Mana = 0
    if err := model.Execute(context, &DestroyItemCommand{Item: ItemSlot{Vault: &slot2}}); err != nil {
        test.Fatalf("destroy: %v", err)
    }
    if player.VaultEquipment[2] != nil || player.Mana != 100 {
        test.Errorf("destroying a 200 cost item should give 100 mana, mana %v", player.Mana)
    }

    if err := model.Execute(context, &DestroyItemCommand{Item: ItemSlot{Vault: &slot2}}); err == nil {
        test.Errorf("destroying an empty slot should fail")
    }
}

func TestDiplomacyCommands(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    enemy := model.Players[1]
    context := &CommandContext{Player: player}
    banner := enemy.GetBanner().String()

    if err := model.Execute(context, &ProposeTreatyCommand{Wizard: banner, Treaty: "pact"}); err == nil {
        test.Errorf("a wizard you haven't met can't be talked to")
    }

    player.AwarePlayer(enemy)
    enemy.AwarePlayer(player)
    if err := model.Execute(context, &ProposeTreatyCommand{Wizard: banner, Treaty: "pact"}); err != nil {
        test.Fatalf("pact: %v", err)
    }
    relation, _ := player.GetDiplomaticRelation(enemy)
    if relation.Treaty != data.TreatyPact {
        test.Errorf("the treaty should be a pact, got %v", relation.Treaty)
    }

    player.Gold = 1000
    enemyGold := enemy.Gold
    if err := model.Execute(context, &OfferTributeCommand{Wizard: banner, Gold: true}); err != nil {
        test.Fatalf("tribute: %v", err)
    }
    if player.Gold != 900 || enemy.Gold != enemyGold + 100 {
        test.Errorf("a tribute should be a tenth of the treasury: %v %v", player.Gold, enemy.Gold)
    }

    fireball := spellbook.Spell{Name: "Fireball"}
    player.KnownSpells.AddSpell(fireball)
    if err := model.Execute(context, &OfferTributeCommand{Wizard: banner, Spell: "Fireball"}); err != nil || !enemy.KnownSpells.Contains(fireball) {
        test.Errorf("spell tribute: %v", err)
    }
}

func TestCreateArtifactCommand(test *testing.T) {
    model, player, _ := makeCommandTestModel()
    attack := artifact.Power{Type: artifact.PowerTypeAttack, Amount: 1, Name: "+1 Attack"}
    model.ArtifactPowers = func() (map[artifact.Power]int, map[artifact.Power]set.Set[artifact.ArtifactType], error) {
        return map[artifact.Power]int{attack: 20}, map[artifact.Power]set.Set[artifact.ArtifactType]{attack: *set.NewSet(artifact.ArtifactTypeSword)}, nil
    }
    player.KnownSpells.AddSpell(spellbook.Spell{Name: "Enchant Item", Magic: data.ArcaneMagic, CastCost: 0})
    player.Mana = 1000
    player.RemainingCastingSkill = 1000
    context := &CommandContext{Player: player}

    design := &artifact.Design{Type: "Sword", Powers: []string{"+1 Attack"}}
    if err := model.Execute(context, &CastSpellCommand{Spell: "Enchant Item", Artifact: design}); err != nil {
        test.Fatalf("enchant item: %v", err)
    }
    pending := model.PendingItems[player]
    if len(pending) != 1 || pending[0].Type != artifact.ArtifactTypeSword {
        test.Fatalf("the sword should arrive at the vault: %v", pending)
    }
    if player.Mana >= 1000 {
        test.Errorf("the item should cost mana")
    }

    if err := model.Execute(context, &CastSpellCommand{Spell: "Enchant Item"}); err == nil {
        test.Errorf("enchant item without a design should fail")
    }
}
