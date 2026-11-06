package gamedata

import (
    "testing"

    "github.com/bfalese/civ-of-magic/core/artifact"
    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    "github.com/bfalese/civ-of-magic/core/data"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/lib/set"
)

func TestSaveLoad(test *testing.T) {
    fireball := spellbook.Spell{Name: "Fireball", Magic: data.ChaosMagic, CastCost: 15, ResearchCost: 160}
    power := artifact.Power{Type: artifact.PowerTypeAttack, Amount: 1, Name: "+1 Attack"}
    original := GameData{
        Spells: spellbook.SpellsFromArray([]spellbook.Spell{fireball}),
        Buildings: buildinglib.BuildingInfos{{Name: "Barracks", ConstructionCost: 30}},
        HeroNames: map[int]map[herolib.HeroType]string{0: {herolib.HeroRakir: "Rakir"}},
        CityNames: []string{"Alpha", "Beta"},
        Artifacts: []artifact.Artifact{{Name: "Sword", Cost: 100, Powers: []artifact.Power{power}}},
        PowerCosts: map[artifact.Power]int{power: 50},
        PowerCompatibilities: map[artifact.Power]set.Set[artifact.ArtifactType]{power: *set.NewSet(artifact.ArtifactTypeSword)},
    }

    directory := test.TempDir()
    if err := original.Save(directory); err != nil {
        test.Fatalf("save: %v", err)
    }
    loaded, err := Load(directory)
    if err != nil {
        test.Fatalf("load: %v", err)
    }

    if loaded.Spells.FindByName("Fireball").ResearchCost != 160 {
        test.Errorf("spell fields should survive, got %+v", loaded.Spells.FindByName("Fireball"))
    }
    if loaded.Buildings[0].ConstructionCost != 30 || loaded.HeroNames[0][herolib.HeroRakir] != "Rakir" || len(loaded.CityNames) != 2 {
        test.Errorf("tables not restored: %+v", loaded)
    }
    if len(loaded.Artifacts) != 1 || loaded.Artifacts[0].Powers[0].Name != "+1 Attack" {
        test.Errorf("artifacts not restored: %+v", loaded.Artifacts)
    }
    compatible := loaded.PowerCompatibilities[power]
    if loaded.PowerCosts[power] != 50 || !compatible.Contains(artifact.ArtifactTypeSword) {
        test.Errorf("power table not restored")
    }
}

func TestTerrainData(test *testing.T) {
    terrainData := TerrainData()
    if len(terrainData.Tiles) == 0 {
        test.Fatalf("no tiles")
    }
}
