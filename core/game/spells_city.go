package game

import (
    "github.com/bfalese/civ-of-magic/core/building"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
)

// Effects of spells cast on cities. The frontend handles city selection, fizzle checks, sounds and
// the "new building" screen.

// a city enchantment takes hold
func (model *GameModel) AddCityEnchantment(city *citylib.City, player *playerlib.Player, enchantment data.CityEnchantment) {
    city.AddEnchantment(enchantment, player.GetBanner())
    city.UpdateUnrest()
}

// a spell adds a building (Wall of Stone, Summoning Circle, Move Fortress)
func (model *GameModel) AddCityBuilding(city *citylib.City, newBuilding building.Building) {
    city.AddBuilding(newBuilding)
    city.UpdateUnrest()
}

// Consecration removes the city curses
func (model *GameModel) ApplyConsecration(city *citylib.City) {
    // Remove all the city curses from the targeted city on cast (https://masterofmagic.fandom.com/wiki/Consecration)
    // Wiki specifies only the following ones as city curses
    city.RemoveEnchantments(
        data.CityEnchantmentChaosRift,
        data.CityEnchantmentCursedLands,
        data.CityEnchantmentEvilPresence,
        data.CityEnchantmentFamine,
        data.CityEnchantmentPestilence,
    )
}

// after Summoning Circle is placed in chosenCity: the old circle disappears
func (model *GameModel) ApplySummoningCircleMoved(player *playerlib.Player, chosenCity *citylib.City) {
    for _, city := range player.Cities {
        if city != chosenCity && city.Buildings.Contains(building.BuildingSummoningCircle) {
            city.Buildings.Remove(building.BuildingSummoningCircle)
            break
        }
    }
}

// after Move Fortress to chosenCity: the old fortress disappears and a summoning circle there moves
// along; the wizard takes the city's race
func (model *GameModel) ApplyFortressMoved(player *playerlib.Player, chosenCity *citylib.City) {
    player.Wizard.Race = chosenCity.Race

    for _, city := range player.Cities {
        if city != chosenCity && city.Buildings.Contains(building.BuildingFortress) {
            city.Buildings.Remove(building.BuildingFortress)

            if city.Buildings.Contains(building.BuildingSummoningCircle) {
                chosenCity.Buildings.Insert(building.BuildingSummoningCircle)
                city.Buildings.Remove(building.BuildingSummoningCircle)
            }
            break
        }
    }

    player.UpdateUnrest()
}
