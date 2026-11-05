package game

import (
    "errors"
    "slices"

    "github.com/bfalese/civ-of-magic/core/building"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/units"
)

func (model *GameModel) IsGlobalEnchantmentActive(enchantment data.Enchantment) bool {
    return slices.ContainsFunc(model.Players, func (player *playerlib.Player) bool {
        return player.GlobalEnchantments.Contains(enchantment)
    })
}

// moves a stack to the other plane at the same position, merging with a friendly stack there.
// Returns an error (and changes nothing) if planar travel isn't possible.
func (model *GameModel) PlaneShift(stack *playerlib.UnitStack, player *playerlib.Player) error {
    if model.IsGlobalEnchantmentActive(data.EnchantmentPlanarSeal) {
        return errors.New("Cannot plane shift while the Planar Seal is in effect")
    }

    canMove := false

    cityOppositePlane := player.FindCity(stack.X(), stack.Y(), stack.Plane().Opposite())

    mapPlane := model.GetMap(stack.Plane().Opposite())

    tile := mapPlane.GetTile(stack.X(), stack.Y())
    canMove = cityOppositePlane != nil || stack.AllFlyers() || tile.Tile.IsLand()

    if cityOppositePlane != nil {
        for _, unit := range stack.ActiveUnits() {
            if !cityOppositePlane.CanEnter(unit) {
                canMove = false
                break
            }
        }
    }

    // cannot planar travel if there is an encounter node
    if canMove && mapPlane.GetEncounter(stack.X(), stack.Y()) != nil {
        canMove = false
    }

    if canMove && tile.Tile.IsWater() && stack.AllLandWalkers() {
        canMove = false
    }

    cityStackInfo := model.ComputeCityStackInfo()

    if cityStackInfo.ContainsEnemy(stack.X(), stack.Y(), stack.Plane().Opposite(), player) {
        canMove = false
    }

    // no matter what the reason, just emit a message that planar travel is not possible
    if !canMove {
        return errors.New("The selected units cannot planar travel at this location.")
    }

    if canMove {
        // if there is a friendly stack at the new location then merge the stacks
        mergeStack := cityStackInfo.FindFriendlyStack(stack.X(), stack.Y(), stack.Plane().Opposite(), player)
        stack.SetPlane(stack.Plane().Opposite())
        if mergeStack != nil {
            player.MergeStacks(stack, mergeStack)
        }
        player.UpdateFogVisibility()
    }

    return nil
}

// Lycanthropy: the unit becomes a werewolf with no experience; it keeps weapon bonus and enchantments
func (model *GameModel) ApplyLycanthropy(unit units.StackUnit) {
    overworldUnit, ok := unit.(*units.OverworldUnit)
    if ok {
        overworldUnit.Unit = units.WereWolf
        overworldUnit.Experience = 0
        // unit keeps weapon bonus and enchantments
    }
}

// Spell of Return: the banished wizard returns, with their fortress and summoning circle in city
func (model *GameModel) ApplySpellOfReturn(player *playerlib.Player, city *citylib.City) {
    player.Banished = false

    for _, check := range player.Cities {
        check.RemoveBuilding(building.BuildingSummoningCircle)
        check.RemoveBuilding(building.BuildingFortress)
    }

    city.Buildings.Insert(building.BuildingFortress)
    city.Buildings.Insert(building.BuildingSummoningCircle)
    player.Wizard.Race = city.Race
}
