package game

import (
    "encoding/json"
    "errors"
    "fmt"

    "github.com/bfalese/civ-of-magic/core/artifact"
    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/units"
    "github.com/bfalese/civ-of-magic/lib/fraction"
)

// Commands are the actions a player takes, as data: a frontend (or a network peer, or the wasm
// bridge) sends a command, the model checks it and applies it, and the results come out as
// GameEvents on model.Events. Each command validates everything it relies on (ownership, gold,
// legal targets), since it may come from an untrusted client.
//
// Things are referred to by map location (plane, x, y): a city's location is unique, and a
// player has one stack per tile.
//
// JSON form: {"type": "move-stack", "command": {...}} (EncodeCommand / DecodeCommand).
type Command interface {
    // the name used in JSON
    CommandType() string
    Execute(context *CommandContext) error
}

// what a command needs besides the model; the frontend fills it in
type CommandContext struct {
    Model *GameModel
    Player *playerlib.Player
    // screens for battles, encounters and dialogs while a stack moves
    Handlers PlayerMovementHandler
    // names new outposts (city names may come from LBX)
    SuggestCityName func(race data.Race) string
    // ends the player's turn; the frontend owns the turn loop
    EndTurn func()
}

var ErrNotYourTurn = errors.New("not this player's turn")

// check and apply a command for player
func (model *GameModel) Execute(context *CommandContext, command Command) error {
    if context.Model == nil {
        context.Model = model
    }
    if context.Player == nil {
        return errors.New("no player")
    }
    if model.CurrentPlayer < 0 || model.CurrentPlayer >= len(model.Players) || model.Players[model.CurrentPlayer] != context.Player {
        return ErrNotYourTurn
    }
    return command.Execute(context)
}

type Location struct {
    Plane data.Plane `json:"plane"`
    X int `json:"x"`
    Y int `json:"y"`
}

func (context *CommandContext) ownStack(location Location) (*playerlib.UnitStack, error) {
    stack := context.Player.FindStack(location.X, location.Y, location.Plane)
    if stack == nil {
        return nil, fmt.Errorf("no stack of yours at %v,%v", location.X, location.Y)
    }
    return stack, nil
}

func (context *CommandContext) ownCity(location Location) (*citylib.City, error) {
    city := context.Player.FindCity(location.X, location.Y, location.Plane)
    if city == nil {
        return nil, fmt.Errorf("no city of yours at %v,%v", location.X, location.Y)
    }
    return city, nil
}

// the building with this name (as in the building infos), BuildingNone if unknown
func (model *GameModel) buildingByName(name string) buildinglib.Building {
    for building := buildinglib.BuildingBarracks; building < buildinglib.BuildingLast; building++ {
        if model.BuildingInfo.Name(building) == name {
            return building
        }
    }
    return buildinglib.BuildingNone
}

// move a stack to a destination, as the player does on the map (see GameModel.OrderStackMove):
// only the active units go, Earth Gate moves are instant, otherwise the units walk the path as far
// as their moves go (the rest is kept for next turns). Battles and encounters on the way go
// through context.Handlers.
type MoveStackCommand struct {
    From Location `json:"from"`
    To Location `json:"to"`
}

func (command *MoveStackCommand) CommandType() string { return "move-stack" }

func (command *MoveStackCommand) Execute(context *CommandContext) error {
    model := context.Model
    player := context.Player
    stack, err := context.ownStack(command.From)
    if err != nil {
        return err
    }
    if command.To.Plane != command.From.Plane {
        return errors.New("use planar travel to change plane")
    }
    if context.Handlers == nil {
        return errors.New("moving needs movement handlers")
    }

    walking, err := model.OrderStackMove(player, stack, command.To.X, command.To.Y)
    if err != nil {
        return err
    }
    if walking != nil {
        player.SelectedStack = walking
        model.MoveSelectedStack(context.Handlers, player)
    }
    return nil
}

type SetResearchCommand struct {
    Spell string `json:"spell"`
}

func (command *SetResearchCommand) CommandType() string { return "set-research" }

func (command *SetResearchCommand) Execute(context *CommandContext) error {
    player := context.Player
    spell := player.ResearchCandidateSpells.FindByName(command.Spell)
    if !spell.Valid() {
        return fmt.Errorf("%v can't be researched now", command.Spell)
    }
    player.ResearchingSpell = spell
    return nil
}

