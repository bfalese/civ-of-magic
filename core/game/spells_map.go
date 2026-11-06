package game

import (
    "math/rand/v2"

    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/terrain"
    "github.com/bfalese/civ-of-magic/core/units"
)

// Effects of overworld map spells. The frontend plays the spell animation and calls these at the
// same moment the effect used to happen (some mid-animation, at a given frame).

// Corruption (applied at animation frame 6)
// FIXME (inherited): the row check is always true (|| instead of &&)
func (model *GameModel) ApplyCorruption(plane data.Plane, x int, y int) {
    mapObject := model.GetMap(plane)
    if y >= 0 || y < mapObject.Map.Rows() {
        x = mapObject.WrapX(x)
        mapObject.SetCorruption(x, y)
    }
}

// Raise Volcano, terrain part (applied at animation frame 8 for each animated tile)
func (model *GameModel) ApplyRaiseVolcanoTerrain(plane data.Plane, x int, y int, tileX int, tileY int) {
    mapObject := model.GetMap(plane)
    mapObject.Map.SetTerrainAt(x, y, terrain.Grass, mapObject.Data, mapObject.Plane)
    mapObject.SetBonus(tileX, tileY, data.BonusNone)
}

// Raise Volcano, after the animation
func (model *GameModel) ApplyRaiseVolcano(plane data.Plane, tileX int, tileY int, player *playerlib.Player) {
    mapObject := model.GetMap(plane)
    mapObject.SetVolcano(tileX, tileY, player)

    // volcanoes may destroy buildings if cast in a city
    for _, player := range model.Players {
        city := player.FindCity(tileX, tileY, mapObject.Plane)
        if city != nil {
            for _, building := range city.Buildings.Values() {
                if rand.N(100) < 15 {
                    city.Buildings.Remove(building)
                }
            }
        }
    }
}

// Enchant Road, after the animation
func (model *GameModel) ApplyEnchantRoad(plane data.Plane, tileX int, tileY int) {
    useMap := model.GetMap(plane)

    // all roads in a 5x5 square around the target tile should become enchanted
    for dx := -2; dx <= 2; dx++ {
        for dy := -2; dy <= 2; dy++ {
            cx := useMap.WrapX(tileX + dx)
            cy := tileY + dy
            if cy < 0 || cy >= useMap.Height() {
                continue
            }

            if useMap.ContainsRoad(cx, cy) {
                useMap.SetRoad(cx, cy, true)
            }
        }
    }
}

// Change Terrain (applied at animation frame 7)
func (model *GameModel) ApplyChangeTerrain(plane data.Plane, x int, y int) {
    mapObject := model.GetMap(plane)
    switch mapObject.GetTile(x, y).Tile.TerrainType() {
        case terrain.Desert, terrain.Forest, terrain.Hill, terrain.Swamp:
            mapObject.Map.SetTerrainAt(x, y, terrain.Grass, mapObject.Data, mapObject.Plane)
        case terrain.Grass:
            mapObject.Map.SetTerrainAt(x, y, terrain.Forest, mapObject.Data, mapObject.Plane)
        case terrain.Volcano:
            mapObject.RemoveVolcano(x, y)
        case terrain.Mountain:
            mapObject.Map.SetTerrainAt(x, y, terrain.Hill, mapObject.Data, mapObject.Plane)
    }
}

// Transmute (applied at animation frame 6)
func (model *GameModel) ApplyTransmute(plane data.Plane, x int, y int) {
    mapObject := model.GetMap(plane)
    switch mapObject.GetBonusTile(x, y) {
        case data.BonusCoal: mapObject.SetBonus(x, y, data.BonusGem)
        case data.BonusGem: mapObject.SetBonus(x, y, data.BonusCoal)
        case data.BonusIronOre: mapObject.SetBonus(x, y, data.BonusGoldOre)
        case data.BonusGoldOre: mapObject.SetBonus(x, y, data.BonusIronOre)
        case data.BonusSilverOre: mapObject.SetBonus(x, y, data.BonusMithrilOre)
        case data.BonusMithrilOre: mapObject.SetBonus(x, y, data.BonusSilverOre)
    }
}

// Warp Node, after the animation
func (model *GameModel) ApplyWarpNode(plane data.Plane, tileX int, tileY int, caster *playerlib.Player) {
    node := model.GetMap(plane).GetMagicNode(tileX, tileY)
    if node != nil {
        node.Warped = true
        node.WarpedOwner = caster
    }
}

// Floating Island (applied at animation frame 5); the frontend then checks for newly met wizards
func (model *GameModel) ApplyFloatingIsland(plane data.Plane, player *playerlib.Player, tileX int, tileY int) {
    overworldUnit := units.MakeOverworldUnitFromUnit(units.FloatingIsland, tileX, tileY, model.GetMap(plane).Plane, player.Wizard.Banner, player.MakeExperienceInfo(), player.MakeUnitEnchantmentProvider())
    player.AddUnit(overworldUnit)
    player.LiftFog(tileX, tileY, 1, plane)
}
