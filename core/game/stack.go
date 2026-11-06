package game

import (
    "log"
    "math/rand/v2"
    "slices"

    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/units"
)

// enforces the stack size limit at a tile: moves random units to nearby free tiles, then removes
// the cheapest units (heroes last) if there is still no room
func (model *GameModel) ResolveStackAt(x int, y int, plane data.Plane) {
    stack, player := model.FindStack(x, y, plane)
    if stack == nil {
        return
    }

    count := len(stack.Units())
    if count <= data.MaxUnitsInStack {
        return
    }

    // try to move random units to a nearby tile
    stackUnits := stack.Units()
    for _, i := range rand.Perm(len(stackUnits)) {
        unit := stackUnits[i]

        positions := model.FindEscapePosition(player, unit)

        if len(positions) != 0 {
            // set to a random position
            position := positions[rand.IntN(len(positions))]
            unit.SetX(position.X)
            unit.SetY(position.Y)

            // merge stacks
            stack.RemoveUnit(unit)
            player.AddStack(playerlib.MakeUnitStackFromUnits([]units.StackUnit{unit}))
            allStacks := player.FindAllStacks(position.X, position.Y, stack.Plane())
            for i := 1; i < len(allStacks); i++ {
                player.MergeStacks(allStacks[0], allStacks[i])
            }

            count -= 1
            if count <= data.MaxUnitsInStack {
                break
            }
        }
    }

    // kill units until enough room
    if count > data.MaxUnitsInStack {
        stackUnits = stack.Units()
        slices.SortFunc(stackUnits, func(unitA, unitB units.StackUnit) int {
            // non-heros before heroes
            if unitA.IsHero() != unitB.IsHero() {
                if unitA.IsHero() {
                    return 1
                }
                return -1
            }

            // low-leveled heroes first
            if unitA.IsHero() && unitB.IsHero() {
                return unitA.GetExperience() - unitB.GetExperience()
            }

            // low production or casting cost first
            minCostA := min(unitA.GetRawUnit().ProductionCost, unitA.GetRawUnit().CastingCost)
            minCostB := min(unitB.GetRawUnit().ProductionCost, unitB.GetRawUnit().CastingCost)
            return minCostA - minCostB
        })

        for _, unit := range stackUnits {
            log.Printf("Unit %v killed by ResolveStack", unit)
            player.RemoveUnit(unit)
            count -= 1
            if count <= data.MaxUnitsInStack {
                break
            }
        }
    }
}

// moves a unit to the player's summoning city (merging stacks there); does nothing without one
func (model *GameModel) RelocateUnit(player *playerlib.Player, unit units.StackUnit) {
    summonCity := player.FindSummoningCity()
    if summonCity == nil {
        return
    }

    player.UpdateUnitLocation(unit, summonCity.X, summonCity.Y, summonCity.Plane)

    allStacks := player.FindAllStacks(summonCity.X, summonCity.Y, summonCity.Plane)
    for i := 1; i < len(allStacks); i++ {
        player.MergeStacks(allStacks[0], allStacks[i])
    }

    model.ResolveStackAt(summonCity.X, summonCity.Y, summonCity.Plane)

    unit.SetBusy(units.BusyStatusNone)
}

func (model *GameModel) GetPlayerByBanner(banner data.BannerType) *playerlib.Player {
    for _, player := range model.Players {
        if player.GetBanner() == banner {
            return player
        }
    }

    return nil
}

// the player whose stack holds the unit, nil if none
func (model *GameModel) GetUnitOwner(unit units.StackUnit) *playerlib.Player {
    for _, player := range model.Players {
        if player.FindStackByUnit(unit) != nil {
            return player
        }
    }
    return nil
}
