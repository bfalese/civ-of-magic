package game

import (
    "fmt"

    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/combat"
    "github.com/bfalese/civ-of-magic/core/maplib"
)

// frontend steps while a player's selected stack walks its path (see MoveSelectedStack)
type PlayerMovementHandler interface {
    MovementHandler
    // ask whether to enter a lair, node or tower on the path
    ConfirmEncounter(encounter *maplib.ExtraEncounter) bool
    // the stack moved one step and keeps going (the frontend refreshes the HUD)
    SteppedStack()
    // the stack is out of moves: select the next stack that can move
    NextUnit(player *playerlib.Player)
}

// walk the player's selected stack along its path, one step at a time, until it runs out of moves,
// reaches the end, or stops (encounter, battle, city capture, meeting a wizard, blocked tile).
// The frontend shows each step and the dialogs/battles through handlers.
func (model *GameModel) MoveSelectedStack(handlers PlayerMovementHandler, player *playerlib.Player) {
    stack := player.SelectedStack
    if stack == nil || len(stack.ActiveUnits()) == 0 {
        return
    }

    player.MovedStacksThisTurn += 1

    mapUse := model.GetMap(stack.Plane())

    stepsTaken := 0
    stopMoving := false
    var mergeStack *playerlib.UnitStack
    // kind of a hack, in case the stack couldn't move due to spell ward or something we attempt to merge
    // the stack with whatever stack it is standing on
    for _, otherStack := range player.FindAllStacks(stack.X(), stack.Y(), stack.Plane()) {
        if otherStack != stack {
            mergeStack = otherStack
        }
    }

    getStack := func(x int, y int) (playerlib.PathStack, bool) {
        found := player.FindStack(mapUse.WrapX(x), y, stack.Plane())
        return found, found != nil
    }

    entityInfo := model.ComputeCityStackInfo()

    quitMoving:
    for i, step := range stack.CurrentPath {
        if stack.AnyOutOfMoves() {
            break
        }

        oldX := stack.X()
        oldY := stack.Y()

        // already at this location, just skip it
        if oldX == step.X && oldY == step.Y {
            stepsTaken = i + 1
            continue
        }

        city := entityInfo.FindCity(step.X, step.Y, stack.Plane())
        if city != nil {
            // units might not be able to enter a city if the city has spell wards in effect
            for _, unit := range stack.ActiveUnits() {
                if !city.CanEnter(unit) {
                    stopMoving = true
                    model.Events <- &GameEventNotice{Message: fmt.Sprintf("%v can not enter the city of %v", unit.GetRawUnit().Name, city.Name)}
                    break quitMoving
                }
            }
        }

        terrainCost, canMove := model.ComputeTerrainCost(stack, stack.X(), stack.Y(), step.X, step.Y, mapUse, getStack)

        if canMove {

            encounter := mapUse.GetEncounter(mapUse.WrapX(step.X), step.Y)
            if encounter != nil {
                if handlers.ConfirmEncounter(encounter) {
                    stack.Move(step.X - stack.X(), step.Y - stack.Y(), terrainCost, model.GetNormalizeCoordinateFunc())
                    handlers.ShowMovement(oldX, oldY, stack, true)
                    player.LiftFogSquare(stack.X(), stack.Y(), stack.GetSightRange(), stack.Plane())
                    handlers.DiscoverWizards()

                    stack.ExhaustMoves()
                    state := handlers.DoEncounter(player, stack, encounter, mapUse, stack.X(), stack.Y())
                    if state == combat.CombatStateAttackerFlee {
                        stack.SetX(oldX)
                        stack.SetY(oldY)
                    }

                    model.RefreshUI()
                } else {
                    encounter.ExploredBy.Insert(player)
                }

                stopMoving = true
                break quitMoving
            }

            stepsTaken = i + 1
            mergeStack = player.FindStack(mapUse.WrapX(step.X), step.Y, stack.Plane())

            stack.Move(step.X - stack.X(), step.Y - stack.Y(), terrainCost, model.GetNormalizeCoordinateFunc())
            handlers.ShowMovement(oldX, oldY, stack, true)
            player.LiftFogSquare(stack.X(), stack.Y(), stack.GetSightRange(), stack.Plane())
            metWizard := handlers.DiscoverWizards()

            if entityInfo.ContainsEnemy(stack.X(), stack.Y(), stack.Plane(), player) {
                // FIXME: this should get all stacks at the given location and merge them into a single stack for combat
                otherStack := entityInfo.FindStack(stack.X(), stack.Y(), stack.Plane())
                if otherStack != nil {
                    otherCity := entityInfo.FindCity(stack.X(), stack.Y(), stack.Plane())
                    zone := combat.ZoneType{
                        City: otherCity,
                    }

                    defenderPlayer := model.GetStackOwner(otherStack)

                    // note: doCombat will already call defeatCity if the attacker wins the battle
                    state := handlers.DoCombat(player, stack, defenderPlayer, otherStack, zone)
                    if state == combat.CombatStateAttackerFlee {
                        stack.SetX(oldX)
                        stack.SetY(oldY)
                    } else if state == combat.CombatStateDefenderFlee {
                        model.DoMoveFleeingDefender(defenderPlayer, otherStack)
                    }

                    stack.ExhaustMoves()
                    model.RefreshUI()

                    stopMoving = true
                    break quitMoving
                }

                // defeat any unguarded cities immediately
                otherCity := entityInfo.FindCity(stack.X(), stack.Y(), stack.Plane())
                if otherCity != nil {
                    defenderPlayer := model.GetCityOwner(otherCity)
                    raze, gold := handlers.DefeatCity(player, stack, defenderPlayer, otherCity)

                    // FIXME: show a notice about any fame won
                    player.Fame = max(0, player.Fame + otherCity.FameForCaptureOrRaze(!raze))
                    defenderPlayer.Fame = max(0, defenderPlayer.Fame + otherCity.FameForCaptureOrRaze(false))
                    player.Gold += gold
                    defenderPlayer.Gold -= gold

                    stack.ExhaustMoves()
                    model.RefreshUI()

                    stopMoving = true
                    break quitMoving

                }
            }

            if metWizard {
                stopMoving = true
                break quitMoving
            }

            // have to force the ui to refresh because we are not processing events here
            handlers.SteppedStack()

            // some units in the stack might not have any moves left
            /*
            beforeActive := len(stack.ActiveUnits())
            stack.EnableMovers()
            afterActive := len(stack.ActiveUnits())
            if afterActive > 0 && afterActive != beforeActive {
                // stopMoving = true
                break
            }
            */
        } else {
            // can't move, so abort the rest of the path
            stopMoving = true
            break
        }
    }

    if stopMoving {
        stack.CurrentPath = nil
        // if any engineeers were following a path then force them to stop building
        stack.SetBuildRoadPath(nil)
    } else if stepsTaken > 0 {
        stack.CurrentPath = stack.CurrentPath[stepsTaken:]
    }

    // there might be some units in the stack that can still move, but if they move as a group
    // then all units should be out of moves once the unit with least amount of movement points is done
    if stack.GetRemainingMoves().IsZero() {
        stack.ExhaustMoves()
        handlers.NextUnit(player)
        model.RefreshUI()
    }

    // only merge stacks if both stacks are stopped, otherwise they can move through each other
    if len(stack.CurrentPath) == 0 && mergeStack != nil && mergeStack != stack && mergeStack.X() == stack.X() && mergeStack.Y() == stack.Y() {
        stack = player.MergeStacks(mergeStack, stack)
        player.SelectedStack = stack
        model.RefreshUI()
    }

    // update unrest for new units in the city
    newCity := player.FindCity(stack.X(), stack.Y(), stack.Plane())
    if newCity != nil {
        newCity.UpdateUnrest()
    }

    /*
    if stepsTaken > 0 {
        if stack.AnyOutOfMoves() {
            stack.ExhaustMoves()
            handlers.NextUnit(player)
        }

        model.RefreshUI()
    }
    */
}