// tax rate in gold per citizen: 0, 1/2, 1, 3/2, 2, 5/2 or 3
type SetTaxRateCommand struct {
    Numerator int `json:"numerator"`
    Denominator int `json:"denominator"`
}

func (command *SetTaxRateCommand) CommandType() string { return "set-tax-rate" }

func (command *SetTaxRateCommand) Execute(context *CommandContext) error {
    if command.Denominator <= 0 {
        return errors.New("bad tax rate")
    }
    rate := fraction.Make(command.Numerator, command.Denominator)

    // walk the allowed rates from zero
    allowed := fraction.Zero()
    for {
        if allowed.Equals(rate) {
            context.Player.UpdateTaxRate(rate)
            return nil
        }
        next, ok := context.Player.NextTaxRate(allowed)
        if !ok {
            return fmt.Errorf("tax rate %v/%v not allowed", command.Numerator, command.Denominator)
        }
        allowed = next
    }
}

// what a city produces: a building (by name) or a unit (by name); exactly one of them
type SetCityProductionCommand struct {
    City Location `json:"city"`
    Building string `json:"building,omitempty"`
    Unit string `json:"unit,omitempty"`
}

func (command *SetCityProductionCommand) CommandType() string { return "set-city-production" }

func (command *SetCityProductionCommand) Execute(context *CommandContext) error {
    city, err := context.ownCity(command.City)
    if err != nil {
        return err
    }

    if (command.Building == "") == (command.Unit == "") {
        return errors.New("give either a building or a unit")
    }

    if command.Building != "" {
        building := context.Model.buildingByName(command.Building)
        if building == buildinglib.BuildingNone {
            return fmt.Errorf("unknown building %v", command.Building)
        }
        // what the build screen offers: allowed here, not built yet, prerequisites met
        if !city.ComputePossibleBuildings(false).Contains(building) {
            return fmt.Errorf("%v can't build %v", city.Name, command.Building)
        }
        city.ProducingBuilding = building
        city.ProducingUnit = units.UnitNone
        return nil
    }

    for _, unit := range city.ComputePossibleUnits() {
        if unit.Name == command.Unit {
            city.ProducingBuilding = buildinglib.BuildingNone
            city.ProducingUnit = unit
            return nil
        }
    }
    return fmt.Errorf("%v can't build %v", city.Name, command.Unit)
}

// set how many citizens farm; the others (except rebels) work. At least the subsistence farmers.
type SetCityFarmersCommand struct {
    City Location `json:"city"`
    Farmers int `json:"farmers"`
}

func (command *SetCityFarmersCommand) CommandType() string { return "set-city-farmers" }

func (command *SetCityFarmersCommand) Execute(context *CommandContext) error {
    city, err := context.ownCity(command.City)
    if err != nil {
        return err
    }

    available := city.Citizens() - city.Rebels
    minimum := min(city.ComputeSubsistenceFarmers(), available)
    if command.Farmers < minimum || command.Farmers > available {
        return fmt.Errorf("farmers must be between %v and %v", minimum, available)
    }

    city.SetFarmers(command.Farmers)
    return nil
}

type BuyProductionCommand struct {
    City Location `json:"city"`
}

func (command *BuyProductionCommand) CommandType() string { return "buy-production" }

func (command *BuyProductionCommand) Execute(context *CommandContext) error {
    city, err := context.ownCity(command.City)
    if err != nil {
        return err
    }
    if !context.Player.BuyProduction(city) {
        return errors.New("nothing to buy or not enough gold")
    }
    return nil
}

// sell a building back for gold, one per city per turn
type SellBuildingCommand struct {
    City Location `json:"city"`
    Building string `json:"building"`
}

func (command *SellBuildingCommand) CommandType() string { return "sell-building" }

func (command *SellBuildingCommand) Execute(context *CommandContext) error {
    city, err := context.ownCity(command.City)
    if err != nil {
        return err
    }
    if city.SoldBuilding {
        return errors.New("you can only sell back one building per turn")
    }
    building := context.Model.buildingByName(command.Building)
    if building == buildinglib.BuildingNone || !city.Buildings.Contains(building) || !city.CanSellBuilding(building) {
        return fmt.Errorf("%v can't sell %v", city.Name, command.Building)
    }

    context.Player.SellBuilding(city, building)
    return nil
}

