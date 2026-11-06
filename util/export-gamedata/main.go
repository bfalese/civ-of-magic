// export-gamedata reads the data tables (spells, buildings, artifacts, names) from an original
// Master of Magic install and writes them as JSON (core/gamedata), so the game model can run
// without LBX files, e.g. headless.
//
//   go run ./util/export-gamedata -data "/path/to/Master of Magic" -out ~/.local/share/civ-of-magic/gamedata
//
// The output is derived from copyrighted data: don't commit it.
package main

import (
    "flag"
    "fmt"
    "log"
    "os"

    "github.com/bfalese/civ-of-magic/core/gamedata"
    "github.com/bfalese/civ-of-magic/game/magic/lbxdata"
    "github.com/bfalese/civ-of-magic/lib/lbx"
)

func run(dataPath string, outPath string) error {
    cache := lbx.CacheFromPath(dataPath)
    if cache == nil {
        return fmt.Errorf("no LBX data found in %v", dataPath)
    }

    spells, err := lbxdata.ReadSpellsFromCache(cache)
    if err != nil {
        return fmt.Errorf("spells: %v", err)
    }

    buildings, err := lbxdata.ReadBuildingInfo(cache)
    if err != nil {
        return fmt.Errorf("buildings: %v", err)
    }

    cityNames, err := lbxdata.ReadCityNames(cache)
    if err != nil {
        return fmt.Errorf("city names: %v", err)
    }

    artifacts, err := lbxdata.ReadArtifacts(cache)
    if err != nil {
        return fmt.Errorf("artifacts: %v", err)
    }

    _, powerCosts, powerCompatibilities, err := lbxdata.ReadPowers(cache)
    if err != nil {
        return fmt.Errorf("artifact powers: %v", err)
    }

    gameData := gamedata.GameData{
        Spells: spells,
        Buildings: buildings,
        HeroNames: lbxdata.ReadNamesPerWizard(cache),
        CityNames: cityNames,
        Artifacts: artifacts,
        PowerCosts: powerCosts,
        PowerCompatibilities: powerCompatibilities,
    }

    if err := gameData.Save(outPath); err != nil {
        return err
    }

    log.Printf("wrote %v spells, %v buildings, %v city names, %v artifacts, %v powers to %v", len(spells.Spells), len(buildings), len(cityNames), len(artifacts), len(powerCosts), outPath)
    return nil
}

func main() {
    log.SetFlags(0)

    var dataPath string
    var outPath string
    flag.StringVar(&dataPath, "data", "", "path to the Master of Magic install (directory or zip)")
    flag.StringVar(&outPath, "out", "", "directory to write gamedata.json to (keep it outside the repository)")
    flag.Parse()

    if dataPath == "" || outPath == "" {
        flag.Usage()
        os.Exit(2)
    }

    if err := run(dataPath, outPath); err != nil {
        log.Printf("Error: %v", err)
        os.Exit(1)
    }
}
