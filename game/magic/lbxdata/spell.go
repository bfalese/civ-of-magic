package lbxdata

import (
    "bytes"
    "fmt"

    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/lib/lbx"
)

func ReadSpellsFromCache(cache *lbx.LbxCache) (spellbook.Spells, error) {
    file, err := cache.GetLbxFile("spelldat.lbx")
    if err != nil {
        return spellbook.Spells{}, err
    }
    return ReadSpells(file, 0)
}

// pass in spelldat.lbx and 0
func ReadSpells(lbxFile *lbx.LbxFile, entry int) (spellbook.Spells, error) {
    if entry < 0 || entry >= len(lbxFile.Data) {
        return spellbook.Spells{}, fmt.Errorf("invalid lbx index %v, must be between 0 and %v", entry, len(lbxFile.Data) - 1)
    }

    reader := bytes.NewReader(lbxFile.Data[entry])

    numEntries, err := lbx.ReadUint16(reader)
    if err != nil {
        return spellbook.Spells{}, err
    }

    entrySize, err := lbx.ReadUint16(reader)
    if err != nil {
        return spellbook.Spells{}, err
    }

    var spells spellbook.Spells

    type MagicData struct {
        Magic data.MagicType
        Rarity spellbook.SpellRarity
    }

    // FIXME: turn this into a go 1.23 iterator
    spellMagicIterator := (func() chan MagicData {
        out := make(chan MagicData)

        go func() {
            defer close(out)

            out <- MagicData{Magic: data.MagicNone}
            order := []data.MagicType{data.NatureMagic, data.SorceryMagic, data.ChaosMagic, data.LifeMagic, data.DeathMagic}
            rarities := []spellbook.SpellRarity{spellbook.SpellRarityCommon, spellbook.SpellRarityUncommon, spellbook.SpellRarityRare, spellbook.SpellRarityVeryRare}

            for _, magic := range order {
                // 10 types of common, uncommon, rare, very rare for each book of magic
                for _, rarity := range rarities {
                    for i := 0; i < 10; i++ {
                        out <- MagicData{Magic: magic, Rarity: rarity}
                    }
                }
            }

            // for arcane the spells are
            // common: magic spirit, dispel magic, spell of return, summoning circle
            // uncommon: detect magic, recall hero, disenchant area, enchant item, summon hero
            // rare: awareness, disjunction, create artifact, summon champion
            // very rare: spell of mastery

            for i := 0; i < 4; i++ {
                out <- MagicData{Magic: data.ArcaneMagic, Rarity: spellbook.SpellRarityCommon}
            }

            for i := 0; i < 5; i++ {
                out <- MagicData{Magic: data.ArcaneMagic, Rarity: spellbook.SpellRarityUncommon}
            }

            for i := 0; i < 4; i++ {
                out <- MagicData{Magic: data.ArcaneMagic, Rarity: spellbook.SpellRarityRare}
            }

            out <- MagicData{Magic: data.ArcaneMagic, Rarity: spellbook.SpellRarityVeryRare}
        }()

        return out
    })()

    for i := 0; i < int(numEntries); i++ {
        data := make([]byte, entrySize)
        n, err := reader.Read(data)
        if err != nil {
            return spellbook.Spells{}, fmt.Errorf("Error reading help index %v: %v", i, err)
        }

        buffer := bytes.NewBuffer(data[0:n])

        nameData := buffer.Next(19)
        // fmt.Printf("Spell %v\n", i)

        name, err := bytes.NewBuffer(nameData).ReadString(0)
        if err != nil {
            name = string(nameData)
        } else {
            name = name[0:len(name)-1]
        }
        // fmt.Printf("  Name: %v\n", string(name))

        aiGroup, err := buffer.ReadByte()
        if err != nil {
            return spellbook.Spells{}, err
        }
        // fmt.Printf("  AI Group: %v\n", aiGroup)

        aiValue, err := buffer.ReadByte()
        if err != nil {
            return spellbook.Spells{}, err
        }
        // fmt.Printf("  AI Value: %v\n", aiValue)

        spellType, err := buffer.ReadByte()
        if err != nil {
            return spellbook.Spells{}, err
        }

        // fmt.Printf("  Spell Type: %v\n", spellType)

        section, err := buffer.ReadByte()
        if err != nil {
            return spellbook.Spells{}, err
        }

        // fmt.Printf("  Section: %v\n", section)

        realm, err := buffer.ReadByte()
        if err != nil {
            return spellbook.Spells{}, err
        }

        // fmt.Printf("  Magic Realm: %v\n", realm)

        eligibility, err := buffer.ReadByte()
        if err != nil {
            return spellbook.Spells{}, err
        }

        // fmt.Printf("  Caster Eligibility: %v\n", eligibility)

        buffer.Next(1) // ignore extra unused byte from 2-byte alignment

        castCost, err := lbx.ReadUint16(buffer)
        if err != nil {
            return spellbook.Spells{}, err
        }

        // fmt.Printf("  Casting Cost: %v\n", castCost)

        researchCost, err := lbx.ReadUint16(buffer)
        if err != nil {
            return spellbook.Spells{}, err
        }

        // fmt.Printf("  Research Cost: %v\n", researchCost)

        sound, err := buffer.ReadByte()
        if err != nil {
            return spellbook.Spells{}, err
        }

        // fmt.Printf("  Sound effect: %v\n", sound)

        // skip extra byte due to 2-byte alignment
        buffer.ReadByte()

        summoned, err := buffer.ReadByte()
        if err != nil {
            return spellbook.Spells{}, err
        }

        // fmt.Printf("  Summoned: %v\n", summoned)

        flag1, err := buffer.ReadByte()
        if err != nil {
            return spellbook.Spells{}, err
        }

        flag2, err := buffer.ReadByte()
        if err != nil {
            return spellbook.Spells{}, err
        }

        // FIXME: should this be a uint16?
        flag3, err := buffer.ReadByte()
        if err != nil {
            return spellbook.Spells{}, err
        }

        // fmt.Printf("  Flag1=%v Flag2=%v Flag3=%v\n", flag1, flag2, flag3)

        magicData := <-spellMagicIterator

        spells.AddSpell(spellbook.Spell{
            Name: name,
            Index: i,
            AiGroup: int(aiGroup),
            AiValue: int(aiValue),
            SpellType: int(spellType),
            Section: spellbook.Section(section),
            Realm: int(realm),
            Eligibility: spellbook.EligibilityType(eligibility),
            CastCost: int(castCost),
            ResearchCost: int(researchCost),
            Sound: int(sound),
            Summoned: int(summoned),
            Flag1: int(flag1),
            Flag2: int(flag2),
            Flag3: int(flag3),
            Magic: magicData.Magic,
            Rarity: magicData.Rarity,
        })
    }

    return spells, nil
}

func ReadSpellDescriptionsFromCache(cache *lbx.LbxCache) ([]string, error) {
    file, err := cache.GetLbxFile("desc.lbx")
    if err != nil {
        return nil, err
    }
    return ReadSpellDescriptions(file)
}

// pass in desc.lbx
func ReadSpellDescriptions(file *lbx.LbxFile) ([]string, error) {
    entries, err := file.RawData(0)
    if err != nil {
        return nil, err
    }

    reader := bytes.NewReader(entries)

    count, err := lbx.ReadUint16(reader)
    if err != nil {
        return nil, err
    }

    if count > 10000 {
        return nil, fmt.Errorf("Spell count was too high: %v", count)
    }

    size, err := lbx.ReadUint16(reader)
    if err != nil {
        return nil, err
    }

    if size > 10000 {
        return nil, fmt.Errorf("Size of each spell entry was too high: %v", size)
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

    return descriptions, nil
}