// the build button for a stack: settle an outpost, meld a node or purify. Stacks that can only
// build roads need BuildRoadCommand.
type BuildActionCommand struct {
    Stack Location `json:"stack"`
}

func (command *BuildActionCommand) CommandType() string { return "build-action" }

func (command *BuildActionCommand) Execute(context *CommandContext) error {
    stack, err := context.ownStack(command.Stack)
    if err != nil {
        return err
    }
    if context.SuggestCityName == nil {
        return errors.New("settling needs a city name source")
    }

    context.Player.SelectedStack = stack
    if context.Model.DoBuildAction(context.Player, context.SuggestCityName) {
        return errors.New("this stack can only build roads: use build-road")
    }
    return nil
}

// engineers in the stack build a road to the destination, over the next turns
type BuildRoadCommand struct {
    Stack Location `json:"stack"`
    To Location `json:"to"`
}

func (command *BuildRoadCommand) CommandType() string { return "build-road" }

func (command *BuildRoadCommand) Execute(context *CommandContext) error {
    model := context.Model
    player := context.Player
    stack, err := context.ownStack(command.Stack)
    if err != nil {
        return err
    }
    if !stack.ActiveUnitsHasAbility(data.AbilityConstruction) {
        return errors.New("no engineers in the stack")
    }

    path := model.FindRoadPath(stack.X(), stack.Y(), command.To.X, command.To.Y, player, stack, player.GetFog(stack.Plane()))
    if len(path) == 0 {
        return fmt.Errorf("no road path to %v,%v", command.To.X, command.To.Y)
    }

    for _, unit := range stack.Units() {
        if unit.HasAbility(data.AbilityConstruction) {
            unit.SetBuildRoadPath(path)
        }
    }
    model.MaybeBuildRoads(stack, player)
    return nil
}

type EndTurnCommand struct {
}

func (command *EndTurnCommand) CommandType() string { return "end-turn" }

func (command *EndTurnCommand) Execute(context *CommandContext) error {
    if context.EndTurn == nil {
        return errors.New("the frontend doesn't end turns through commands")
    }
    context.EndTurn()
    return nil
}


// what a spell is cast on: a tile (city, stack or map tile), and for unit spells the unit (its id,
// see GameModel.UnitId; it must stand on that tile); or a wizard (by banner)
// for spells on wizards; plus a choice by name where the spell needs one (Spell Ward's ward, the
// enchantment for Spell Binding / Disjunction, the hero for Resurrection)
type CommandSpellTarget struct {
    Location *Location `json:"location,omitempty"`
    Unit *int `json:"unit,omitempty"`
    Wizard string `json:"wizard,omitempty"`
    Choice string `json:"choice,omitempty"`
}

func (context *CommandContext) spellTarget(target *CommandSpellTarget) (SpellTarget, error) {
    if target == nil {
        return SpellTarget{}, nil
    }

    out := SpellTarget{Choice: target.Choice}

    if target.Location != nil {
        location := *target.Location
        out.Location = &location
    }

    if target.Unit != nil {
        unit, stack, err := context.Model.FindUnitById(context.Player, *target.Unit)
        if err != nil {
            return SpellTarget{}, err
        }
        here := Location{Plane: stack.Plane(), X: stack.X(), Y: stack.Y()}
        if out.Location != nil && *out.Location != here {
            return SpellTarget{}, fmt.Errorf("unit %v isn't at %v,%v", *target.Unit, out.Location.X, out.Location.Y)
        }
        out.Location = &here
        out.Unit = unit
    }

    if target.Wizard != "" {
        wizard, err := context.wizardByBanner(target.Wizard)
        if err != nil {
            return SpellTarget{}, err
        }
        out.Wizard = wizard
    }

    return out, nil
}

