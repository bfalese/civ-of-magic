package city

import (
    "cmp"
    "slices"

    buildinglib "github.com/bfalese/civ-of-magic/core/building"
)

// One line of a city's resource breakdown (the dialogs behind the food/production/gold/... rows)
type ResourceUsage struct {
    Count int // can be negative
    Name string
    Replaced bool // true if the building has been replaced by another
}

func sortBuildings(buildings []buildinglib.Building) []buildinglib.Building {
    slices.SortFunc(buildings, func(a buildinglib.Building, b buildinglib.Building) int {
        return cmp.Compare(a, b)
    })
    return buildings
}

// true if a better building in the same line exists in the city (e.g. a shrine once there is a temple)
func (city *City) WasBuildingReplaced(building buildinglib.Building) bool {
    if building == buildinglib.BuildingNone {
        return false
    }

    replacedBy := building.ReplacedBy()
    return replacedBy != buildinglib.BuildingNone && (city.Buildings.Contains(replacedBy) || city.WasBuildingReplaced(replacedBy))
}

func (city *City) CanSellBuilding(building buildinglib.Building) bool {
    return city.BuildingInfo.ProductionCost(building) > 0
}

// gold received for selling the building
func (city *City) SellValue(building buildinglib.Building) int {
    cost := city.BuildingInfo.ProductionCost(building) / 3
    if building == buildinglib.BuildingCityWalls {
        cost /= 2
    }

    if cost < 1 {
        cost = 1
    }

    return cost
}

// remove the building (one sale per turn); returns the gold it sold for
func (city *City) SellBuilding(building buildinglib.Building) int {
    city.SoldBuilding = true
    gold := city.SellValue(building)
    city.Buildings.Remove(building)
    return gold
}

// gold needed to finish what the city is producing right now, and the production that buys.
// (0, 0) when there is nothing to buy (trade goods, housing, or already paid for)
func (city *City) BuyCost() (int, float32) {
    // returns the amount of gold and the amount of production that will be used to buy a building
    computeBuyAmount := func(cost int) (int, float32) {
        // FIXME: take terrain/race/buildings into effect, such as the miners guild for dwarves

        remaining := float32(cost) - city.Production
        if remaining > 0 {
            modifier := float32(4)
            if city.Production == 0 {
                modifier = 4
            } else if city.Production / float32(cost) < 1.0/3 {
                modifier = 3
            } else {
                modifier = 2
            }

            buyAmount := int(remaining * modifier)
            buyProduction := remaining

            return buyAmount, buyProduction
        }

        return 0, 0
    }

    buyAmount := 0
    buyProduction := float32(0)

    if city.ProducingBuilding != buildinglib.BuildingNone && city.ProducingBuilding != buildinglib.BuildingTradeGoods && city.ProducingBuilding != buildinglib.BuildingHousing {
        cost := float32(city.BuildingInfo.ProductionCost(city.ProducingBuilding))
        buyAmount, buyProduction = computeBuyAmount(int(cost))
    }

    if !city.ProducingUnit.IsNone() {
        buyAmount, buyProduction = computeBuyAmount(city.UnitProductionCost(&city.ProducingUnit))
    }

    return buyAmount, buyProduction
}

// set the number of farmers; the remaining non-rebel citizens work
func (city *City) SetFarmers(farmers int) {
    city.Farmers = farmers
    city.Workers = city.Citizens() - city.Rebels - city.Farmers
}

// move count workers to the fields
func (city *City) WorkersToFarmers(count int) {
    city.Workers -= count
    city.Farmers += count
}

func (city *City) FoodProducers() []ResourceUsage {
    var usages []ResourceUsage

    // FIXME: should this take enchantments like famine into account and only show half the food production?
    usages = append(usages, ResourceUsage{
        Count: city.FarmerFoodProduction(city.Farmers),
        Name: "Farmers",
    })

    buildings := city.Buildings.Values()
    for _, building := range sortBuildings(buildings) {
        value := 0
        switch building {
            case buildinglib.BuildingForestersGuild: value = 2
            case buildinglib.BuildingGranary: value = 2
            case buildinglib.BuildingFarmersMarket: value = 3
        }

        if value > 0 {
            usages = append(usages, ResourceUsage{
                Count: value,
                Name: city.BuildingInfo.Name(building),
                Replaced: city.WasBuildingReplaced(building),
            })
        }
    }

    wildGame := city.ComputeWildGame()
    if wildGame > 0 {
        usages = append(usages, ResourceUsage{
            Count: wildGame,
            Name: "Wild Game",
        })
    }

    return usages
}

