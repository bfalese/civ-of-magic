package game

import (
    "fmt"

    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/core/terrain"
)

// what kind of overworld tile a spell targets
type LocationType int
const (
    LocationTypeAny LocationType = iota
    LocationTypeLand
    LocationTypeEmptyWater
    LocationTypeFriendlyCity
    LocationTypeEnemyCity
    LocationTypeFriendlyUnit
    LocationTypeEnemyUnit
    LocationTypeChangeTerrain
    LocationTypeTransmute
    LocationTypeRaiseVolcano
    LocationTypeEnemyMeldedNode
    LocationTypeDisenchant
)

// checks whether the tile is a valid target for a spell of the given location type, as seen by the
// player. Returns the tile x to use (wrapped around the map for map-tile types), whether the target
// is valid, and a message to show when the target is refused for a reason the player should know.
// cityStackInfo is a snapshot of cities and stacks (see ComputeCityStackInfo).
func (model *GameModel) CheckSpellTarget(player *playerlib.Player, spell spellbook.Spell, locationType LocationType, entityInfo playerlib.CityStackInfo, tileX int, tileY int) (int, bool, string) {
    mapObject := model.CurrentMap()

    switch locationType {
        case LocationTypeAny: return tileX, true, ""
        case LocationTypeLand:
            if tileY >= 0 && tileY < mapObject.Height() {
                tileX = mapObject.WrapX(tileX)
                if player.IsTileExplored(tileX, tileY, model.Plane) {
                    if mapObject.GetTile(tileX, tileY).Tile.IsLand() {
                        return tileX, true, ""
                    }
                }
            }
        case LocationTypeEmptyWater:
            if tileY >= 0 && tileY < mapObject.Height() {
                tileX = mapObject.WrapX(tileX)
                if player.IsTileExplored(tileX, tileY, model.Plane) {
                    if mapObject.GetTile(tileX, tileY).Tile.IsWater() {
                        empty := true
                        for _, enemy := range model.GetEnemies(player) {
                            if enemy.FindStack(tileX, tileY, model.Plane) != nil {
                                empty = false
                                break
                            }
                        }

                        if empty {
                            return tileX, true, ""
                        }
                    }
                }
            }
        case LocationTypeFriendlyCity:
            city := player.FindCity(tileX, tileY, model.Plane)
            if city != nil {
                return tileX, true, ""
            }
        case LocationTypeChangeTerrain:
            if tileY >= 0 && tileY < mapObject.Height() {
                tileX = mapObject.WrapX(tileX)

                if player.IsTileExplored(tileX, tileY, model.Plane) {
                    terrainType := mapObject.GetTile(tileX, tileY).Tile.TerrainType()
                    switch terrainType {
                        case terrain.Desert, terrain.Forest, terrain.Hill,
                             terrain.Swamp, terrain.Grass, terrain.Volcano,
                             terrain.Mountain:
                            return tileX, true, ""
                    }
                }
            }
        case LocationTypeTransmute:
            if tileY >= 0 && tileY < mapObject.Height() {
                tileX = mapObject.WrapX(tileX)

                if player.IsTileExplored(tileX, tileY, model.Plane) {
                    bonusType := mapObject.GetBonusTile(tileX, tileY)
                    switch bonusType {
                        case data.BonusCoal, data.BonusGem, data.BonusIronOre,
                             data.BonusGoldOre, data.BonusSilverOre, data.BonusMithrilOre:
                            return tileX, true, ""
                    }
                }
            }
        case LocationTypeRaiseVolcano:
            if tileY >= 0 && tileY < mapObject.Height() {
                tileX = mapObject.WrapX(tileX)

                if player.IsTileExplored(tileX, tileY, model.Plane) {
                    terrainType := mapObject.GetTile(tileX, tileY).Tile.TerrainType()
                    switch terrainType {
                        case terrain.Desert, terrain.Forest, terrain.Swamp, terrain.Grass, terrain.Tundra:
                            return tileX, true, ""
                    }
                }
            }
        case LocationTypeEnemyMeldedNode:
            if tileY >= 0 && tileY < mapObject.Height() {
                tileX = mapObject.WrapX(tileX)

                if player.IsTileExplored(tileX, tileY, model.Plane) {
                    node := mapObject.GetMagicNode(tileX, tileY)
                    if node != nil && node.MeldingWizard != player {
                        return tileX, true, ""
                    }
                }
            }

        case LocationTypeDisenchant:
            // return if the tile has a stack, town, or is a magic node

            if entityInfo.FindStack(tileX, tileY, model.Plane) != nil ||
               entityInfo.FindCity(tileX, tileY, model.Plane) != nil ||
               mapObject.GetMagicNode(tileX, tileY) != nil {

                return tileX, true, ""
            }

        case LocationTypeEnemyCity:
            for _, enemy := range model.GetEnemies(player) {
                city := enemy.FindCity(tileX, tileY, model.Plane)
                if city != nil {
                    if !city.CanTarget(spell) {
                        return tileX, false, fmt.Sprintf("You cannot cast %v on this city", spell.Name)
                    } else {
                        return tileX, true, ""
                    }
                }
            }

        case LocationTypeFriendlyUnit:
            stack := player.FindStack(tileX, tileY, model.Plane)
            if stack != nil {
                return tileX, true, ""
            }

        case LocationTypeEnemyUnit:
            if player.IsVisible(tileX, tileY, model.Plane) {
                stack := entityInfo.FindStack(tileX, tileY, model.Plane)
                if stack != nil && entityInfo.ContainsEnemy(tileX, tileY, model.Plane, player) {
                    city := entityInfo.FindCity(tileX, tileY, model.Plane)

                    if city != nil && !city.CanTarget(spell) {
                        return tileX, false, fmt.Sprintf("You cannot cast %v on this unit", spell.Name)
                    } else {
                        return tileX, true, ""
                    }
                }
            }
    }

    return tileX, false, ""
}

// a stable name for JSON
func (locationType LocationType) Name() string {
    switch locationType {
        case LocationTypeAny: return "any"
        case LocationTypeLand: return "land"
        case LocationTypeEmptyWater: return "empty-water"
        case LocationTypeFriendlyCity: return "friendly-city"
        case LocationTypeEnemyCity: return "enemy-city"
        case LocationTypeFriendlyUnit: return "friendly-unit"
        case LocationTypeEnemyUnit: return "enemy-unit"
        case LocationTypeChangeTerrain: return "change-terrain"
        case LocationTypeTransmute: return "transmute"
        case LocationTypeRaiseVolcano: return "raise-volcano"
        case LocationTypeEnemyMeldedNode: return "enemy-melded-node"
        case LocationTypeDisenchant: return "disenchant"
    }
    return "unknown"
}