// cast an overland spell: right away if mana and the remaining casting skill allow it (a spell
// that needs a target must then come with one), otherwise it becomes the spell being cast over
// the next turns (when it finishes, a choose-spell-target event asks for the target)
type CastSpellCommand struct {
    Spell string `json:"spell"`
    Target *CommandSpellTarget `json:"target,omitempty"`
    // the item to make, for Create Artifact and Enchant Item
    Artifact *artifact.Design `json:"artifact,omitempty"`
}

func (command *CastSpellCommand) CommandType() string { return "cast-spell" }

func (command *CastSpellCommand) Execute(context *CommandContext) error {
    model := context.Model
    player := context.Player

    overlandSpells := player.KnownSpells.OverlandSpells()
    spell := overlandSpells.FindByName(command.Spell)
    if !spell.Valid() || spell.Name == "Spell of Return" {
        return fmt.Errorf("%v can't be cast now", command.Spell)
    }

    target, err := context.spellTarget(command.Target)
    if err != nil {
        return err
    }

    if spell.Name == "Create Artifact" || spell.Name == "Enchant Item" {
        if command.Artifact == nil {
            return errors.New("give the item design")
        }
        if model.ArtifactPowers == nil {
            return errors.New("the artifact power table isn't available")
        }
        costs, compatibilities, err := model.ArtifactPowers()
        if err != nil {
            return err
        }
        created, err := artifact.BuildDesign(*command.Artifact, spell.Name == "Enchant Item", &player.Wizard, player.KnownSpells.CombatSpells(false), costs, compatibilities)
        if err != nil {
            return err
        }
        player.CreateArtifact = created
        spell.OverrideCost = created.Cost
    }

    cost := player.ComputeEffectiveSpellCost(spell, true)
    if cost <= player.Mana && cost <= player.RemainingCastingSkill {
        if _, needsTarget := model.SpellTargetType(player, spell); needsTarget && command.Target == nil {
            return ErrSpellNeedsTarget
        }
        if err := model.CastSpell(player, spell, target); err != nil {
            return err
        }
        player.Mana -= cost
        player.RemainingCastingSkill -= cost
        return nil
    }

    player.CastingSpell = spell
    player.CastingSpellProgress = 0
    return nil
}

// the target for the player's finished spell announced by choose-spell-target
type CastSpellTargetCommand struct {
    Target CommandSpellTarget `json:"target"`
}

func (command *CastSpellTargetCommand) CommandType() string { return "cast-spell-target" }

func (command *CastSpellTargetCommand) Execute(context *CommandContext) error {
    model := context.Model
    spell, ok := model.PendingSpells[context.Player]
    if !ok {
        return errors.New("no spell is waiting for a target")
    }

    target, err := context.spellTarget(&command.Target)
    if err != nil {
        return err
    }
    if err := model.CastSpell(context.Player, spell, target); err != nil {
        return err
    }
    delete(model.PendingSpells, context.Player)
    return nil
}

// accept or decline the open hero / mercenaries / merchant offer (hire-hero, hire-mercenaries,
// merchant events)
type AnswerOfferCommand struct {
    Accept bool `json:"accept"`
}

func (command *AnswerOfferCommand) CommandType() string { return "answer-offer" }

func (command *AnswerOfferCommand) Execute(context *CommandContext) error {
    return context.Model.AnswerOffer(context.Player, command.Accept)
}

// move the active units of a stack to the other plane (tower, Astral Gate, Plane Shift)
type PlanarTravelCommand struct {
    Stack Location `json:"stack"`
}

func (command *PlanarTravelCommand) CommandType() string { return "planar-travel" }

func (command *PlanarTravelCommand) Execute(context *CommandContext) error {
    model := context.Model
    player := context.Player
    stack, err := context.ownStack(command.Stack)
    if err != nil {
        return err
    }

    // PlanarTravel moves the selected stack while the view is on the destination plane
    viewedPlane := model.Plane
    model.Plane = stack.Plane().Opposite()
    defer func() {
        model.Plane = viewedPlane
    }()

    player.SelectedStack = stack
    startPlane := stack.Plane()
    model.PlanarTravel(player)
    if player.SelectedStack == nil || player.SelectedStack.Plane() == startPlane {
        return errors.New("the stack can't travel to the other plane here")
    }
    return nil
}

// the stack's active units stop and patrol (they guard the tile until given a new order)
type PatrolCommand struct {
    Stack Location `json:"stack"`
}

