package game

import (
    "errors"
    "fmt"
    "image"

    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/units"
    "github.com/bfalese/civ-of-magic/core/pathfinding"
    "github.com/bfalese/civ-of-magic/lib/functional"
)

// return the number of engineers in the stack. only consider ones currently building a road if buildingRoad is true
func ComputeEngineerCount(stack *playerlib.UnitStack, buildingRoad bool) int {
    engineerCount := 0
    for _, unit := range stack.Units() {
        if unit.HasAbility(data.AbilityConstruction) && (!buildingRoad || unit.GetBusy() == units.BusyStatusBuildRoad) {
            engineerCount += 1

            if unit.GetRace() == data.RaceDwarf {
                engineerCount += 1
            }

            if unit.HasEnchantment(data.UnitEnchantmentEndurance) {
                engineerCount += 1
            }
        }
    }

    return engineerCount
}

func (model *GameModel) FindRoadPath(oldX int, oldY int, newX int, newY int, player *playerlib.Player, stack playerlib.PathStack, fog data.FogMap) pathfinding.Path {
    useMap := model.GetMap(stack.Plane())

    if newY < 0 || newY >= useMap.Height() {
        return nil
    }

    oldX = useMap.WrapX(oldX)
    newX = useMap.WrapX(newX)

    normalized := func (a image.Point) image.Point {
        return image.Pt(useMap.WrapX(a.X), a.Y)
    }

    // check equality of two points taking wrapping into account
    tileEqual := func (a image.Point, b image.Point) bool {
        return normalized(a) == normalized(b)
    }

    // cache locations of enemies
    enemyStacks := make(map[image.Point]struct{})
    enemyCities := make(map[image.Point]struct{})

    for _, enemy := range model.Players {
        if enemy != player {
            for _, enemyStack := range enemy.Stacks {
                enemyStacks[image.Pt(enemyStack.X(), enemyStack.Y())] = struct{}{}
            }
            for _, enemyCity := range enemy.Cities {
                enemyCities[image.Pt(enemyCity.X, enemyCity.Y)] = struct{}{}
            }
        }
    }

    // cache the containsEnemy result
    // true if the given coordinates contain an enemy unit or city
    containsEnemy := functional.Memoize2(func (x int, y int) bool {
        _, ok := enemyStacks[image.Pt(x, y)]
        if ok {
            return true
        }
        _, ok = enemyCities[image.Pt(x, y)]
        if ok {
            return true
        }

        return false
    })

    tileCost := func (x1 int, y1 int, x2 int, y2 int) float64 {
        x1 = useMap.WrapX(x1)
        x2 = useMap.WrapX(x2)

        if x1 < 0 || x1 >= useMap.Width() || y1 < 0 || y1 >= useMap.Height() {
            return pathfinding.Infinity
        }

        if x2 < 0 || x2 >= useMap.Width() || y2 < 0 || y2 >= useMap.Height() {
            return pathfinding.Infinity
        }

        if fog[x2][y2] == data.FogTypeUnexplored {
            return pathfinding.Infinity
        }

        tile := useMap.GetTile(x2, y2)
        if tile.Tile.IsWater() {
            return pathfinding.Infinity
        }

        // FIXME: it might be more optimal to put the infinity cases into the neighbors function instead

        // avoid encounters
        encounter := useMap.GetEncounter(x2, y2)
        if encounter != nil {
            if !tileEqual(image.Pt(x2, y2), image.Pt(newX, newY)) {
                return pathfinding.Infinity
            }
        }

        // avoid enemy units/cities
        if !tileEqual(image.Pt(x2, y2), image.Pt(newX, newY)) && containsEnemy(x2, y2) {
            return pathfinding.Infinity
        }

        if tile.HasRoad() {
            if stack.Plane() == data.PlaneMyrror {
                // should non-corporeal engineers take more than 0 time to cross a road?
                return 0
            }
            return 0.5
        }

        if x1 != x2 && y1 != y2 {
            // make diagonals more expensive
            return 1.1
        }

        return 1
    }

    neighbors := func (x int, y int) []image.Point {
        return pathfinding.NeighborsWrapX(x, y, useMap.Height(), useMap.WrapX)
    }

    path, ok := pathfinding.FindPath(image.Pt(oldX, oldY), image.Pt(newX, newY), 10000, tileCost, neighbors, tileEqual)
    if ok {
        return path
    }

    return nil
}

// the road the player's engineers at stack would build to reach to (what build-road would do),
// from the stack's tile to the destination; an error if there are no engineers or no way
func (model *GameModel) RoadPathFor(player *playerlib.Player, stackAt Location, to Location) ([]Location, error) {
    stack := player.FindStack(stackAt.X, stackAt.Y, stackAt.Plane)
    if stack == nil {
        return nil, fmt.Errorf("no stack of yours at %v,%v", stackAt.X, stackAt.Y)
    }
    if !stack.ActiveUnitsHasAbility(data.AbilityConstruction) {
        return nil, errors.New("no engineers in the stack")
    }
    path := model.FindRoadPath(stack.X(), stack.Y(), to.X, to.Y, player, stack, player.GetFog(stack.Plane()))
    if len(path) == 0 {
        return nil, fmt.Errorf("no road path to %v,%v", to.X, to.Y)
    }
    out := []Location{}
    for _, point := range path {
        out = append(out, Location{Plane: stack.Plane(), X: point.X, Y: point.Y})
    }
    return out, nil
}
