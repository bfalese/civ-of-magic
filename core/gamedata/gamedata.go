// Package gamedata holds the game's data tables (spells, buildings, artifacts, names) in a plain
// JSON form, so the game model can be created without the LBX files.
//
// The tables are exported once from an original install with util/export-gamedata. They are
// derived from copyrighted data: keep the exported files out of the repository.
package gamedata

import (
    "encoding/json"
    "fmt"
    "image"
    "os"
    "path/filepath"

    "github.com/bfalese/civ-of-magic/core/artifact"
    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/core/terrain"
    "github.com/bfalese/civ-of-magic/lib/set"
)

// the file written by Save and read by Load, inside the data directory
const FileName = "gamedata.json"

type GameData struct {
    Spells spellbook.Spells
    Buildings buildinglib.BuildingInfos
    // wizard index -> hero type -> name
    HeroNames map[int]map[herolib.HeroType]string
    // city names of the 14 races, one list after the other (see GameModel.SuggestCityName)
    CityNames []string
    // the premade artifacts
    Artifacts []artifact.Artifact
    // artifact power table: cost and which item types can have each power
    PowerCosts map[artifact.Power]int
    PowerCompatibilities map[artifact.Power]set.Set[artifact.ArtifactType]
}

// all of a spell's fields (spellbook.Spell marshals as its name only, which save games rely on)
type serializedSpell spellbook.Spell

type serializedPowerEntry struct {
    Power artifact.SerializedPower `json:"power"`
    Cost int `json:"cost"`
    Compatible []artifact.ArtifactType `json:"compatible"`
}

type serializedGameData struct {
    Version int `json:"version"`
    Spells []serializedSpell `json:"spells"`
    Buildings []buildinglib.BuildingInfo `json:"buildings"`
    HeroNames map[int]map[herolib.HeroType]string `json:"hero-names"`
    CityNames []string `json:"city-names"`
    Artifacts []artifact.SerializedArtifact `json:"artifacts"`
    Powers []serializedPowerEntry `json:"powers"`
}

const currentVersion = 1

// the artifact power table in the form treasure creation wants it
func (gameData *GameData) ArtifactPowers() (map[artifact.Power]int, map[artifact.Power]set.Set[artifact.ArtifactType], error) {
    return gameData.PowerCosts, gameData.PowerCompatibilities, nil
}

func (gameData *GameData) ArtifactCatalog() *artifact.Catalog {
    return artifact.MakeCatalog(gameData.Artifacts)
}

// terrain tiles with their placement rules and no images; enough to generate and play maps
// without a frontend. The tile rules are built into the terrain package.
func TerrainData() *terrain.TerrainData {
    var tiles []terrain.TerrainTile
    for index := range terrain.MyrrorStart * 2 {
        tiles = append(tiles, terrain.TerrainTile{
            ImageIndex: index,
            TileIndex: index,
            Tile: terrain.GetTile(index),
        })
    }

    return terrain.MakeTerrainData([]image.Image{}, tiles)
}

func (gameData *GameData) serialize() serializedGameData {
    out := serializedGameData{
        Version: currentVersion,
        Buildings: gameData.Buildings,
        HeroNames: gameData.HeroNames,
        CityNames: gameData.CityNames,
    }

    for _, spell := range gameData.Spells.Spells {
        out.Spells = append(out.Spells, serializedSpell(spell))
    }

    for index := range gameData.Artifacts {
        out.Artifacts = append(out.Artifacts, artifact.SerializeArtifact(&gameData.Artifacts[index]))
    }

    for power, cost := range gameData.PowerCosts {
        entry := serializedPowerEntry{
            Power: artifact.SerializePowers([]artifact.Power{power})[0],
            Cost: cost,
        }
        if compatible, ok := gameData.PowerCompatibilities[power]; ok {
            entry.Compatible = compatible.Values()
        }
        out.Powers = append(out.Powers, entry)
    }

    return out
}

func deserialize(serialized serializedGameData) (*GameData, error) {
    if serialized.Version != currentVersion {
        return nil, fmt.Errorf("unsupported game data version %v (expected %v)", serialized.Version, currentVersion)
    }

    var spellList []spellbook.Spell
    for _, spell := range serialized.Spells {
        spellList = append(spellList, spellbook.Spell(spell))
    }
    spells := spellbook.SpellsFromArray(spellList)

    out := &GameData{
        Spells: spells,
        Buildings: serialized.Buildings,
        HeroNames: serialized.HeroNames,
        CityNames: serialized.CityNames,
        PowerCosts: make(map[artifact.Power]int),
        PowerCompatibilities: make(map[artifact.Power]set.Set[artifact.ArtifactType]),
    }

    for index := range serialized.Artifacts {
        out.Artifacts = append(out.Artifacts, *artifact.ReconstructArtifact(&serialized.Artifacts[index], spells))
    }

    for _, entry := range serialized.Powers {
        power := artifact.ReconstructPowers([]artifact.SerializedPower{entry.Power}, spells)[0]
        out.PowerCosts[power] = entry.Cost
        out.PowerCompatibilities[power] = *set.NewSet(entry.Compatible...)
    }

    return out, nil
}

// write the data to directory/gamedata.json
func (gameData *GameData) Save(directory string) error {
    if err := os.MkdirAll(directory, 0o755); err != nil {
        return err
    }

    encoded, err := json.MarshalIndent(gameData.serialize(), "", "  ")
    if err != nil {
        return err
    }

    return os.WriteFile(filepath.Join(directory, FileName), encoded, 0o644)
}

// read directory/gamedata.json
func Load(directory string) (*GameData, error) {
    encoded, err := os.ReadFile(filepath.Join(directory, FileName))
    if err != nil {
        return nil, err
    }

    var serialized serializedGameData
    if err := json.Unmarshal(encoded, &serialized); err != nil {
        return nil, fmt.Errorf("unable to parse %v: %v", FileName, err)
    }

    return deserialize(serialized)
}