func (command *PatrolCommand) CommandType() string { return "patrol" }

func (command *PatrolCommand) Execute(context *CommandContext) error {
    stack, err := context.ownStack(command.Stack)
    if err != nil {
        return err
    }
    for _, unit := range stack.ActiveUnits() {
        unit.SetBusy(units.BusyStatusPatrol)
    }
    stack.EnableMovers()
    return nil
}

// dismiss one of the player's units (by id, see GameModel.UnitId)
type DisbandUnitCommand struct {
    Unit int `json:"unit"`
}

func (command *DisbandUnitCommand) CommandType() string { return "disband-unit" }

func (command *DisbandUnitCommand) Execute(context *CommandContext) error {
    unit, _, err := context.Model.FindUnitById(context.Player, command.Unit)
    if err != nil {
        return err
    }
    context.Player.RemoveUnit(unit)
    context.Model.RefreshUI()
    return nil
}

func (context *CommandContext) wizardByBanner(banner string) (*playerlib.Player, error) {
    for _, player := range context.Model.Players {
        if player.GetBanner().String() == banner {
            return player, nil
        }
    }
    return nil, fmt.Errorf("no wizard with banner %q", banner)
}

// a wizard the player knows and can talk to
func (context *CommandContext) rival(banner string) (*playerlib.Player, error) {
    other, err := context.wizardByBanner(banner)
    if err != nil {
        return nil, err
    }
    if other == context.Player || other.IsNeutral() || other.Defeated || !context.Player.IsAwareOf(other) {
        return nil, fmt.Errorf("you can't talk to %v", other.Wizard.Name)
    }
    return other, nil
}

// split power between mana, research and casting skill (shares 0-1 adding up to 1)
type SetPowerDistributionCommand struct {
    Mana float64 `json:"mana"`
    Research float64 `json:"research"`
    Skill float64 `json:"skill"`
}

func (command *SetPowerDistributionCommand) CommandType() string { return "set-power-distribution" }

func (command *SetPowerDistributionCommand) Execute(context *CommandContext) error {
    return context.Player.SetPowerDistribution(command.Mana, command.Research, command.Skill)
}

// convert mana into gold ("mana-to-gold") or gold into mana ("gold-to-mana")
type AlchemyCommand struct {
    Direction string `json:"direction"`
    Amount int `json:"amount"`
}

func (command *AlchemyCommand) CommandType() string { return "alchemy" }

func (command *AlchemyCommand) Execute(context *CommandContext) error {
    switch command.Direction {
        case "mana-to-gold":
            _, err := context.Player.Alchemy(true, command.Amount)
            return err
        case "gold-to-mana":
            _, err := context.Player.Alchemy(false, command.Amount)
            return err
    }
    return fmt.Errorf("unknown direction %q", command.Direction)
}

// choose whether a unit in a stack moves with the stack (move-stack only moves active units;
// the others stay behind as their own stack)
type SetUnitActiveCommand struct {
    // the unit's id (see GameModel.UnitId)
    Unit int `json:"unit"`
    Active bool `json:"active"`
}

func (command *SetUnitActiveCommand) CommandType() string { return "set-unit-active" }

func (command *SetUnitActiveCommand) Execute(context *CommandContext) error {
    unit, stack, err := context.Model.FindUnitById(context.Player, command.Unit)
    if err != nil {
        return err
    }
    // units in stasis can't be toggled
    if unit.GetBusy() == units.BusyStatusStasis {
        return errors.New("the unit is in stasis")
    }
    stack.SetActive(unit, command.Active)
    return nil
}

type RenameCityCommand struct {
    City Location `json:"city"`
    Name string `json:"name"`
}

func (command *RenameCityCommand) CommandType() string { return "rename-city" }

func (command *RenameCityCommand) Execute(context *CommandContext) error {
    city, err := context.ownCity(command.City)
    if err != nil {
        return err
    }
    if command.Name == "" {
        return errors.New("empty name")
    }
    city.Name = command.Name
    return nil
}

// rename one of the player's heroes (index 0-5, as in the vault / player view)
type RenameHeroCommand struct {
    Hero int `json:"hero"`
    Name string `json:"name"`
}

