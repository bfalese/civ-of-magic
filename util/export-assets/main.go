// export-assets reads the overland map images (terrain, units, cities, roads, bonuses, lairs...)
// from an original Master of Magic install and writes them as PNG sprite sheets plus a manifest
// (assets.json) that maps asset IDs (core/assets) to rectangles in the sheets. Web frontends load
// these instead of LBX files; replacing a sprite later means changing the image behind its ID.
//
//   go run ./util/export-assets -data "/path/to/Master of Magic" -out ~/.local/share/civ-of-magic/assets
//
// The output is derived from copyrighted data: don't commit it.
//
// Manifest:
//
//   {"version": 1, "tileWidth": 20, "tileHeight": 18, "sheets": ["sheet-0.png", ...],
//    "sprites": {"terrain/0": [[sheet, x, y, width, height], ...one per animation frame], ...}}
package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "image"
    "image/color"
    "image/draw"
    "image/png"
    "log"
    "os"
    "path/filepath"
    "slices"
    "sort"

    "github.com/bfalese/civ-of-magic/core/assets"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/maplib"
    "github.com/bfalese/civ-of-magic/core/units"
    "github.com/bfalese/civ-of-magic/game/magic/lbxdata"
    "github.com/bfalese/civ-of-magic/game/magic/unitview"
    "github.com/bfalese/civ-of-magic/game/magic/util"
    "github.com/bfalese/civ-of-magic/lib/lbx"
)

const ManifestName = "assets.json"

const sheetWidth = 1024
const sheetMaxHeight = 2048

var Banners = []data.BannerType{data.BannerBlue, data.BannerGreen, data.BannerPurple, data.BannerRed, data.BannerYellow, data.BannerBrown}

// a frame: sheet index, x, y, width, height
type Frame [5]int

type Manifest struct {
    Version int `json:"version"`
    TileWidth int `json:"tileWidth"`
    TileHeight int `json:"tileHeight"`
    Sheets []string `json:"sheets"`
    Sprites map[string][]Frame `json:"sprites"`
    // battle-unit sprites carry these four colours where the banner colour goes; replace them
    // with BannerColors[banner] (same order) to colour a unit for its owner
    BannerMarker [][3]uint8 `json:"bannerMarker"`
    BannerColors map[string][][3]uint8 `json:"bannerColors"`
}

// the banner palette entries units are recoloured in (unitview.MakeUpdateUnitColorsFunc)
const bannerPaletteStart = 215
const bannerPaletteCount = 4

// colours no MoM palette uses, standing in for the banner colour in battle-unit sprites
var bannerMarker = [][3]uint8{{255, 0, 254}, {254, 0, 253}, {253, 0, 252}, {252, 0, 251}}

type sprite struct {
    id string
    frames []image.Image
}

// collects sprites, then packs them into sheets (rows of sprites, tallest first)
type Exporter struct {
    cache *lbx.LbxCache
    sprites []sprite
    seen map[string]bool
}

func (exporter *Exporter) add(id string, frames []image.Image) error {
    if exporter.seen[id] {
        return fmt.Errorf("duplicate asset id %v", id)
    }
    if len(frames) == 0 {
        return fmt.Errorf("%v: no images", id)
    }
    exporter.seen[id] = true
    exporter.sprites = append(exporter.sprites, sprite{id: id, frames: frames})
    return nil
}

// every frame of an LBX entry, with the palette the game uses for it, optionally recoloured
func (exporter *Exporter) readFrames(lbxName string, index int, recolor func(*image.Paletted) image.Image) ([]image.Image, error) {
    lbxFile, err := exporter.cache.GetLbxFile(lbxName)
    if err != nil {
        return nil, err
    }

    paletteMap, err := lbx.GetPaletteOverrideMap(exporter.cache, lbxFile, lbxName)
    if err != nil {
        return nil, err
    }
    palette := paletteMap[index]
    if palette == nil {
        palette = paletteMap[-1]
    }

    sprites, err := lbxFile.ReadImagesWithPalette(index, palette, palette != nil)
    if err != nil {
        return nil, fmt.Errorf("%v %v: %v", lbxName, index, err)
    }

    var out []image.Image
    for _, sprite := range sprites {
        if recolor != nil {
            out = append(out, recolor(sprite))
        } else {
            out = append(out, sprite)
        }
    }
    return out, nil
}

func (exporter *Exporter) addLbx(id string, lbxName string, index int, recolor func(*image.Paletted) image.Image) error {
    frames, err := exporter.readFrames(lbxName, index, recolor)
    if err != nil {
        return err
    }
    return exporter.add(id, frames)
}

