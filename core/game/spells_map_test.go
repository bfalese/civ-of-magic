package game

import (
    "image"
    "testing"

    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/maplib"
    "github.com/bfalese/civ-of-magic/core/terrain"
)

type noCities struct {
}

func (noCities) ContainsCity(x int, y int, plane data.Plane) bool {
    return false
}

// a model whose current map is a rows x columns land map with empty extras on every tile
func makeSpellTestModel(rows int, columns int) *GameModel {
    terrainData := terrain.MakeTerrainData([]image.Image{nil}, []terrain.TerrainTile{terrain.TerrainTile{TileIndex: 0, Tile: terrain.TileLand}})
    extras := make(map[image.Point]map[maplib.ExtraKind]maplib.ExtraTile)
    for x := 0; x < columns; x++ {
        for y := 0; y < rows; y++ {
            extras[image.Pt(x, y)] = make(map[maplib.ExtraKind]maplib.ExtraTile)
        }
    }

    arcanus := &maplib.Map{
        Data: terrainData,
        Plane: data.PlaneArcanus,
        Map: terrain.MakeMap(rows, columns),
        CityProvider: noCities{},
        ExtraMap: extras,
    }

    return &GameModel{
        Plane: data.PlaneArcanus,
        ArcanusMap: arcanus,
    }
}

func TestApplyTransmuteSwapsBonuses(test *testing.T) {
    model := makeSpellTestModel(5, 5)
    pairs := map[data.BonusType]data.BonusType{
        data.BonusCoal: data.BonusGem,
        data.BonusGem: data.BonusCoal,
        data.BonusIronOre: data.BonusGoldOre,
        data.BonusGoldOre: data.BonusIronOre,
        data.BonusSilverOre: data.BonusMithrilOre,
        data.BonusMithrilOre: data.BonusSilverOre,
    }

    for from, to := range pairs {
        model.ArcanusMap.SetBonus(2, 2, from)
        model.ApplyTransmute(data.PlaneArcanus, 2, 2)
        if got := model.ArcanusMap.GetBonusTile(2, 2); got != to {
            test.Errorf("transmute %v: got %v, want %v", from, got, to)
        }
    }

    // other bonuses are left alone
    model.ArcanusMap.SetBonus(2, 2, data.BonusWildGame)
    model.ApplyTransmute(data.PlaneArcanus, 2, 2)
    if got := model.ArcanusMap.GetBonusTile(2, 2); got != data.BonusWildGame {
        test.Errorf("transmute wild game: got %v", got)
    }
}

func TestApplyEnchantRoadEnchantsRoadsInRange(test *testing.T) {
    model := makeSpellTestModel(9, 9)
    useMap := model.ArcanusMap
    useMap.SetRoad(4, 4, false) // center
    useMap.SetRoad(6, 6, false) // corner of the 5x5 square
    useMap.SetRoad(7, 4, false) // outside

    model.ApplyEnchantRoad(data.PlaneArcanus, 4, 4)

    check := func(x int, y int, want bool) {
        road, ok := useMap.ExtraMap[image.Pt(x, y)][maplib.ExtraKindRoad].(*maplib.ExtraRoad)
        if !ok {
            test.Fatalf("road at %v,%v missing", x, y)
        }
        if road.Enchanted != want {
            test.Errorf("road at %v,%v: enchanted=%v, want %v", x, y, road.Enchanted, want)
        }
    }
    check(4, 4, true)
    check(6, 6, true)
    check(7, 4, false)

    if useMap.ContainsRoad(5, 5) {
        test.Errorf("enchant road must not create roads")
    }
}

func TestApplyCorruption(test *testing.T) {
    model := makeSpellTestModel(5, 5)
    model.ApplyCorruption(data.PlaneArcanus, 1, 3)
    if !model.ArcanusMap.HasCorruption(1, 3) {
        test.Errorf("expected corruption at 1,3")
    }
}

func TestApplyWarpNodeWithoutNode(test *testing.T) {
    model := makeSpellTestModel(3, 3)
    // no node on the tile: nothing happens (and nothing panics)
    model.ApplyWarpNode(data.PlaneArcanus, 1, 1, nil)
}
