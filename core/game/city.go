package game

import (
    "github.com/bfalese/civ-of-magic/core/artifact"
    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/setup"
)

type ChangeCityEnchantments int
const (
    ChangeCityKeepEnchantments ChangeCityEnchantments = iota
    ChangeCityRemoveOwnerEnchantments
    ChangeCityRemoveAllEnchantments
)

// city is controlled by the newOwner instead of owner
func ChangeCityOwner(city *citylib.City, owner *playerlib.Player, newOwner *playerlib.Player, enchantmentChange ChangeCityEnchantments) {
    owner.RemoveCity(city)
    newOwner.AddCity(city)
    city.ReignProvider = newOwner

    city.Buildings.Remove(buildinglib.BuildingFortress)
    city.Buildings.Remove(buildinglib.BuildingSummoningCircle)

    switch enchantmentChange {
        case ChangeCityKeepEnchantments:
        case ChangeCityRemoveOwnerEnchantments:
            city.RemoveAllEnchantmentsByOwner(owner.GetBanner())
        case ChangeCityRemoveAllEnchantments:
            city.Enchantments.Clear()
    }

    city.UpdateUnrest()
}

// to be able to use the artifact, the wizard must have enough magic books to satisfy the artifact's requirements
func CanUseArtifact(check *artifact.Artifact, wizard setup.WizardCustom) bool {
    // all artifact requirements must be satisfied
    for _, requirement := range check.Requirements {
        if wizard.MagicLevel(requirement.MagicType) < requirement.Amount {
            return false
        }
    }

    // and all ability requirements must be satisfied
    for _, power := range check.Powers {
        if power.Type == artifact.PowerTypeAbility1 || power.Type == artifact.PowerTypeAbility2 || power.Type == artifact.PowerTypeAbility3 {
            if wizard.MagicLevel(power.Magic) < power.Amount {
                return false
            }
        }
    }

    return true
}
