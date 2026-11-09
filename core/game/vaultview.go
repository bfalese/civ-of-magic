package game

import (
    "github.com/bfalese/civ-of-magic/core/artifact"
    "github.com/bfalese/civ-of-magic/core/assets"
    "github.com/bfalese/civ-of-magic/core/data"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/units"
)

// The vault and hero equipment in the player view, for a frontend's items screen: the slots
// move-item and destroy-item name ({vault: i}, {hero: i, slot: j}, {new: i}) with what is in them.

type ItemView struct {
    Name string `json:"name"`
    // Sword, Mace, ..., Misc
    Type string `json:"type"`
    // the picture (core/assets Item)
    Asset string `json:"asset"`
    Cost int `json:"cost"`
    // mana destroy-item gives for it
    Value int `json:"value"`
    Powers []string `json:"powers"`
}

type HeroSlotView struct {
    // melee-weapon, ranged-weapon, magic-weapon, any-weapon, armor, jewelry
    Kind string `json:"kind"`
    // the item types the slot takes
    Takes []string `json:"takes"`
    Item *ItemView `json:"item"`
}

type HeroView struct {
    // the hero index move-item's {hero: i} takes, and the unit id (stacks, disband-unit)
    Index int `json:"index"`
    Unit int `json:"unit"`
    Name string `json:"name"`
    Title string `json:"title"`
    Level string `json:"level"`
    Location Location `json:"location"`
    Asset string `json:"asset"`
    Slots []HeroSlotView `json:"slots"`
}

// the vault, its newly arrived items and the heroes' equipment
type VaultView struct {
    // the four vault slots; null when empty
    Vault []*ItemView `json:"vault"`
    // items that just arrived (Create Artifact, merchants, treasure, dead heroes): place them in
    // an empty slot or destroy them
    New []ItemView `json:"new"`
    Heroes []HeroView `json:"heroes"`
    // mana for an item that arrives at a hero away from where it was (the fortress for the vault)
    TeleportCost int `json:"teleport-cost"`
    // where the vault is: the fortress
    Fortress *Location `json:"fortress,omitempty"`
}

func artifactSlotName(slot artifact.ArtifactSlot) string {
    switch slot {
        case artifact.ArtifactSlotMeleeWeapon: return "melee-weapon"
        case artifact.ArtifactSlotRangedWeapon: return "ranged-weapon"
        case artifact.ArtifactSlotMagicWeapon: return "magic-weapon"
        case artifact.ArtifactSlotAnyWeapon: return "any-weapon"
        case artifact.ArtifactSlotArmor: return "armor"
        case artifact.ArtifactSlotJewelry: return "jewelry"
    }
    return "unknown"
}

func makeItemView(player *playerlib.Player, item *artifact.Artifact) *ItemView {
    if item == nil {
        return nil
    }
    view := &ItemView{
        Name: item.Name,
        Type: item.Type.Name(),
        Asset: assets.Item(item.Image),
        Cost: item.Cost,
        Value: player.ArtifactManaValue(item),
        Powers: []string{},
    }
    for _, power := range item.Powers {
        view.Powers = append(view.Powers, power.Name)
    }
    return view
}

func (model *GameModel) makeVaultView(player *playerlib.Player) VaultView {
    view := VaultView{
        Vault: []*ItemView{},
        New: []ItemView{},
        Heroes: []HeroView{},
        TeleportCost: playerlib.VaultTeleportCost,
    }
    for _, item := range player.VaultEquipment {
        view.Vault = append(view.Vault, makeItemView(player, item))
    }
    for _, item := range model.PendingItems[player] {
        if item != nil {
            view.New = append(view.New, *makeItemView(player, item))
        }
    }
    if fortress := player.FindFortressCity(); fortress != nil {
        view.Fortress = &Location{Plane: fortress.Plane, X: fortress.X, Y: fortress.Y}
    }

    for index, hero := range player.Heroes {
        if hero == nil {
            continue
        }
        point := hero.GetPlanePoint()
        heroView := HeroView{
            Index: index,
            Unit: model.UnitId(hero),
            Name: hero.GetName(),
            Title: hero.GetTitle(),
            Level: heroLevelName(hero),
            Location: Location{Plane: point.Plane, X: point.X, Y: point.Y},
            Asset: unitAsset(hero),
            Slots: []HeroSlotView{},
        }
        for slotIndex, slot := range hero.GetArtifactSlots() {
            slotView := HeroSlotView{Kind: artifactSlotName(slot), Takes: []string{}}
            for kind := artifact.ArtifactTypeSword; kind <= artifact.ArtifactTypePlate; kind++ {
                if slot.CompatibleWith(kind) {
                    slotView.Takes = append(slotView.Takes, kind.Name())
                }
            }
            if slotIndex < len(hero.Equipment) {
                slotView.Item = makeItemView(player, hero.Equipment[slotIndex])
            }
            heroView.Slots = append(heroView.Slots, slotView)
        }
        view.Heroes = append(view.Heroes, heroView)
    }
    return view
}

// a unit's numbers for the unit details on the map
type UnitDetailsView struct {
    Melee int `json:"melee"`
    Ranged int `json:"ranged"`
    RangedType string `json:"ranged-type,omitempty"`
    Defense int `json:"defense"`
    Resistance int `json:"resistance"`
    // per figure
    HitPoints int `json:"hit-points"`
    Figures int `json:"figures"`
    Moves float64 `json:"moves"`
    Level string `json:"level,omitempty"`
    UpkeepGold int `json:"upkeep-gold,omitempty"`
    UpkeepFood int `json:"upkeep-food,omitempty"`
    UpkeepMana int `json:"upkeep-mana,omitempty"`
    Abilities []string `json:"abilities"`
    Enchantments []string `json:"enchantments"`
}

func heroLevelName(hero *herolib.Hero) string {
    level := hero.GetHeroExperienceLevel()
    return level.Name()
}

func makeUnitDetails(unit units.StackUnit) UnitDetailsView {
    details := UnitDetailsView{
        Melee: unit.GetMeleeAttackPower(),
        Ranged: unit.GetRangedAttackPower(),
        Defense: unit.GetDefense(),
        Resistance: unit.GetResistance(),
        HitPoints: unit.GetHitPoints(),
        Figures: unit.GetCount(),
        Moves: unit.GetMovementSpeed(true).ToFloat(),
        UpkeepGold: unit.GetUpkeepGold(),
        UpkeepFood: unit.GetUpkeepFood(),
        UpkeepMana: unit.GetUpkeepMana(),
        Abilities: []string{},
        Enchantments: []string{},
    }
    if details.Ranged > 0 {
        details.RangedType = unit.GetRangedAttackDamageType().String()
    }
    if hero, ok := unit.(*herolib.Hero); ok {
        details.Level = heroLevelName(hero)
    } else if unit.GetRace() != data.RaceFantastic {
        details.Level = unit.GetExperienceLevel().Name()
    }
    for _, ability := range unit.GetAbilities() {
        details.Abilities = append(details.Abilities, ability.Name())
    }
    for _, enchantment := range unit.GetEnchantments() {
        details.Enchantments = append(details.Enchantments, enchantment.Name())
    }
    return details
}
