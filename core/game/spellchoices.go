package game

import (
    "cmp"
    "slices"

    "github.com/bfalese/civ-of-magic/core/artifact"
    "github.com/bfalese/civ-of-magic/core/assets"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/spellbook"
)

// What the magic screen needs for spells that take a choice by name (Spell Ward, Spell Binding,
// Disjunction, Resurrection) and for designing an item (Create Artifact, Enchant Item).

// one way to cast a spell that needs a choice: cast-spell's target is {choice: name} plus
// location (Spell Ward: the city) or wizard (Spell Binding, Disjunction: the owner)
type SpellChoiceView struct {
    Name string `json:"name"`
    // the city for Spell Ward
    Location *Location `json:"location,omitempty"`
    City string `json:"city,omitempty"`
    // the owner of the global enchantment (banner) for Spell Binding and Disjunction
    Wizard string `json:"wizard,omitempty"`
    WizardName string `json:"wizard-name,omitempty"`
}

// the choices for a spell cast with a choice, and whether it is one
func (model *GameModel) spellChoices(player *playerlib.Player, spell spellbook.Spell) ([]SpellChoiceView, bool) {
    choices := []SpellChoiceView{}
    switch spell.Name {
        case "Spell Ward":
            for _, city := range player.Cities {
                location := Location{Plane: city.Plane, X: city.X, Y: city.Y}
                for _, ward := range model.AvailableSpellWards(city).Values() {
                    choices = append(choices, SpellChoiceView{Name: ward.Name(), Location: &location, City: city.Name})
                }
            }
            slices.SortStableFunc(choices, func(a SpellChoiceView, b SpellChoiceView) int {
                return cmp.Or(cmp.Compare(a.City, b.City), cmp.Compare(a.Name, b.Name))
            })
        case "Spell Binding", "Disjunction", "Disjunction True":
            for _, other := range model.Players {
                if other == player || !player.IsAwareOf(other) || other.Defeated {
                    continue
                }
                for _, enchantment := range other.GlobalEnchantments.Values() {
                    if spell.Name == "Spell Binding" && player.GlobalEnchantments.Contains(enchantment) {
                        continue
                    }
                    choices = append(choices, SpellChoiceView{Name: enchantment.String(), Wizard: other.GetBanner().String(), WizardName: other.Wizard.Name})
                }
            }
            slices.SortStableFunc(choices, func(a SpellChoiceView, b SpellChoiceView) int {
                return cmp.Or(cmp.Compare(a.WizardName, b.WizardName), cmp.Compare(a.Name, b.Name))
            })
        case "Resurrection":
            if player.FreeHeroSlots() == 0 {
                return choices, true
            }
            for _, hero := range player.GetDeadHeroes() {
                choices = append(choices, SpellChoiceView{Name: hero.GetName()})
            }
            slices.SortFunc(choices, func(a SpellChoiceView, b SpellChoiceView) int {
                return cmp.Compare(a.Name, b.Name)
            })
        default:
            return nil, false
    }
    return choices, true
}

// the item designer's options (Create Artifact, Enchant Item): cast-spell's artifact
type ItemDesignView struct {
    Types []ItemTypeView `json:"types"`
    Powers []ItemPowerView `json:"powers"`
    // combat spells a staff or wand can hold, 1-4 charges
    Spells []ItemSpellView `json:"spells"`
    // the most powers, spell charges not counted
    MaxPowers int `json:"max-powers"`
    // Enchant Item takes only powers (not abilities) costing at most this
    EnchantItemLimit int `json:"enchant-item-limit"`
}

type ItemTypeView struct {
    Name string `json:"name"`
    // the item's cost before powers
    BaseCost int `json:"base-cost"`
    // powers cost this many times their price (2 for jewelry)
    PowerFactor int `json:"power-factor"`
    // the pictures the item can use: design image numbers with their asset ids
    Images []ItemImageView `json:"images"`
}

type ItemImageView struct {
    Image int `json:"image"`
    Asset string `json:"asset"`
}

type ItemPowerView struct {
    Name string `json:"name"`
    // one power per group (Attack, Defense, ...); abilities: one each from "Ability 1" and
    // "Ability 2", any number from "Ability 3"
    Group string `json:"group"`
    Ability bool `json:"ability,omitempty"`
    Cost int `json:"cost"`
    // the item types that can have it
    Types []string `json:"types"`
    // abilities need this many spellbooks of the realm
    Magic string `json:"magic,omitempty"`
    Books int `json:"books,omitempty"`
    // the wizard has the spellbooks for it
    Allowed bool `json:"allowed"`
}

type ItemSpellView struct {
    Name string `json:"name"`
    Magic string `json:"magic"`
    // the price of one charge
    ChargeCost int `json:"charge-cost"`
}

func (model *GameModel) itemDesignView(player *playerlib.Player) *ItemDesignView {
    if model.ArtifactPowers == nil {
        return nil
    }
    costs, compatibilities, err := model.ArtifactPowers()
    if err != nil {
        return nil
    }

    view := &ItemDesignView{MaxPowers: artifact.MaxDesignPowers, EnchantItemLimit: artifact.EnchantItemCostThreshold}
    for kind := artifact.ArtifactTypeSword; kind <= artifact.ArtifactTypePlate; kind++ {
        item := artifact.Artifact{Type: kind}
        typeView := ItemTypeView{Name: kind.Name(), BaseCost: artifact.CalculateCost(&item, costs), PowerFactor: 1}
        if kind == artifact.ArtifactTypeMisc {
            typeView.PowerFactor = 2
        }
        first, last := artifact.DesignImages(kind)
        for image := first; image <= last; image++ {
            typeView.Images = append(typeView.Images, ItemImageView{Image: image, Asset: assets.Item(image)})
        }
        view.Types = append(view.Types, typeView)
    }

    powers := make([]artifact.Power, 0, len(costs))
    for power := range costs {
        powers = append(powers, power)
    }
    slices.SortFunc(powers, func(a artifact.Power, b artifact.Power) int {
        return cmp.Or(cmp.Compare(a.Index, b.Index), cmp.Compare(a.Name, b.Name))
    })
    for _, power := range powers {
        powerView := ItemPowerView{Name: power.Name, Group: power.Type.String(), Cost: costs[power], Types: []string{}, Allowed: true}
        for kind := artifact.ArtifactTypeSword; kind <= artifact.ArtifactTypePlate; kind++ {
            compatible := compatibilities[power]
            if compatible.Contains(kind) {
                powerView.Types = append(powerView.Types, kind.Name())
            }
        }
        if artifact.IsAbility(power) {
            powerView.Ability = true
            powerView.Magic = power.Magic.String()
            powerView.Books = power.Amount
            powerView.Allowed = player.Wizard.MagicLevel(power.Magic) >= power.Amount
        }
        view.Powers = append(view.Powers, powerView)
    }

    view.Spells = []ItemSpellView{}
    for _, spell := range player.KnownSpells.CombatSpells(false).Spells {
        view.Spells = append(view.Spells, ItemSpellView{Name: spell.Name, Magic: spell.Magic.String(), ChargeCost: artifact.SpellChargeCost(spell)})
    }
    return view
}

// what the player pays for an item spell as a share of the item's cost (retorts, spellbooks)
func itemCostFactor(player *playerlib.Player, spell spellbook.Spell) float64 {
    const probe = 100000
    spell.OverrideCost = probe
    return float64(player.ComputeEffectiveSpellCost(spell, true)) / probe
}
