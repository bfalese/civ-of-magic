package game

import (
    "cmp"
    "image"
    "math"
    "math/rand/v2"
    "slices"

    "github.com/bfalese/civ-of-magic/core/ai"
    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/setup"
    "github.com/bfalese/civ-of-magic/core/units"
    "github.com/bfalese/civ-of-magic/lib/fraction"
)

// New game setup: players, capitals, starting units, raiders.

// the two units a wizard starts with in the capital
func StartingUnits(race data.Race) []units.Unit {
    switch race {
        case data.RaceLizard: return []units.Unit{units.LizardSwordsmen, units.LizardSpearmen}
        case data.RaceNomad: return []units.Unit{units.NomadSwordsmen, units.NomadSpearmen}
        case data.RaceOrc: return []units.Unit{units.OrcSwordsmen, units.OrcSpearmen}
        case data.RaceTroll: return []units.Unit{units.TrollSwordsmen, units.TrollSpearmen}
        case data.RaceBarbarian: return []units.Unit{units.BarbarianSwordsmen, units.BarbarianSpearmen}
        case data.RaceBeastmen: return []units.Unit{units.BeastmenSwordsmen, units.BeastmenSpearmen}
        case data.RaceDarkElf: return []units.Unit{units.DarkElfSwordsmen, units.DarkElfSpearmen}
        case data.RaceDraconian: return []units.Unit{units.DraconianSwordsmen, units.DraconianSpearmen}
        case data.RaceDwarf: return []units.Unit{units.DwarfSwordsmen, units.DwarfSwordsmen}
        case data.RaceGnoll: return []units.Unit{units.GnollSwordsmen, units.GnollSpearmen}
        case data.RaceHalfling: return []units.Unit{units.HalflingSwordsmen, units.HalflingSpearmen}
        case data.RaceHighElf: return []units.Unit{units.HighElfSwordsmen, units.HighElfSpearmen}
        case data.RaceHighMen: return []units.Unit{units.HighMenSwordsmen, units.HighMenSpearmen}
        case data.RaceKlackon: return []units.Unit{units.KlackonSwordsmen, units.KlackonSpearmen}
        default: return nil
    }
}

func euclideanDistance(x1, y1, x2, y2 int) float64 {
    dx := float64(x1 - x2)
    dy := float64(y1 - y2)

    return math.Sqrt(dx*dx + dy*dy)
}

// a good spot for a starting city: far from other cities, with a large maximum population
func (model *GameModel) FindCityLocation(startingPlane data.Plane, cityArea CityValidArea) (int, int) {
    allCities := model.AllCities()

    closestDistance := func(x, y int) int {
        distance := -1

        for _, city := range allCities {
            if city.Plane == startingPlane {
                d := int(euclideanDistance(x, y, city.X, city.Y))
                if distance == -1 || d < distance {
                    distance = d
                }
            }
        }

        if distance == -1 {
            return 0
        } else {
            return distance
        }
    }

    type CityLocation struct {
        X, Y int
        // distance to closest city
        Distance int
        Population int
    }

    var locations []CityLocation
    for range 10 {
        // x, y, ok := model.FindValidCityLocation(startingPlane)
        x, y, ok := cityArea.FindLocation()
        if ok {
            distance := closestDistance(x, y)
            // either there are no other cities nearby (distance=0) or the closest city is farther than 10 squares away
            if distance == 0 || distance > 10 {
                locations = append(locations, CityLocation{X: x, Y: y, Distance: distance, Population: model.ComputeMaximumPopulation(x, y, startingPlane)})
            }
        }
    }

    // compute a weighted sum of distance to other cities and maximum population of the location
    computeValue := func (point CityLocation) float64 {
        distance := point.Distance
        // assume a distance if no other cities are nearby
        if distance == 0 {
            distance = 150
        }

        return math.Log2(float64(distance)) * 2 + float64(point.Population) * 0.5
    }

    slices.SortFunc(locations, func(pointA, pointB CityLocation) int {
        return cmp.Compare(computeValue(pointA), computeValue(pointB))
    })

    if len(locations) > 0 {
        // choose furthest point
        return locations[len(locations) - 1].X, locations[len(locations) - 1].Y
    } else {
        // couldn't find a good spot, just pick anything
        for range 100 {
            cityX, cityY, ok := cityArea.FindLocation()
            if ok {
                return cityX, cityY
            }
        }
    }

    return -1, -1
}