func (command *RenameHeroCommand) CommandType() string { return "rename-hero" }

func (command *RenameHeroCommand) Execute(context *CommandContext) error {
    if command.Hero < 0 || command.Hero >= len(context.Player.Heroes) || context.Player.Heroes[command.Hero] == nil {
        return fmt.Errorf("no hero %v", command.Hero)
    }
    if command.Name == "" {
        return errors.New("empty name")
    }
    context.Player.Heroes[command.Hero].SetName(command.Name)
    return nil
}

// swap the items in two slots: the vault, a hero's equipment, or a newly arrived item
// (see GameModel.MoveItem; moving an item to a hero elsewhere costs mana)
type MoveItemCommand struct {
    From ItemSlot `json:"from"`
    To ItemSlot `json:"to"`
}

func (command *MoveItemCommand) CommandType() string { return "move-item" }

func (command *MoveItemCommand) Execute(context *CommandContext) error {
    return context.Model.MoveItem(context.Player, command.From, command.To)
}

// destroy an item for mana
type DestroyItemCommand struct {
    Item ItemSlot `json:"item"`
}

func (command *DestroyItemCommand) CommandType() string { return "destroy-item" }

func (command *DestroyItemCommand) Execute(context *CommandContext) error {
    _, err := context.Model.DestroyItem(context.Player, command.Item)
    return err
}

// propose a treaty ("pact", "alliance" or "peace") to a wizard
type ProposeTreatyCommand struct {
    Wizard string `json:"wizard"`
    Treaty string `json:"treaty"`
}

func (command *ProposeTreatyCommand) CommandType() string { return "propose-treaty" }

func (command *ProposeTreatyCommand) Execute(context *CommandContext) error {
    other, err := context.rival(command.Wizard)
    if err != nil {
        return err
    }
    return context.Player.ProposeTreaty(other, command.Treaty)
}

// offer a tribute to a wizard: a tenth of the treasury (gold) or a spell they don't know
type OfferTributeCommand struct {
    Wizard string `json:"wizard"`
    Gold bool `json:"gold,omitempty"`
    Spell string `json:"spell,omitempty"`
}

func (command *OfferTributeCommand) CommandType() string { return "offer-tribute" }

func (command *OfferTributeCommand) Execute(context *CommandContext) error {
    other, err := context.rival(command.Wizard)
    if err != nil {
        return err
    }
    if command.Gold == (command.Spell != "") {
        return errors.New("offer either gold or a spell")
    }
    if command.Gold {
        _, err := context.Player.TributeGold(other)
        return err
    }
    return context.Player.TributeSpell(other, context.Player.KnownSpells.FindByName(command.Spell))
}

// offer a spell in trade: if the wizard is willing, they say which spell they'd give back (a
// trade-offer event); answer-trade then makes the exchange or declines it, as on the diplomacy
// screen
type TradeSpellsCommand struct {
    Wizard string `json:"wizard"`
    Spell string `json:"spell"`
}

func (command *TradeSpellsCommand) CommandType() string { return "trade-spells" }

func (command *TradeSpellsCommand) Execute(context *CommandContext) error {
    player := context.Player
    other, err := context.rival(command.Wizard)
    if err != nil {
        return err
    }
    give := player.KnownSpells.FindByName(command.Spell)
    if !give.Valid() || other.KnownSpells.Contains(give) {
        return fmt.Errorf("%v can't be traded to %v", command.Spell, other.Wizard.Name)
    }
    if !player.WillTradeSpells(other) {
        return fmt.Errorf("%v doesn't want to trade spells now", other.Wizard.Name)
    }
    received, ok := player.SpellOfferedBy(other)
    if !ok {
        return fmt.Errorf("%v has no spell you don't know", other.Wizard.Name)
    }

    offer := &GameEventTradeOffer{Player: player, Wizard: other, Give: give, Receive: received}
    model := context.Model
    if model.PendingTrades == nil {
        model.PendingTrades = make(map[*playerlib.Player]*GameEventTradeOffer)
    }
    model.PendingTrades[player] = offer
    select {
        case model.Events <- offer:
        default:
    }
    return nil
}

// accept or decline the spell trade a wizard offered (trade-offer)
type AnswerTradeCommand struct {
    Accept bool `json:"accept"`
}

