package main

import (
    "fmt"
    "os"
    "reflect"
    "testing"

    "github.com/bfalese/civ-of-magic/core/gamedata"
    "github.com/bfalese/civ-of-magic/game/magic/lbxdata"
    "github.com/bfalese/civ-of-magic/lib/lbx"
)

// exporting and loading again gives the same tables as reading the LBX files.
// needs MOM_DATA=/path/to/Master of Magic, skipped otherwise (CI has no data)
func TestRoundTrip(test *testing.T) {
    dataPath := os.Getenv("MOM_DATA")
    if dataPath == "" {
        test.Skip("MOM_DATA not set")
    }

    out := test.TempDir()
    if err := run(dataPath, out); err != nil {
        test.Fatalf("export: %v", err)
    }

    loaded, err := gamedata.Load(out)
    if err != nil {
        test.Fatalf("load: %v", err)
    }

    cache := lbx.CacheFromPath(dataPath)
    spells, _ := lbxdata.ReadSpellsFromCache(cache)
    buildings, _ := lbxdata.ReadBuildingInfo(cache)
    artifacts, _ := lbxdata.ReadArtifacts(cache)
    _, costs, compatibilities, _ := lbxdata.ReadPowers(cache)

    if !reflect.DeepEqual(spells.Spells, loaded.Spells.Spells) {
        test.Errorf("spells differ")
    }
    if description := loaded.SpellDescriptions["Fireball"]; description == "" {
        test.Errorf("Fireball has no description")
    }
    if len(loaded.SpellDescriptions) < len(spells.Spells) * 9 / 10 {
        test.Errorf("only %v descriptions for %v spells", len(loaded.SpellDescriptions), len(spells.Spells))
    }
    if !reflect.DeepEqual(buildings, loaded.Buildings) {
        test.Errorf("buildings differ")
    }
    if !reflect.DeepEqual(lbxdata.ReadNamesPerWizard(cache), loaded.HeroNames) {
        test.Errorf("hero names differ")
    }
    if len(artifacts) != len(loaded.Artifacts) {
        test.Fatalf("artifact count differs: %v vs %v", len(artifacts), len(loaded.Artifacts))
    }
    // compared printed: serializing turns nil requirement lists into empty ones
    for index := range artifacts {
        if fmt.Sprintf("%+v", artifacts[index]) != fmt.Sprintf("%+v", loaded.Artifacts[index]) {
            test.Errorf("artifact %v (%v) differs:\n%+v\n%+v", index, artifacts[index].Name, artifacts[index], loaded.Artifacts[index])
            break
        }
    }
    if !reflect.DeepEqual(costs, loaded.PowerCosts) {
        test.Errorf("power costs differ")
    }
    for power, compatible := range compatibilities {
        loadedCompatible := loaded.PowerCompatibilities[power]
        if len(compatible.Values()) != len(loadedCompatible.Values()) {
            test.Errorf("compatibilities of %v differ", power.Name)
        }
    }
}
