package game

import (
    "github.com/bfalese/civ-of-magic/core/units"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/lib/fraction"
)

// the build button: the selected stack settles an outpost, melds a node, or purifies corruption,
// whichever it can do first. returns true if it should build a road instead (the frontend asks
// where). suggestCityName names a new outpost.
func (model *GameModel) DoBuildAction(player *playerlib.Player, suggestCityName func(race data.Race) string) bool {
    if player.SelectedStack != nil {
        var powers UnitBuildPowers

        if player.SelectedStack != nil {
            powers = ComputeUnitBuildPowers(player.SelectedStack)
        }

        // if a stack has multiple units with different build powers, only do the first legal one found

        if powers.CreateOutpost {
            // search for the settlers (the only unit with the create outpost ability
            for _, settlers := range player.SelectedStack.ActiveUnits() {
                if model.IsSettlableLocation(settlers.GetX(), settlers.GetY(), settlers.GetPlane()) && settlers.HasAbility(data.AbilityCreateOutpost) {
                    model.CreateOutpost(settlers, player, suggestCityName(settlers.GetRace()))
                    model.RefreshUI()
                    return false
                }
            }
        }

        if powers.Meld {
            node := model.GetMap(player.SelectedStack.Plane()).GetMagicNode(player.SelectedStack.X(), player.SelectedStack.Y())
            if node != nil {
                for _, melder := range player.SelectedStack.ActiveUnits() {
                    if melder.HasAbility(data.AbilityMeld) && !node.Warped {
                        model.DoMeld(melder, player, node)
                        model.RefreshUI()
                        return false
                    }
                }
            }
        }

        if powers.Purify {

            tile := model.GetMap(player.SelectedStack.Plane()).GetTile(player.SelectedStack.X(), player.SelectedStack.Y())
            if tile.Corrupted() {
                for _, unit := range player.SelectedStack.ActiveUnits() {
                    if unit.HasAbility(data.AbilityPurify) {
                        unit.SetBusy(units.BusyStatusPurify)
                        unit.SetMovesLeft(true, fraction.Zero())
                    }
                }

                player.SelectedStack.EnableMovers()

                // player.SelectedStack.ExhaustMoves()
                model.RefreshUI()
                return false
            }

        }

        if powers.BuildRoad {
            // the frontend asks the user where to build the road
            return true
        }
    }

    return false
}