func (command *AnswerTradeCommand) CommandType() string { return "answer-trade" }

func (command *AnswerTradeCommand) Execute(context *CommandContext) error {
    model := context.Model
    player := context.Player
    offer, ok := model.PendingTrades[player]
    if !ok {
        return errors.New("no spell trade is waiting for an answer")
    }
    delete(model.PendingTrades, player)

    if !command.Accept {
        return nil
    }
    if err := player.TradeSpells(offer.Wizard, offer.Give, offer.Receive); err != nil {
        return err
    }
    select {
        case model.Events <- &GameEventLearnedSpell{Player: player, Spell: offer.Receive}:
        default:
    }
    return nil
}

var commandTypes = map[string]func() Command{
    "move-stack": func() Command { return &MoveStackCommand{} },
    "set-research": func() Command { return &SetResearchCommand{} },
    "set-tax-rate": func() Command { return &SetTaxRateCommand{} },
    "set-city-production": func() Command { return &SetCityProductionCommand{} },
    "set-city-farmers": func() Command { return &SetCityFarmersCommand{} },
    "buy-production": func() Command { return &BuyProductionCommand{} },
    "sell-building": func() Command { return &SellBuildingCommand{} },
    "build-action": func() Command { return &BuildActionCommand{} },
    "build-road": func() Command { return &BuildRoadCommand{} },
    "end-turn": func() Command { return &EndTurnCommand{} },
    "cast-spell": func() Command { return &CastSpellCommand{} },
    "cast-spell-target": func() Command { return &CastSpellTargetCommand{} },
    "answer-offer": func() Command { return &AnswerOfferCommand{} },
    "planar-travel": func() Command { return &PlanarTravelCommand{} },
    "patrol": func() Command { return &PatrolCommand{} },
    "disband-unit": func() Command { return &DisbandUnitCommand{} },
    "set-power-distribution": func() Command { return &SetPowerDistributionCommand{} },
    "alchemy": func() Command { return &AlchemyCommand{} },
    "set-unit-active": func() Command { return &SetUnitActiveCommand{} },
    "rename-city": func() Command { return &RenameCityCommand{} },
    "rename-hero": func() Command { return &RenameHeroCommand{} },
    "move-item": func() Command { return &MoveItemCommand{} },
    "destroy-item": func() Command { return &DestroyItemCommand{} },
    "propose-treaty": func() Command { return &ProposeTreatyCommand{} },
    "offer-tribute": func() Command { return &OfferTributeCommand{} },
    "trade-spells": func() Command { return &TradeSpellsCommand{} },
    "answer-trade": func() Command { return &AnswerTradeCommand{} },
    "battle-act": func() Command { return &BattleActCommand{} },
    "battle-done": func() Command { return &BattleDoneCommand{} },
    "battle-wait": func() Command { return &BattleWaitCommand{} },
    "battle-flee": func() Command { return &BattleFleeCommand{} },
    "battle-auto": func() Command { return &BattleAutoCommand{} },
    "battle-cast": func() Command { return &BattleCastCommand{} },
    "confirm-encounter": func() Command { return &ConfirmEncounterCommand{} },
    "raze-city": func() Command { return &RazeCityCommand{} },
}

type commandEnvelope struct {
    Type string `json:"type"`
    Command json.RawMessage `json:"command"`
}

func EncodeCommand(command Command) ([]byte, error) {
    body, err := json.Marshal(command)
    if err != nil {
        return nil, err
    }
    return json.Marshal(commandEnvelope{Type: command.CommandType(), Command: body})
}

func DecodeCommand(encoded []byte) (Command, error) {
    var envelope commandEnvelope
    if err := json.Unmarshal(encoded, &envelope); err != nil {
        return nil, err
    }

    makeCommand, ok := commandTypes[envelope.Type]
    if !ok {
        return nil, fmt.Errorf("unknown command type %q", envelope.Type)
    }

    command := makeCommand()
    if len(envelope.Command) > 0 {
        if err := json.Unmarshal(envelope.Command, command); err != nil {
            return nil, fmt.Errorf("bad %v command: %v", envelope.Type, err)
        }
    }
    return command, nil
}
