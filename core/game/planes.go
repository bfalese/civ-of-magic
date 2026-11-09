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

    cityStackInfo := model.ComputeCityStackInfo()
    if err := model.planeShiftCheck(stack, player, cityStackInfo); err != nil {
        return err
    }

    // if there is a friendly stack at the new location then merge the stacks
    mergeStack := cityStackInfo.FindFriendlyStack(stack.X(), stack.Y(), stack.Plane().Opposite(), player)
    stack.SetPlane(stack.Plane().Opposite())
    if mergeStack != nil {
        player.MergeStacks(stack, mergeStack)
    }
    player.UpdateFogVisibility()

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

// Awareness: the owner sees every city, and meets the wizards who own them
func (model *GameModel) ExploreFogForAwareness(awarenessOwner *playerlib.Player) {
    for _, city := range model.AllCities() {
        if city.GetBanner() == awarenessOwner.GetBanner() {
            continue // No need, those cities do already provide vision
        }
        awarenessOwner.ExploreFogSquare(city.X, city.Y, 1, city.Plane)
        _, owner := model.FindCity(city.X, city.Y, city.Plane)
        if owner != nil {
            if meeting := model.MakeWizardContact(awarenessOwner, owner); meeting != nil {
                select {
                    case model.Events <- meeting:
                    default:
                }
            }
        }
    }
}

// how a stack could go to the other plane from where it stands: an open tower, an Astral Gate in
// a city of the player's on either plane, or Plane Shift on its active units
func (model *GameModel) hasPlanarTravelWay(player *playerlib.Player, stack *playerlib.UnitStack) bool {
    if model.GetMap(stack.Plane()).HasOpenTower(stack.X(), stack.Y()) {
        return true
    }
    cityThisPlane := player.FindCity(stack.X(), stack.Y(), stack.Plane())
    cityOppositePlane := player.FindCity(stack.X(), stack.Y(), stack.Plane().Opposite())
    if (cityThisPlane != nil && cityThisPlane.HasEnchantment(data.CityEnchantmentAstralGate)) ||
       (cityOppositePlane != nil && cityOppositePlane.HasEnchantment(data.CityEnchantmentAstralGate)) {
        return true
    }
    return stack.ActiveUnitsHasAbility(data.AbilityPlaneShift)
}

// whether PlanarTravel would move the stack's active units now (for the player view)
func (model *GameModel) CanPlanarTravel(player *playerlib.Player, stack *playerlib.UnitStack) bool {
    if stack == nil || len(stack.ActiveUnits()) == 0 || model.IsGlobalEnchantmentActive(data.EnchantmentPlanarSeal) {
        return false
    }
    if !model.hasPlanarTravelWay(player, stack) {
        return false
    }
    return model.planeShiftCheck(stack, player, model.ComputeCityStackInfo()) == nil
}

// the active units of the player's stack go to the other plane, through an open tower, an Astral
// Gate, or by Plane Shift. Planar Seal blocks it. The stack's own plane counts, not the one the
// frontend shows. On success player.SelectedStack is the stack that arrived.
func (model *GameModel) PlanarTravel(player *playerlib.Player, stack *playerlib.UnitStack) error {
    if stack == nil {
        return errors.New("no stack")
    }
    if model.IsGlobalEnchantmentActive(data.EnchantmentPlanarSeal) {
        return errors.New("Planar Seal keeps everyone on their plane")
    }
    if !model.hasPlanarTravelWay(player, stack) {
        return errors.New("planar travel needs a tower, an Astral Gate or Plane Shift")
    }

    activeStack := player.SplitActiveStack(stack)
    if len(activeStack.Units()) == 0 {
        return errors.New("no active units to travel")
    }

    if err := model.PlaneShift(activeStack, player); err != nil {
        if activeStack != stack {
            player.MergeStacks(stack, activeStack)
        }
        select {
            case model.Events <- &GameEventNotice{Message: "The selected units cannot planar travel at this location."}:
            default:
        }
        return err
    }

    // PlaneShift merges into a friendly stack on the other side: find the stack that holds them
    arrived := activeStack
    if leader := activeStack.Leader(); leader != nil {
        if found := player.FindStack(leader.GetX(), leader.GetY(), leader.GetPlane()); found != nil {
            arrived = found
        }
    }
    player.SelectedStack = arrived
    player.UpdateFogVisibility()
    model.AnnounceVisibleWizards()
    return nil
}

// why the stack can't land on the other plane at its position (nil if it can): water for walkers,
// an encounter, an enemy, a city it may not enter
func (model *GameModel) planeShiftCheck(stack *playerlib.UnitStack, player *playerlib.Player, cityStackInfo playerlib.CityStackInfo) error {
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

    if cityStackInfo.ContainsEnemy(stack.X(), stack.Y(), stack.Plane().Opposite(), player) {
        canMove = false
    }

    // no matter what the reason, just emit a message that planar travel is not possible
    if !canMove {
        return errors.New("The selected units cannot planar travel at this location.")
    }

    return nil
}