func (exporter *Exporter) collect() (int, int, error) {
    // terrain: one sprite per tile index (both planes), animated tiles have 4 frames
    terrainLbx, err := exporter.cache.GetLbxFile("terrain.lbx")
    if err != nil {
        return 0, 0, err
    }
    terrainData, err := lbxdata.ReadTerrainData(terrainLbx)
    if err != nil {
        return 0, 0, fmt.Errorf("terrain: %v", err)
    }
    tileWidth, tileHeight := 0, 0
    for index, tile := range terrainData.Tiles {
        if len(tile.Images) > 0 {
            tileWidth = tile.Images[0].Bounds().Dx()
            tileHeight = tile.Images[0].Bounds().Dy()
        }
        if err := exporter.add(assets.TerrainTile(index), tile.Images); err != nil {
            return 0, 0, err
        }
    }

    // units, recoloured for each banner as the game does
    for _, unit := range units.AllUnits {
        for _, banner := range Banners {
            recolor := unitview.MakeUpdateUnitColorsFunc(banner)
            frames, err := exporter.readFrames(unit.LbxFile, unit.Index, func(sprite *image.Paletted) image.Image {
                return recolor(sprite)
            })
            if err != nil {
                return 0, 0, fmt.Errorf("unit %v: %v", unit.Name, err)
            }
            // the overland sprite is the first frame
            if err := exporter.add(assets.Unit(unit.Race, unit.Name, banner), frames[:1]); err != nil {
                return 0, 0, err
            }
        }
    }

    // unit backgrounds and cities per banner
    backgrounds := map[data.BannerType]int{data.BannerBlue: 14, data.BannerGreen: 15, data.BannerPurple: 16, data.BannerRed: 17, data.BannerYellow: 18, data.BannerBrown: 19}
    for _, banner := range Banners {
        if err := exporter.addLbx(assets.UnitBackground(banner), "mapback.lbx", backgrounds[banner], nil); err != nil {
            return 0, 0, err
        }

        recolor := unitview.MakeUpdateUnitColorsFunc(banner)
        for _, walled := range []bool{false, true} {
            index := 21
            if walled {
                index = 20
            }
            frames, err := exporter.readFrames("mapback.lbx", index, func(sprite *image.Paletted) image.Image {
                return recolor(sprite)
            })
            if err != nil {
                return 0, 0, fmt.Errorf("cities: %v", err)
            }
            for sizeIndex, size := range assets.CitySizes {
                if sizeIndex >= len(frames) {
                    return 0, 0, fmt.Errorf("cities: only %v frames", len(frames))
                }
                if err := exporter.add(assets.City(size, walled, banner), frames[sizeIndex:sizeIndex+1]); err != nil {
                    return 0, 0, err
                }
            }
        }
    }

    // roads: a centre piece and one piece per direction (mapback 45-53, enchanted 54-62)
    for _, enchanted := range []bool{false, true} {
        base := 45
        if enchanted {
            base = 54
        }
        for offset, direction := range assets.RoadDirections {
            if err := exporter.addLbx(assets.Road(direction, enchanted), "mapback.lbx", base + offset, nil); err != nil {
                return 0, 0, err
            }
        }
    }

    // bonuses, encounters, the open tower, corruption, node zones
    bonuses := []data.BonusType{data.BonusWildGame, data.BonusNightshade, data.BonusSilverOre, data.BonusGoldOre, data.BonusIronOre, data.BonusCoal, data.BonusMithrilOre, data.BonusAdamantiumOre, data.BonusGem, data.BonusQuorkCrystal, data.BonusCrysxCrystal}
    for _, bonus := range bonuses {
        if err := exporter.addLbx(assets.Bonus(bonus), "mapback.lbx", bonus.LbxIndex(), nil); err != nil {
            return 0, 0, err
        }
    }

    encounters := []maplib.EncounterType{maplib.EncounterTypeLair, maplib.EncounterTypeCave, maplib.EncounterTypePlaneTower, maplib.EncounterTypeAncientTemple, maplib.EncounterTypeFallenTemple, maplib.EncounterTypeRuins, maplib.EncounterTypeAbandonedKeep, maplib.EncounterTypeDungeon}
    for _, encounter := range encounters {
        if err := exporter.addLbx(assets.Encounter(encounter), "mapback.lbx", encounter.LbxIndex(), nil); err != nil {
            return 0, 0, err
        }
    }

    if err := exporter.addLbx(assets.OpenTower, "mapback.lbx", 70, nil); err != nil {
        return 0, 0, err
    }
    if err := exporter.addLbx(assets.Corruption, "mapback.lbx", 77, nil); err != nil {
        return 0, 0, err
    }

    zones := map[data.BannerType]int{data.BannerBlue: 63, data.BannerGreen: 64, data.BannerPurple: 65, data.BannerRed: 66, data.BannerYellow: 67}
    for _, banner := range Banners {
        index, ok := zones[banner]
        if !ok {
            continue
        }
        if err := exporter.addLbx(assets.NodeZone(banner), "mapback.lbx", index, nil); err != nil {
            return 0, 0, err
        }
    }

    if err := exporter.collectBattle(); err != nil {
        return 0, 0, err
    }

    return tileWidth, tileHeight, nil
}

