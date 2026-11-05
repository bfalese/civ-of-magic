package lbxdata

import (
    "bytes"
    "fmt"
    "log"

    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    "github.com/bfalese/civ-of-magic/lib/lbx"
)

func parseName(name []byte) string {
    for i, b := range name {
        if b == 0 {
            return string(name[:i])
        }
    }
    return string(name)
}

func ReadBuildingInfo(cache *lbx.LbxCache) (buildinglib.BuildingInfos, error) {
    data, err := cache.GetLbxFile("builddat.lbx")
    if err != nil {
        return nil, fmt.Errorf("Unable to read builddat.lbx: %v", err)
    }

    reader, err := data.GetReader(0)
    if err != nil {
        return nil, fmt.Errorf("unable to read entry 0 in builddat.lbx: %v", err)
    }

    numBuildings, err := lbx.ReadUint16(reader)
    if err != nil {
        return nil, fmt.Errorf("read error: %v", err)
    }

    entrySize, err := lbx.ReadUint16(reader)
    if err != nil {
        return nil, fmt.Errorf("read error: %v", err)
    }

    var out buildinglib.BuildingInfos

    for i := 0; i < int(numBuildings); i++ {
        buildingData := make([]byte, entrySize)
        n, err := reader.Read(buildingData)
        if err != nil || n != int(entrySize) {
            return nil, fmt.Errorf("unable to read building info %v: %v", i, err)
        }

        buildingReader := bytes.NewReader(buildingData)
        name := make([]byte, 20)
        _, err = buildingReader.Read(name)
        if err != nil {
            return nil, fmt.Errorf("unable to read building name %v: %v", i, err)
        }

        dependency1, err := lbx.ReadUint16(buildingReader)
        if err != nil {
            return nil, fmt.Errorf("unable to read building dependency1 %v: %v", i, err)
        }

        dependency2, err := lbx.ReadUint16(buildingReader)
        if err != nil {
            return nil, fmt.Errorf("unable to read building dependency2 %v: %v", i, err)
        }

        replace, err := lbx.ReadUint16(buildingReader)
        if err != nil {
            return nil, fmt.Errorf("unable to read building replace %v: %v", i, err)
        }

        if replace == 65535 {
            replace = 0
        }

        grant20xp, err := lbx.ReadUint16(buildingReader)
        if err != nil {
            return nil, fmt.Errorf("unable to read building grant20xp %v: %v", i, err)
        }

        grant60xp, err := lbx.ReadUint16(buildingReader)
        if err != nil {
            return nil, fmt.Errorf("unable to read building grant60xp %v: %v", i, err)
        }

        alchemist, err := lbx.ReadUint16(buildingReader)
        if err != nil {
            return nil, fmt.Errorf("unable to read building alchemist %v: %v", i, err)
        }

        upkeepGold, err := lbx.ReadUint16(buildingReader)
        if err != nil {
            return nil, fmt.Errorf("unable to read building upkeepGold %v: %v", i, err)
        }

        populationGrowth, err := lbx.ReadUint16(buildingReader)
        if err != nil {
            return nil, fmt.Errorf("unable to read building populationGrowth %v: %v", i, err)
        }

        unknown1, err := lbx.ReadUint16(buildingReader)
        if err != nil {
            return nil, fmt.Errorf("unable to read building unknown1 %v: %v", i, err)
        }
        _ = unknown1

        unknown2, err := lbx.ReadUint16(buildingReader)
        if err != nil {
            return nil, fmt.Errorf("unable to read building unknown2 %v: %v", i, err)
        }
        _ = unknown2

        religion, err := lbx.ReadUint16(buildingReader)
        if err != nil {
            return nil, fmt.Errorf("unable to read building religion %v: %v", i, err)
        }

        research, err := lbx.ReadUint16(buildingReader)
        if err != nil {
            return nil, fmt.Errorf("unable to read building research %v: %v", i, err)
        }

        constructionCost, err := lbx.ReadUint16(buildingReader)
        if err != nil {
            return nil, fmt.Errorf("unable to read building constructionCost %v: %v", i, err)
        }

        unknown3, err := lbx.ReadUint16(buildingReader)
        if err != nil {
            return nil, fmt.Errorf("unable to read building unknown3 %v: %v", i, err)
        }
        _ = unknown3

        animation, err := lbx.ReadUint16(buildingReader)
        if err != nil {
            return nil, fmt.Errorf("unable to read building animation %v: %v", i, err)
        }

        category, err := lbx.ReadUint16(buildingReader)
        if err != nil {
            return nil, fmt.Errorf("unable to read building category %v: %v", i, err)
        }

        terrainDependency := uint16(0)
        if dependency1 > 100 {
            terrainDependency = dependency1
            dependency1 = 0
        }

        info := buildinglib.BuildingInfo{
            Name: parseName(name),
            BuildingDependency1: int(dependency1),
            BuildingDependency2: int(int16(dependency2)),
            TerrainDependency: int(int16(terrainDependency)),
            BuildingReplace: int(replace),
            Grant20XP: grant20xp == 1,
            Grant60XP: grant60xp == 1,
            Alchemist: alchemist == 1,
            UpkeepGold: -int(int16(upkeepGold)),
            PopulationGrowth: int(populationGrowth),
            Religion: int(religion),
            Research: int(research),
            ConstructionCost: int(constructionCost),
            Animation: int(int16(animation)),
            Category: int(category),
        }

        // fmt.Printf("Building %v: %+v\n", i, info)
        out = append(out, info)
    }

    // builddat has research as 7, but its supposed to be 5
    university := out.GetBuildingByName("University")
    if university != nil {
        university.Research = 5
    }

    // FIXME: does this value live in builddat.lbx?
    wizardsGuild := out.GetBuildingByName("Wizards' Guild")
    if wizardsGuild != nil {
        wizardsGuild.UpkeepPower = 3
    }

    oracle := out.GetBuildingByName("Oracle")
    if oracle != nil {
        // oracle should not provide research
        oracle.Research = 0
    }

    none := out.GetBuildingByName("None")
    if none != nil {
        none.Name = ""
    }

    return out, nil
}

func MakeBuildDescriptions(cache *lbx.LbxCache) *buildinglib.BuildingDescriptions {
    buildDescriptionLbx, err := cache.GetLbxFile("buildesc.lbx")
    if err == nil {
    } else {
        log.Printf("Unable to read building descriptions")
        return nil
    }

    descriptions := readBuildDescriptions(buildDescriptionLbx)

    return &buildinglib.BuildingDescriptions{
        Descriptions: descriptions,
    }
}

func readBuildDescriptions(buildDescriptionLbx *lbx.LbxFile) []string {
    entries, err := buildDescriptionLbx.RawData(0)
    if err != nil {
        return nil
    }

    reader := bytes.NewReader(entries)

    count, err := lbx.ReadUint16(reader)
    if err != nil {
        return nil
    }

    if count > 10000 {
        return nil
    }

    size, err := lbx.ReadUint16(reader)
    if err != nil {
        return nil
    }

    if size > 10000 {
        return nil
    }

    var descriptions []string

    for i := 0; i < int(count); i++ {
        data := make([]byte, size)
        _, err := reader.Read(data)

        if err != nil {
            break
        }

        nullByte := bytes.IndexByte(data, 0)
        if nullByte != -1 {
            descriptions = append(descriptions, string(data[0:nullByte]))
        } else {
            descriptions = append(descriptions, string(data))
        }
    }

    return descriptions
}
