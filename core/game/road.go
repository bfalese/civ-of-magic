package game

import (
    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/units"
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