func cropped(sprite *image.Paletted) image.Image {
    return util.AutoCrop(sprite)
}

// the battlefield: ground tiles, trees/rocks/buildings (cropped, as the battle screen draws them),
// mud, the town road and clouds; and every unit's combat figures for the 8 facings
func (exporter *Exporter) collectBattle() error {
    terrainFiles := []string{"cmbgrass.lbx", "cmbgrasc.lbx", "cmbdesrt.lbx", "cmbdesrc.lbx", "cmbmount.lbx", "cmbmounc.lbx", "cmbtundr.lbx", "cmbtundc.lbx"}
    for _, file := range terrainFiles {
        // 0-47 ground (the model picks from the first 32), 48-57 trees and rocks
        for index := 0; index < 48; index++ {
            if err := exporter.addLbx(assets.BattleTile(file, index), file, index, nil); err != nil {
                return err
            }
        }
        for index := 48; index < 58; index++ {
            if err := exporter.addLbx(assets.BattleTop(file, index), file, index, cropped); err != nil {
                return err
            }
        }
    }

    // water: cmbtcity 109-112 on Arcanus, chriver 12-15 on Myrror
    for index := 109; index < 113; index++ {
        if err := exporter.addLbx(assets.BattleTile("cmbtcity.lbx", index), "cmbtcity.lbx", index, nil); err != nil {
            return err
        }
    }
    for index := 12; index < 16; index++ {
        if err := exporter.addLbx(assets.BattleTile("chriver.lbx", index), "chriver.lbx", index, nil); err != nil {
            return err
        }
    }

    // houses (2-6), the wizard's fortress (17), lair/tower/ruins/keep/temples (19-23), nodes (65, 66)
    for _, index := range []int{2, 3, 4, 5, 6, 17, 19, 20, 21, 22, 23, 65, 66} {
        if err := exporter.addLbx(assets.BattleTop("cmbtcity.lbx", index), "cmbtcity.lbx", index, cropped); err != nil {
            return err
        }
    }
    // city walls (0-35: stone, dark, ivy), walls of fire (36-49) and darkness (50-63)
    for index := 0; index < 64; index++ {
        if err := exporter.addLbx(assets.BattleWall(index), "citywall.lbx", index, nil); err != nil {
            return err
        }
    }
    if err := exporter.addLbx(assets.BattleMud, "cmbtcity.lbx", 118, nil); err != nil {
        return err
    }
    if err := exporter.addLbx(assets.BattleRoad, "cmbtcity.lbx", 0, cropped); err != nil {
        return err
    }
    if err := exporter.addLbx(assets.BattleClouds, "cmbtcity.lbx", 113, nil); err != nil {
        return err
    }

    markBanner := func(sprite *image.Paletted) image.Image {
        palette := make(color.Palette, len(sprite.Palette))
        copy(palette, sprite.Palette)
        for i, marker := range bannerMarker {
            palette[bannerPaletteStart + i] = color.RGBA{R: marker[0], G: marker[1], B: marker[2], A: 255}
        }
        sprite.Palette = palette
        return sprite
    }

    for _, unit := range units.AllUnits {
        if unit.CombatLbxFile == "" {
            continue
        }
        for _, facing := range assets.Facings {
            if err := exporter.addLbx(assets.BattleUnit(unit.Race, unit.Name, facing), unit.CombatLbxFile, unit.GetCombatIndex(facing), markBanner); err != nil {
                return fmt.Errorf("battle unit %v: %v", unit.Name, err)
            }
        }
    }
    return nil
}

// the four banner colours the game puts in the banner palette entries, per banner
func bannerColors() map[string][][3]uint8 {
    out := make(map[string][][3]uint8)
    for _, banner := range Banners {
        sprite := image.NewPaletted(image.Rect(0, 0, 1, 1), lbx.GetDefaultPalette())
        recolored := unitview.MakeUpdateUnitColorsFunc(banner)(sprite).(*image.Paletted)
        for i := range bannerPaletteCount {
            r, g, b, _ := recolored.Palette[bannerPaletteStart + i].RGBA()
            out[banner.String()] = append(out[banner.String()], [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)})
        }
    }
    return out
}

type placement struct {
    sprite int
    frame int
}