// add a wizard with a capital (fortress, summoning circle, a few starting buildings) and two starting
// units. suggestCityName names the capital. returns the player and its capital.
func (model *GameModel) InitializePlayer(wizard setup.WizardCustom, isHuman bool, arcanusCityArea CityValidArea, myrrorCityArea CityValidArea, suggestCityName func(race data.Race) string) (*playerlib.Player, *citylib.City) {
    area := arcanusCityArea
    startingPlane := data.PlaneArcanus
    if wizard.RetortEnabled(data.RetortMyrran) {
        startingPlane = data.PlaneMyrror
        area = myrrorCityArea
    }

    player := model.AddPlayer(wizard, isHuman)

    cityName := suggestCityName(player.Wizard.Race)

    cityX, cityY := model.FindCityLocation(startingPlane, area)
    area[image.Pt(cityX, cityY)] = false

    model.GetMap(startingPlane).SetRoad(cityX, cityY, startingPlane == data.PlaneMyrror)

    introCity := citylib.MakeCity(cityName, cityX, cityY, player.Wizard.Race, model.BuildingInfo, model.GetMap(startingPlane), model, player)
    introCity.Population = 4000
    introCity.Plane = startingPlane

    for _, building := range []buildinglib.Building{buildinglib.BuildingSmithy, buildinglib.BuildingBarracks, buildinglib.BuildingBuildersHall} {
        if introCity.GetBuildableBuildings().Contains(building) {
            introCity.Buildings.Insert(building)
        }
    }

    introCity.Buildings.Insert(buildinglib.BuildingFortress)
    introCity.Buildings.Insert(buildinglib.BuildingSummoningCircle)
    introCity.ProducingBuilding = buildinglib.BuildingHousing
    introCity.ProducingUnit = units.UnitNone
    introCity.Farmers = 4

    introCity.ResetCitizens()

    player.AddCity(introCity)

    for _, unit := range StartingUnits(player.Wizard.Race) {
        player.AddUnit(units.MakeOverworldUnitFromUnit(unit, cityX, cityY, startingPlane, wizard.Banner, player.MakeExperienceInfo(), player.MakeUnitEnchantmentProvider()))
    }

    // for debugging purposes, to start the game super powered
    /*
    if isHuman {
        player.Admin = true
        player.Mana = 90000
        for range 3 {
            player.AddUnit(units.MakeOverworldUnitFromUnit(units.GreatDrake, cityX, cityY, startingPlane, wizard.Banner, player.MakeExperienceInfo(), player.MakeUnitEnchantmentProvider()))
        }
    }
    */

    player.LiftFog(cityX, cityY, 3, introCity.Plane)

    if isHuman {
        model.Plane = startingPlane
    }

    return player, introCity
}

// add the raiders (neutral player) with five towns on each plane
func (model *GameModel) InitializeNeutralPlayer(arcanusCityArea CityValidArea, myrrorCityArea CityValidArea, suggestCityName func(race data.Race) string) *playerlib.Player {
    wizard := setup.WizardCustom{
        Name: "Raiders",
        Base: data.WizardMerlin, // doesn't really matter
        Race: data.RaceBarbarian, // doesn't really matter
        Banner: data.BannerBrown,
    }

    player := model.AddPlayer(wizard, false)
    player.AIBehavior = ai.MakeRaiderAI()
    player.TaxRate = fraction.Zero()

    for _, plane := range []data.Plane{data.PlaneArcanus, data.PlaneMyrror} {
        randomRace := func() data.Race {
            switch plane {
                case data.PlaneArcanus: return data.ArcanianRaces()[rand.N(len(data.ArcanianRaces()))]
                case data.PlaneMyrror: return data.MyrranRaces()[rand.N(len(data.MyrranRaces()))]
            }

            return data.RaceNone
        }

        area := arcanusCityArea
        if plane == data.PlaneMyrror {
            area = myrrorCityArea
        }

        for range 5 {
            cityX, cityY := model.FindCityLocation(plane, area)

            // should every neutral town be a random race, or should they all be related?
            race := randomRace()
            cityName := suggestCityName(race)
            city := citylib.MakeCity(cityName, cityX, cityY, race, model.BuildingInfo, model.GetMap(plane), model, player)
            city.Population = rand.N(5) * 1000 + 2000
            city.ProducingBuilding = buildinglib.BuildingHousing
            city.Plane = plane
            city.Farmers = city.Citizens()
            city.ResetCitizens()

            area[image.Pt(cityX, cityY)] = false

            player.AddCity(city)
        }
    }

    return player
}
