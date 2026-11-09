package game

import (
    "github.com/bfalese/civ-of-magic/core/units"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/lib/fraction"
)

// what the build button does for a stack, the first that applies to its active units:
// "settle" (settlers on a tile where an outpost can go), "meld" (a spirit on an unwarped node),
// "purify" (priests on corrupted land), "road" (engineers: the frontend asks where), or ""
func (model *GameModel) StackBuildAction(stack *playerlib.UnitStack) string {
    if stack == nil {
        return ""
    }
    powers := ComputeUnitBuildPowers(stack)

    if powers.CreateOutpost && model.IsSettlableLocation(stack.X(), stack.Y(), stack.Plane()) {
        return "settle"
    }

    if powers.Meld {
        node := model.GetMap(stack.Plane()).GetMagicNode(stack.X(), stack.Y())
        if node != nil && !node.Warped {
            return "meld"
        }
    }

    if powers.Purify {
        tile := model.GetMap(stack.Plane()).GetTile(stack.X(), stack.Y())
        if tile.Corrupted() {
            return "purify"
        }
    }

    if powers.BuildRoad {
        return "road"
    }

    return ""
}

// the build button: the selected stack settles an outpost, melds a node, or purifies corruption,
// whichever it can do first (StackBuildAction). returns true if it should build a road instead
// (the frontend asks where). suggestCityName names a new outpost.
func (model *GameModel) DoBuildAction(player *playerlib.Player, suggestCityName func(race data.Race) string) bool {
    stack := player.SelectedStack

    switch model.StackBuildAction(stack) {
        case "settle":
            for _, settlers := range stack.ActiveUnits() {
                if settlers.HasAbility(data.AbilityCreateOutpost) {
                    model.CreateOutpost(settlers, player, suggestCityName(settlers.GetRace()))
                    model.RefreshUI()
                    return false
                }
            }

        case "meld":
            node := model.GetMap(stack.Plane()).GetMagicNode(stack.X(), stack.Y())
            for _, melder := range stack.ActiveUnits() {
                if melder.HasAbility(data.AbilityMeld) {
                    model.DoMeld(melder, player, node)
                    model.RefreshUI()
                    return false
                }
            }

        case "purify":
            for _, unit := range stack.ActiveUnits() {
                if unit.HasAbility(data.AbilityPurify) {
                    unit.SetBusy(units.BusyStatusPurify)
                    unit.SetMovesLeft(true, fraction.Zero())
                }
            }
            stack.EnableMovers()
            model.RefreshUI()
            return false

        case "road":
            // the frontend asks the user where to build the road
            return true
    }

    return false
}