// shelf packing: frames sorted by height, rows left to right, a new sheet when one is full
func (exporter *Exporter) pack(outPath string) (*Manifest, error) {
    var order []placement
    for spriteIndex, sprite := range exporter.sprites {
        for frameIndex := range sprite.frames {
            order = append(order, placement{sprite: spriteIndex, frame: frameIndex})
        }
    }
    frameImage := func(item placement) image.Image {
        return exporter.sprites[item.sprite].frames[item.frame]
    }
    sort.SliceStable(order, func(i, j int) bool {
        return frameImage(order[i]).Bounds().Dy() > frameImage(order[j]).Bounds().Dy()
    })

    manifest := &Manifest{Version: 1, Sprites: make(map[string][]Frame)}
    frames := make([][]Frame, len(exporter.sprites))
    for spriteIndex, sprite := range exporter.sprites {
        frames[spriteIndex] = make([]Frame, len(sprite.frames))
    }

    var sheet *image.RGBA
    var sheetIndex, x, y, rowHeight, usedHeight int

    save := func() error {
        if sheet == nil {
            return nil
        }
        name := fmt.Sprintf("sheet-%v.png", sheetIndex)
        cropped := sheet.SubImage(image.Rect(0, 0, sheetWidth, usedHeight))
        file, err := os.Create(filepath.Join(outPath, name))
        if err != nil {
            return err
        }
        defer file.Close()
        if err := png.Encode(file, cropped); err != nil {
            return err
        }
        manifest.Sheets = append(manifest.Sheets, name)
        return nil
    }

    for _, item := range order {
        picture := frameImage(item)
        width, height := picture.Bounds().Dx(), picture.Bounds().Dy()
        if width > sheetWidth || height > sheetMaxHeight {
            return nil, fmt.Errorf("%v is too big (%vx%v)", exporter.sprites[item.sprite].id, width, height)
        }

        if sheet != nil && x + width > sheetWidth {
            x = 0
            y += rowHeight
            rowHeight = 0
        }
        if sheet == nil || y + height > sheetMaxHeight {
            if err := save(); err != nil {
                return nil, err
            }
            if sheet != nil {
                sheetIndex += 1
            }
            sheet = image.NewRGBA(image.Rect(0, 0, sheetWidth, sheetMaxHeight))
            x, y, rowHeight, usedHeight = 0, 0, 0, 0
        }

        target := image.Rect(x, y, x + width, y + height)
        draw.Draw(sheet, target, picture, picture.Bounds().Min, draw.Src)
        frames[item.sprite][item.frame] = Frame{sheetIndex, x, y, width, height}

        x += width
        rowHeight = max(rowHeight, height)
        usedHeight = max(usedHeight, y + height)
    }
    if err := save(); err != nil {
        return nil, err
    }

    for spriteIndex, sprite := range exporter.sprites {
        manifest.Sprites[sprite.id] = frames[spriteIndex]
    }
    return manifest, nil
}

func run(dataPath string, outPath string) (*Manifest, error) {
    cache := lbx.CacheFromPath(dataPath)
    if cache == nil {
        return nil, fmt.Errorf("no LBX data found in %v", dataPath)
    }

    exporter := &Exporter{cache: cache, seen: make(map[string]bool)}
    tileWidth, tileHeight, err := exporter.collect()
    if err != nil {
        return nil, err
    }

    if err := os.MkdirAll(outPath, 0o755); err != nil {
        return nil, err
    }
    // old sheets from a previous export would linger in the manifest's directory
    oldSheets, _ := filepath.Glob(filepath.Join(outPath, "sheet-*.png"))

    manifest, err := exporter.pack(outPath)
    if err != nil {
        return nil, err
    }
    manifest.TileWidth = tileWidth
    manifest.TileHeight = tileHeight
    manifest.BannerMarker = bannerMarker
    manifest.BannerColors = bannerColors()

    for _, old := range oldSheets {
        if !slices.Contains(manifest.Sheets, filepath.Base(old)) {
            os.Remove(old)
        }
    }

    encoded, err := json.Marshal(manifest)
    if err != nil {
        return nil, err
    }
    return manifest, os.WriteFile(filepath.Join(outPath, ManifestName), encoded, 0o644)
}

func defaultOutPath() string {
    home, err := os.UserHomeDir()
    if err != nil {
        return "assets"
    }
    return filepath.Join(home, ".local", "share", "civ-of-magic", "assets")
}

func main() {
    var dataPath string
    var outPath string
    flag.StringVar(&dataPath, "data", "", "directory (or zip) with the Master of Magic LBX files")
    flag.StringVar(&outPath, "out", defaultOutPath(), "directory to write assets.json and the sheets to")
    flag.Parse()

    if dataPath == "" {
        fmt.Fprintln(os.Stderr, "usage: export-assets -data <Master of Magic directory> [-out <directory>]")
        os.Exit(2)
    }

    log.SetFlags(0)
    manifest, err := run(dataPath, outPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    frames := 0
    for _, sprite := range manifest.Sprites {
        frames += len(sprite)
    }
    fmt.Printf("%v sprites (%v frames) in %v sheets written to %v\n", len(manifest.Sprites), frames, len(manifest.Sheets), outPath)
}
