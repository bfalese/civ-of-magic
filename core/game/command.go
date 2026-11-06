package game

import (
    "encoding/json"
    "errors"
    "fmt"

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

// move a stack to a destination: find a path through explored land and walk it as far as its
// moves go (the rest of the path is kept for next turns). Battles and encounters on the way go
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

    path, ok := model.FindPath(stack.X(), stack.Y(), command.To.X, command.To.Y, player, stack, player.GetFog(stack.Plane()))
    if !ok {
        return fmt.Errorf("no path to %v,%v", command.To.X, command.To.Y)
    }

    stack.CurrentPath = path
    stack.SetBuildRoadPath(nil)
    player.SelectedStack = stack
    model.MoveSelectedStack(context.Handlers, player)
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
        if building != buildinglib.BuildingTradeGoods && building != buildinglib.BuildingHousing && !city.GetBuildableBuildings().Contains(building) {
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
