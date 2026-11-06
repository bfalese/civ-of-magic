package game

import (
    "log"
    "slices"

    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/units"
)

// An AI player's turn: the AI decides (player.AIBehavior.Update, slow, the frontend runs it in the
// background), ApplyAiDecisions carries out the decisions, MoveAiStacks walks the stacks along
// their paths (battles and encounters go through the handlers), then the turn ends.
// DoAiTurn does all of it in one go, for headless runs.

// carry out the AI's decisions; suggestCityName names new outposts
func (model *GameModel) ApplyAiDecisions(player *playerlib.Player, decisions []playerlib.AIDecision, suggestCityName func(race data.Race) string) {
    for _, decision := range decisions {
        switch decision.(type) {
            case *playerlib.AIMoveStackDecision:
                moveDecision := decision.(*playerlib.AIMoveStackDecision)
                useStack := moveDecision.Stack

                if len(moveDecision.Units) > 0 && len(moveDecision.Units) != len(moveDecision.Stack.Units()) {
                    useStack = player.SplitStack(moveDecision.Stack, moveDecision.Units)
                }

                useStack.CurrentPath = moveDecision.Path

            // mainly for the raider ai
            case *playerlib.AICreateUnitDecision:
                create := decision.(*playerlib.AICreateUnitDecision)
                log.Printf("ai %v creating %+v", player.Wizard.Name, create)

                overworldUnit := units.MakeOverworldUnitFromUnit(create.Unit, create.X, create.Y, create.Plane, player.Wizard.Banner, player.MakeExperienceInfo(), player.MakeUnitEnchantmentProvider())
                if create.Patrol {
                    overworldUnit.SetBusy(units.BusyStatusPatrol)
                }
                player.CreateUnit(overworldUnit)
                model.ResolveStackAt(create.X, create.Y, create.Plane)
            case *playerlib.AIUpdateCityDecision:
                update := decision.(*playerlib.AIUpdateCityDecision)

                update.City.Farmers = update.Farmers
                update.City.Workers = update.Workers
                update.City.ResetCitizens()

            case *playerlib.AIBuildOutpostDecision:
                build := decision.(*playerlib.AIBuildOutpostDecision)

                stack := build.Stack.GetActiveUnitWithAbility(data.AbilityCreateOutpost)
                if stack != nil {
                    model.CreateOutpost(stack, player, suggestCityName(stack.GetRace()))
                }
            case *playerlib.AIBuildRoadDecision:
                build := decision.(*playerlib.AIBuildRoadDecision)
                roadStack := build.Stack
                path := model.FindRoadPath(roadStack.X(), roadStack.Y(), build.X, build.Y, player, roadStack, player.GetFog(roadStack.Plane()))
                if len(path) > 0 {
                    for _, unit := range roadStack.Units() {
                        if unit.HasAbility(data.AbilityConstruction) {
                            unit.SetBuildRoadPath(path)
                        }
                    }
                    model.MaybeBuildRoads(roadStack, player)
                }
            case *playerlib.AIMeldNodeDecision:
                meld := decision.(*playerlib.AIMeldNodeDecision)
                model.TryMeldNode(meld.Stack, player)
            case *playerlib.AIPlaneShiftDecision:
                shift := decision.(*playerlib.AIPlaneShiftDecision)
                if err := model.PlaneShift(shift.Stack, player); err == nil {
                    model.AnnounceVisibleWizards()
                    shift.Stack.CurrentPath = nil
                }
            case *playerlib.AIProduceDecision:
                produce := decision.(*playerlib.AIProduceDecision)
                log.Printf("Year=%v AI %v(%v) city %v producing %v %v", model.TurnNumber, player.Wizard.Name, player.GetBanner(), produce.City.Name, model.BuildingInfo.Name(produce.Building), produce.Unit.Name)
                produce.City.ProducingBuilding = produce.Building
                produce.City.ProducingUnit = produce.Unit
            case *playerlib.AIResearchSpellDecision:
                research := decision.(*playerlib.AIResearchSpellDecision)
                if player.ResearchingSpell.Invalid() {
                    player.ResearchingSpell = research.Spell
                }

            case *playerlib.AICastSpellDecision:
                cast := decision.(*playerlib.AICastSpellDecision)

                if player.CastingSpell.Invalid() {
                    player.CastingSpell = cast.Spell
                }
            case *playerlib.AICastUnitSpellDecision:
                cast := decision.(*playerlib.AICastUnitSpellDecision)
                if player.CastingSpell.Invalid() {
                    player.CastingSpell = cast.Spell
                    player.CastingSpellTarget = cast.Target
                }
        }
    }
}

// walk the AI's stacks along their paths, continue road building, meld nodes, then let the AI
// look at the result (PostUpdate)
func (model *GameModel) MoveAiStacks(handlers MovementHandler, player *playerlib.Player) {

    for _, stack := range slices.Clone(player.Stacks) {
        // stop moving once any unit in the stack has no moves left
        for !stack.AnyOutOfMoves() && len(stack.CurrentPath) > 0 {
            stack.CurrentPath = model.DoAiMoveUnit(handlers, player, stack)
        }
    }

    // continue any in-progress road building and attempt to meld any node
    // a stack is now standing on (engineers/melders may have just arrived).
    for _, stack := range slices.Clone(player.Stacks) {
        for _, unit := range stack.Units() {
            if unit.GetBusy() == units.BusyStatusBuildRoad && len(unit.GetBuildRoadPath()) > 0 {
                model.MaybeBuildRoads(stack, player)
                break
            }
        }
    }
    for _, stack := range slices.Clone(player.Stacks) {
        model.TryMeldNode(stack, player)
    }

    player.AIBehavior.PostUpdate(player, model)
}

// a whole AI turn without a frontend: decide, act, move. Does not end the turn.
func (model *GameModel) DoAiTurn(handlers MovementHandler, player *playerlib.Player, suggestCityName func(race data.Race) string) {
    if player.AIBehavior == nil {
        return
    }

    decisions := player.AIBehavior.Update(player, model)
    model.ApplyAiDecisions(player, decisions, suggestCityName)
    model.MoveAiStacks(handlers, player)
}

// announce first meetings with wizards that became visible, as diplomacy events
// (for code that can't show the meeting right away). returns true if any wizard was met.
func (model *GameModel) AnnounceVisibleWizards() bool {
    meetings := model.DiscoverVisibleWizards()
    for _, meeting := range meetings {
        select {
            case model.Events <- meeting:
            default:
        }
    }
    return len(meetings) > 0
}

// settle an outpost where the settlers stand; the settlers are used up
func (model *GameModel) CreateOutpost(settlers units.StackUnit, player *playerlib.Player, cityName string) *citylib.City {
    newCity := citylib.MakeCity(cityName, settlers.GetX(), settlers.GetY(), settlers.GetRace(), model.BuildingInfo, model.GetMap(settlers.GetPlane()), model, player)
    newCity.Plane = settlers.GetPlane()
    newCity.Population = 300
    newCity.Outpost = true
    newCity.ProducingBuilding = buildinglib.BuildingHousing
    newCity.ProducingUnit = units.UnitNone

    model.GetMap(settlers.GetPlane()).SetRoad(settlers.GetX(), settlers.GetY(), settlers.GetPlane() == data.PlaneMyrror)

    player.RemoveUnit(settlers)
    player.SelectedStack = nil
    model.RefreshUI()
    player.AddCity(newCity)

    stack := player.FindStack(newCity.X, newCity.Y, newCity.Plane)

    select {
        case model.Events<- &GameEventNewOutpost{City: newCity, Stack: stack, Player: player}:
        default:
    }

    return newCity
}
