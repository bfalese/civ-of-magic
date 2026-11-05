package lbxdata

import (
    "bytes"
    "fmt"
    "log"

    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/lib/lbx"
    "github.com/bfalese/civ-of-magic/lib/set"
    artifactlib "github.com/bfalese/civ-of-magic/core/artifact"
)

func ReadArtifacts(cache *lbx.LbxCache) ([]artifactlib.Artifact, error) {
    itemData, err := cache.GetLbxFile("itemdata.lbx")
    if err != nil {
        return nil, fmt.Errorf("unable to read itemdata.lbx: %v", err)
    }

    spells, err := ReadSpellsFromCache(cache)
    if err != nil {
        // return nil, fmt.Errorf("unable to read spells: %v", err)
        log.Printf("Warning: could not load spells: %v", err)
    }

    reader, err := itemData.GetReader(0)
    if err != nil {
        return nil, fmt.Errorf("unable to read entry 0 in itemdata.lbx: %v", err)
    }

    numEntries, err := lbx.ReadUint16(reader)
    if err != nil {
        return nil, fmt.Errorf("read error: %v", err)
    }

    entrySize, err := lbx.ReadUint16(reader)
    if err != nil {
        return nil, fmt.Errorf("read error: %v", err)
    }
    if entrySize != 56 {
        return nil, fmt.Errorf("unsupported itemdata.lbx")
    }

    var out []artifactlib.Artifact

    slotMap, typeMap, abilityMap := artifactlib.GetItemConversionMaps()

    for i := range numEntries {
        // Name
        name := make([]byte, 30)
        n, err := reader.Read(name)
        if err != nil || n != int(30) {
            return nil, fmt.Errorf("unable to read item name %v: %v", i, err)
        }
        name = bytes.Trim(name, "\x00")

        // Image index to items.lbx
        image, err := lbx.ReadUint16(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }

        // Slot
        slotValue, err := lbx.ReadByte(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }

        slot, exists := slotMap[slotValue]
        if !exists {
            return nil, fmt.Errorf("Invalid slot type %v", slotValue)
        }
        _ = slot

        // Type
        typeValue, err := lbx.ReadByte(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }

        artifactType, exists := typeMap[typeValue]
        if !exists {
            return nil, fmt.Errorf("Invalid artifact type %v", typeValue)
        }

        // Cost
        cost, err := lbx.ReadUint16(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }

        // Modifiers
        var powers []artifactlib.Power
        attack, err := lbx.ReadByte(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }
        if attack != 0 {
            powers = append(powers, artifactlib.Power{Type: artifactlib.PowerTypeAttack, Amount: int(attack), Name: fmt.Sprintf("+%v Attack", attack)})
        }

        toHit, err := lbx.ReadByte(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }
        if toHit != 0 {
            powers = append(powers, artifactlib.Power{Type: artifactlib.PowerTypeToHit, Amount: int(toHit), Name: fmt.Sprintf("+%v To Hit", toHit)})
        }

        defense, err := lbx.ReadByte(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }
        if defense != 0 {
            powers = append(powers, artifactlib.Power{Type: artifactlib.PowerTypeDefense, Amount: int(defense), Name: fmt.Sprintf("+%v Defense", defense)})
        }

        movement, err := lbx.ReadByte(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }
        if movement != 0 {
            powers = append(powers, artifactlib.Power{Type: artifactlib.PowerTypeMovement, Amount: int(movement), Name: fmt.Sprintf("+%v Movement", movement)})
        }

        resistance, err := lbx.ReadByte(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }
        if resistance != 0 {
            powers = append(powers, artifactlib.Power{Type: artifactlib.PowerTypeResistance, Amount: int(resistance), Name: fmt.Sprintf("+%v Resistance", resistance)})
        }

        spellSkill, err := lbx.ReadByte(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }
        if spellSkill != 0 {
            powers = append(powers, artifactlib.Power{Type: artifactlib.PowerTypeSpellSkill, Amount: int(spellSkill), Name: fmt.Sprintf("+%v Spell Skill", spellSkill)})
        }

        spellSave, err := lbx.ReadByte(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }
        if spellSave != 0 {
            powers = append(powers, artifactlib.Power{Type: artifactlib.PowerTypeSpellSave, Amount: int(spellSave), Name: fmt.Sprintf("-%v Spell Save", spellSave)})
        }

        // Spells
        spell, err := lbx.ReadByte(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }
        charges, err := lbx.ReadUint16(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }
        if spell != 0 && charges != 0 {
            useSpell := spells.FindById(int(spell))
            powers = append(powers, artifactlib.Power{
                Type: artifactlib.PowerTypeSpellCharges,
                Amount: int(charges),
                Spell: useSpell,
                SpellCharges: int(charges),
                Name: fmt.Sprintf("%v Charges of %v", charges, useSpell.Name),
            })
        }

        // Abilities
        abilitiesValue, err := lbx.ReadUint32(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }

        for mask, ability := range abilityMap {
            if abilitiesValue&mask != 0 {
                name := ability.Name()
                // note: the "Spell as spell X" is rendered in the view code, not here
                powers = append(powers, artifactlib.Power{Type: artifactlib.PowerTypeAbility1, Amount: 0, Name: name, Ability: ability})
            }
        }

        // Requirements
        var requirements []artifactlib.Requirement
        natureRanksNeeded, err := lbx.ReadByte(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }
        if natureRanksNeeded != 0 {
            requirements = append(requirements, artifactlib.Requirement{MagicType: data.NatureMagic, Amount: int(natureRanksNeeded)})
        }

        sorceryRanksNeeded, err := lbx.ReadByte(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }
        if sorceryRanksNeeded != 0 {
            requirements = append(requirements, artifactlib.Requirement{MagicType: data.SorceryMagic, Amount: int(sorceryRanksNeeded)})
        }

        chaosRanksNeeded, err := lbx.ReadByte(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }
        if chaosRanksNeeded != 0 {
            requirements = append(requirements, artifactlib.Requirement{MagicType: data.ChaosMagic, Amount: int(chaosRanksNeeded)})
        }

        lifeRanksNeeded, err := lbx.ReadByte(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }
        if lifeRanksNeeded != 0 {
            requirements = append(requirements, artifactlib.Requirement{MagicType: data.LifeMagic, Amount: int(lifeRanksNeeded)})
        }

        deathRanksNeeded, err := lbx.ReadByte(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }
        if deathRanksNeeded != 0 {
            requirements = append(requirements, artifactlib.Requirement{MagicType: data.DeathMagic, Amount: int(deathRanksNeeded)})
        }

        // The last byte seems to be some sort of flag
        _, err = lbx.ReadByte(reader)
        if err != nil {
            return nil, fmt.Errorf("read error: %v", err)
        }

        artifact := artifactlib.Artifact{
            Name: string(name),
            Image: int(image),
            Cost: int(cost),
            Type: artifactType,
            Powers: powers,
            Requirements: requirements,
        }
        out = append(out, artifact)
    }

    return out, nil
}