func (city *City) WorkProducers() []ResourceUsage {
    var usage []ResourceUsage

    add := func(count int, name string, building buildinglib.Building){
        if count > 0 {
            usage = append(usage, ResourceUsage{
                Count: count,
                Name: name,
                Replaced: city.WasBuildingReplaced(building),
            })
        }
    }

    add(int(city.ProductionWorkers()), "Workers", buildinglib.BuildingNone)
    add(int(city.ProductionFarmers()), "Farmers", buildinglib.BuildingNone)
    add(int(city.ProductionTerrain()), "Terrain", buildinglib.BuildingNone)
    add(int(city.ProductionSawmill()), "Sawmill", buildinglib.BuildingSawmill)
    add(int(city.ProductionForestersGuild()), "Forester's Guild", buildinglib.BuildingForestersGuild)
    add(int(city.ProductionMinersGuild()), "Miner's Guild", buildinglib.BuildingMinersGuild)
    add(int(city.ProductionMechaniciansGuild()), "Mechanician's Guild", buildinglib.BuildingMechaniciansGuild)
    add(int(city.ProductionInspirations()), "Inspirations", buildinglib.BuildingNone)
    // FIXME: should this show halfed production in case of cursed lands?

    return usage
}

func (city *City) BuildingMaintenanceResources() []ResourceUsage {
    var usage []ResourceUsage

    for _, building := range city.Buildings.Values() {
        maintenance := city.BuildingInfo.UpkeepCost(building)
        if maintenance == 0 {
            continue
        }

        usage = append(usage, ResourceUsage{
            Count: maintenance,
            Name: city.BuildingInfo.Name(building),
            Replaced: city.WasBuildingReplaced(building),
        })
    }

    return usage
}

func (city *City) GoldProducers() []ResourceUsage {
    var usage []ResourceUsage

    add := func(count int, name string, building buildinglib.Building){
        if count > 0 {
            usage = append(usage, ResourceUsage{
                Count: count,
                Name: name,
                Replaced: city.WasBuildingReplaced(building),
            })
        }
    }

    // FIXME: add tiles (road/river/ocean)
    add(int(city.GoldTaxation()), "Taxes", buildinglib.BuildingNone)
    add(int(city.GoldTradeGoods()), "Trade Goods", buildinglib.BuildingTradeGoods)
    add(int(city.GoldMinerals()), "Minerals", buildinglib.BuildingNone)
    add(int(city.GoldMarketplace()), "Marketplace", buildinglib.BuildingMarketplace)
    add(int(city.GoldBank()), "Bank", buildinglib.BuildingBank)
    add(int(city.GoldMerchantsGuild()), "Merchant's Guild", buildinglib.BuildingMerchantsGuild)
    add(int(city.GoldProsperity()), "Prosperity", buildinglib.BuildingNone)

    return usage
}

func (city *City) PowerProducers() []ResourceUsage {
    var usage []ResourceUsage

    add := func(count int, name string, building buildinglib.Building){
        if count != 0 {
            usage = append(usage, ResourceUsage{
                Count: count,
                Name: name,
                Replaced: city.WasBuildingReplaced(building),
            })
        }
    }

    add(city.PowerCitizens(), "Townsfolk", buildinglib.BuildingNone)
    add(city.PowerFortress(), "Fortress", buildinglib.BuildingFortress)
    add(int(city.PowerShrine()), "Shrine", buildinglib.BuildingShrine)
    add(int(city.PowerTemple()), "Temple", buildinglib.BuildingTemple)
    add(int(city.PowerParthenon()), "Parthenon", buildinglib.BuildingParthenon)
    add(int(city.PowerCathedral()), "Cathedral", buildinglib.BuildingCathedral)
    add(city.PowerAlchemistsGuild(), "Alchemist's Guild", buildinglib.BuildingAlchemistsGuild)
    add(city.PowerWizardsGuild(), "Wizard's Guild", buildinglib.BuildingWizardsGuild)
    add(city.PowerMinerals(), "Minerals", buildinglib.BuildingNone)
    add(int(city.PowerDarkRituals()), "Dark Rituals", buildinglib.BuildingNone)

    // FIXME: add tiles (adamantium mine) and miner's guild

    return usage
}

func (city *City) ResearchProducers() []ResourceUsage {
    var usage []ResourceUsage

    for _, building := range sortBuildings(city.Buildings.Values()) {
        research := city.BuildingInfo.ResearchProduction(building)

        if research > 0 {
            usage = append(usage, ResourceUsage{
                Count: research,
                Name: city.BuildingInfo.Name(building),
                Replaced: city.WasBuildingReplaced(building),
            })
        }
    }

    return usage
}
