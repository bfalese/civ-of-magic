package main

import (
    "encoding/json"
    "image/png"
    "os"
    "path/filepath"
    "testing"

    "github.com/bfalese/civ-of-magic/core/assets"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/maplib"
    "github.com/bfalese/civ-of-magic/core/units"
)

// every asset ID the views can produce is in the manifest, inside its sheet.
// needs MOM_DATA=/path/to/Master of Magic, skipped otherwise (CI has no data)
func TestExport(test *testing.T) {
    dataPath := os.Getenv("MOM_DATA")
    if dataPath == "" {
        test.Skip("MOM_DATA not set")
    }

    out := test.TempDir()
    if _, err := run(dataPath, out); err != nil {
        test.Fatalf("export: %v", err)
    }

    encoded, err := os.ReadFile(filepath.Join(out, ManifestName))
    if err != nil {
        test.Fatalf("manifest: %v", err)
    }
    var manifest Manifest
    if err := json.Unmarshal(encoded, &manifest); err != nil {
        test.Fatalf("manifest JSON: %v", err)
    }
    if manifest.TileWidth != 20 || manifest.TileHeight != 18 {
        test.Errorf("tile size %vx%v", manifest.TileWidth, manifest.TileHeight)
    }

    var sizes [][2]int
    for _, name := range manifest.Sheets {
        file, err := os.Open(filepath.Join(out, name))
        if err != nil {
            test.Fatalf("sheet: %v", err)
        }
        config, err := png.DecodeConfig(file)
        file.Close()
        if err != nil {
            test.Fatalf("sheet %v: %v", name, err)
        }
        sizes = append(sizes, [2]int{config.Width, config.Height})
    }

    check := func(id string) {
        frames, ok := manifest.Sprites[id]
        if !ok || len(frames) == 0 {
            test.Errorf("missing %v", id)
            return
        }
        for _, frame := range frames {
            sheet, x, y, width, height := frame[0], frame[1], frame[2], frame[3], frame[4]
            if sheet < 0 || sheet >= len(sizes) || x < 0 || y < 0 || width <= 0 || height <= 0 || x + width > sizes[sheet][0] || y + height > sizes[sheet][1] {
                test.Errorf("%v: frame %v outside the sheets", id, frame)
            }
        }
    }

    for _, unit := range units.AllUnits {
        for _, banner := range Banners {
            check(assets.Unit(unit.Race, unit.Name, banner))
        }
    }
    for _, banner := range Banners {
        check(assets.UnitBackground(banner))
        for _, size := range assets.CitySizes {
            check(assets.City(size, false, banner))
            check(assets.City(size, true, banner))
        }
        if banner != data.BannerBrown {
            check(assets.NodeZone(banner))
        }
    }
    for _, direction := range assets.RoadDirections {
        check(assets.Road(direction, false))
        check(assets.Road(direction, true))
    }
    for bonus := data.BonusWildGame; bonus <= data.BonusCrysxCrystal; bonus++ {
        if bonus.LbxIndex() != -1 {
            check(assets.Bonus(bonus))
        }
    }
    for _, encounter := range []maplib.EncounterType{maplib.EncounterTypeLair, maplib.EncounterTypeCave, maplib.EncounterTypePlaneTower, maplib.EncounterTypeAncientTemple, maplib.EncounterTypeFallenTemple, maplib.EncounterTypeRuins, maplib.EncounterTypeAbandonedKeep, maplib.EncounterTypeDungeon} {
        check(assets.Encounter(encounter))
    }
    for _, unit := range units.AllUnits {
        for _, facing := range assets.Facings {
            check(assets.BattleUnit(unit.Race, unit.Name, facing))
        }
    }
    for _, file := range []string{"cmbgrass.lbx", "cmbgrasc.lbx", "cmbdesrt.lbx", "cmbdesrc.lbx", "cmbmount.lbx", "cmbmounc.lbx", "cmbtundr.lbx", "cmbtundc.lbx"} {
        check(assets.BattleTile(file, 0))
        check(assets.BattleTile(file, 31))
        check(assets.BattleTop(file, 48))
        check(assets.BattleTop(file, 57))
    }
    check(assets.BattleTile("cmbtcity.lbx", 109))
    check(assets.BattleTile("chriver.lbx", 12))
    check(assets.BattleTop("cmbtcity.lbx", 19))
    check(assets.BattleMud)
    check(assets.BattleWall(0))
    check(assets.BattleWall(63))
    check(assets.BattleTop("cmbtcity.lbx", 17))
    check(assets.BattleRoad)
    if len(manifest.BannerMarker) != 4 || len(manifest.BannerColors) != len(Banners) || len(manifest.BannerColors["red"]) != 4 {
        test.Errorf("banner colours: %v %v", manifest.BannerMarker, manifest.BannerColors)
    }
    check(assets.OpenTower)
    check(assets.Corruption)
    check(assets.TerrainTile(0))
    check(assets.Item(0))
    check(assets.Item(115))

    animated := 0
    for index := 0; ; index++ {
        frames, ok := manifest.Sprites[assets.TerrainTile(index)]
        if !ok {
            if index < 1000 {
                test.Errorf("only %v terrain tiles", index)
            }
            break
        }
        if len(frames) > 1 {
            animated += 1
        }
    }
    if animated == 0 {
        test.Errorf("expected animated terrain tiles (ocean, nodes)")
    }
}