func ReadPowers(cache *lbx.LbxCache) ([]artifactlib.Power, map[artifactlib.Power]int, map[artifactlib.Power]set.Set[artifactlib.ArtifactType], error) {
    itemData, err := cache.GetLbxFile("itempow.lbx")
    if err != nil {
        return nil, nil, nil, fmt.Errorf("unable to read itempow.lbx: %v", err)
    }

    reader, err := itemData.GetReader(0)
    if err != nil {
        return nil, nil, nil, fmt.Errorf("unable to read entry 0 in itempow.lbx: %v", err)
    }

    numEntries, err := lbx.ReadUint16(reader)
    if err != nil {
        return nil, nil, nil, fmt.Errorf("read error: %v", err)
    }

    entrySize, err := lbx.ReadUint16(reader)
    if err != nil {
        return nil, nil, nil, fmt.Errorf("read error: %v", err)
    }
    if entrySize != 30 {
        return nil, nil, nil, fmt.Errorf("unsupported itempow.lbx")
    }

    artifactTypeMap := map[uint16]artifactlib.ArtifactType{
        1 << 0: artifactlib.ArtifactTypeSword,
        1 << 1: artifactlib.ArtifactTypeMace,
        1 << 2: artifactlib.ArtifactTypeAxe,
        1 << 3: artifactlib.ArtifactTypeBow,
        1 << 4: artifactlib.ArtifactTypeStaff,
        1 << 5: artifactlib.ArtifactTypeWand,
        1 << 6: artifactlib.ArtifactTypeMisc,
        1 << 7: artifactlib.ArtifactTypeShield,
        1 << 8: artifactlib.ArtifactTypeChain,
        1 << 9: artifactlib.ArtifactTypePlate,
    }

    powerTypeMap := map[byte]artifactlib.PowerType{
        0: artifactlib.PowerTypeAttack,
        1: artifactlib.PowerTypeToHit,
        2: artifactlib.PowerTypeDefense,
        3: artifactlib.PowerTypeSpellSkill,
        4: artifactlib.PowerTypeSpellSave,
        5: artifactlib.PowerTypeMovement,
        6: artifactlib.PowerTypeResistance,
        7: artifactlib.PowerTypeAbility1,
        8: artifactlib.PowerTypeAbility2,
        9: artifactlib.PowerTypeAbility3,
    }

    magicTypeMap := map[byte]data.MagicType{
        0: data.NatureMagic,
        1: data.SorceryMagic,
        2: data.ChaosMagic,
        3: data.LifeMagic,
        4: data.DeathMagic,
    }

    // Reuse the canonical conversion maps so artifact parsing doesn't drift
    // from item loading (both read the same itempow.lbx / itemdata.lbx).
    _, _, abilityMap := artifactlib.GetItemConversionMaps()

    var powers []artifactlib.Power
    costs := make(map[artifactlib.Power]int)
    compatibilities := make(map[artifactlib.Power]set.Set[artifactlib.ArtifactType])

    for i := 0; i < int(numEntries); i++ {
        // Name
        name := make([]byte, 18)
        n, err := reader.Read(name)
        if err != nil || n != int(18) {
            return nil, nil, nil, fmt.Errorf("unable to read item name %v: %v", i, err)
        }
        name = bytes.Trim(name, " \x00")

        // Artifact types
        artifactTypeValue, err := lbx.ReadUint16(reader)
        if err != nil {
            return nil, nil, nil, fmt.Errorf("read error: %v", err)
        }
        if artifactTypeValue == 0 {
            continue  // there seems to be empty entries
        }

        artifactTypes := set.MakeSet[artifactlib.ArtifactType]()
        for mask, artifactType := range artifactTypeMap {
            if artifactTypeValue & mask != 0 {
                artifactTypes.Insert(artifactType)
            }
        }

        // Cost
        cost, err := lbx.ReadUint16(reader)
        if err != nil {
            return nil, nil, nil, fmt.Errorf("read error: %v", err)
        }

        // Power type
        powerTypeValue, err := lbx.ReadByte(reader)
        if err != nil {
            return nil, nil, nil, fmt.Errorf("read error: %v", err)
        }
        powerType, exists := powerTypeMap[powerTypeValue]
        if !exists {
            return nil, nil, nil, fmt.Errorf("Invalid power type %v", powerTypeValue)
        }

        // Magic type
        magicTypeValue, err := lbx.ReadByte(reader)
        if err != nil {
            return nil, nil, nil, fmt.Errorf("read error: %v", err)
        }
        magicType, exists := magicTypeMap[magicTypeValue]
        if !exists {
            return nil, nil, nil, fmt.Errorf("Invalid magic type %v", magicTypeValue)
        }

        // Amount
        amount, err := lbx.ReadUint16(reader)
        if err != nil {
            return nil, nil, nil, fmt.Errorf("read error: %v", err)
        }
        if string(name) == "+6 Defense" {
            amount = 6
        }

        // Abilities
        abilitiesValue, err := lbx.ReadUint32(reader)
        if err != nil {
            return nil, nil, nil, fmt.Errorf("read error: %v", err)
        }

        var ability data.ItemAbility
        for mask, current := range abilityMap {
            if abilitiesValue&mask != 0 {
                ability = current
            }
        }

        // Create power
        if amount == 0 {
            continue // Spell Charges
        }

        if powerType != artifactlib.PowerTypeNone {
            power := artifactlib.Power{
                Type: powerType,
                Name: string(name),
                Amount: int(amount),
                Ability: ability,
                Magic: magicType,
                Index: i,
            }
            powers = append(powers, power)
            costs[power] = int(cost)
            compatibilities[power] = *artifactTypes
        }
    }
    return powers, costs, compatibilities, nil
}
