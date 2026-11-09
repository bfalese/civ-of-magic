package artifact

import (
    "errors"
    "fmt"

    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/lib/set"
)

// Designing an item for Create Artifact / Enchant Item without the designer screen: a Design names
// the item type and powers, and BuildDesign checks it with the same rules as the screen
// (game/magic/artifactview).

// Enchant Item can only use powers up to this cost
const EnchantItemCostThreshold = 200

// at most this many powers (spell charges not counted)
const MaxDesignPowers = 4

// the item pictures each type can use (indexes into items.lbx), as in the designer screen
var designImages = map[ArtifactType][2]int{
    ArtifactTypeSword: {0, 8},
    ArtifactTypeMace: {9, 19},
    ArtifactTypeAxe: {20, 28},
    ArtifactTypeBow: {29, 37},
    ArtifactTypeStaff: {38, 46},
    ArtifactTypeWand: {107, 115},
    ArtifactTypeMisc: {72, 106},
    ArtifactTypeShield: {62, 71},
    ArtifactTypeChain: {47, 54},
    ArtifactTypePlate: {55, 61},
}

type Design struct {
    // Sword, Mace, Axe, Bow, Staff, Wand, Misc, Shield, Chain, Plate
    Type string `json:"type"`
    // a custom name; empty for the generated one
    Name string `json:"name,omitempty"`
    // picture index (see designImages); 0 for the type's first picture
    Image int `json:"image,omitempty"`
    // power names as in the power table, e.g. "+2 Attack", "Flaming"
    Powers []string `json:"powers"`
    // a combat spell to embed (staffs and wands), with 1-4 charges
    Spell string `json:"spell,omitempty"`
    Charges int `json:"charges,omitempty"`
}

func artifactTypeByName(name string) ArtifactType {
    for kind := ArtifactTypeSword; kind <= ArtifactTypePlate; kind++ {
        if kind.Name() == name {
            return kind
        }
    }
    return ArtifactTypeNone
}

// the first and last picture a type can use
func DesignImages(kind ArtifactType) (int, int) {
    images := designImages[kind]
    return images[0], images[1]
}

func IsAbility(power Power) bool {
    return power.Type == PowerTypeAbility1 || power.Type == PowerTypeAbility2 || power.Type == PowerTypeAbility3
}

// check the design and make the item. enchantItem limits the powers' cost; wizard's spellbooks
// limit the abilities; availableSpells are the spells that can be embedded.
func BuildDesign(design Design, enchantItem bool, wizard spellbook.Wizard, availableSpells spellbook.Spells, costs map[Power]int, compatibilities map[Power]set.Set[ArtifactType]) (*Artifact, error) {
    kind := artifactTypeByName(design.Type)
    if kind == ArtifactTypeNone {
        return nil, fmt.Errorf("unknown item type %q", design.Type)
    }

    images := designImages[kind]
    image := design.Image
    if image == 0 {
        image = images[0]
    }
    if image < images[0] || image > images[1] {
        return nil, fmt.Errorf("picture %v is not one of the %v pictures (%v-%v)", image, design.Type, images[0], images[1])
    }

    item := &Artifact{
        Type: kind,
        Image: image,
        CatalogIndex: NotPremade,
    }

    if len(design.Powers) > MaxDesignPowers {
        return nil, errors.New("Only four powers may be enchanted into an item")
    }

    usedTypes := set.MakeSet[PowerType]()
    for _, name := range design.Powers {
        var power Power
        found := false
        for candidate := range costs {
            if candidate.Name == name {
                power = candidate
                found = true
                break
            }
        }
        if !found {
            return nil, fmt.Errorf("unknown power %q", name)
        }

        compatible := compatibilities[power]
        if !compatible.Contains(kind) {
            return nil, fmt.Errorf("a %v can't have %v", design.Type, name)
        }

        if IsAbility(power) {
            if wizard.MagicLevel(power.Magic) < power.Amount {
                return nil, fmt.Errorf("%v needs %v %v spellbooks", name, power.Amount, power.Magic)
            }
            // the first two ability groups allow one choice each, the third any number
            if power.Type != PowerTypeAbility3 && usedTypes.Contains(power.Type) {
                return nil, fmt.Errorf("only one power of the group of %v", name)
            }
        } else {
            if enchantItem && costs[power] > EnchantItemCostThreshold {
                return nil, fmt.Errorf("%v is too strong for Enchant Item", name)
            }
            if usedTypes.Contains(power.Type) {
                return nil, fmt.Errorf("only one %v power", power.Type)
            }
        }

        usedTypes.Insert(power.Type)
        item.AddPower(power)
    }

    if design.Spell != "" {
        if kind != ArtifactTypeStaff && kind != ArtifactTypeWand {
            return nil, errors.New("only staffs and wands can hold spell charges")
        }
        spell := availableSpells.FindByName(design.Spell)
        if !spell.Valid() {
            return nil, fmt.Errorf("%v can't be embedded", design.Spell)
        }
        if design.Charges < 1 || design.Charges > 4 {
            return nil, errors.New("charges must be 1 to 4")
        }
        item.AddPower(Power{
            Type: PowerTypeSpellCharges,
            Spell: spell,
            Amount: design.Charges,
            Name: fmt.Sprintf("%v x%v", spell.Name, design.Charges),
        })
    }

    item.Name = GetName(item, design.Name)
    item.Cost = CalculateCost(item, costs)
    return item, nil
}
