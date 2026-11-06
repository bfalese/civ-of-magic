package city

import (
    "testing"

    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/units"
    "github.com/bfalese/civ-of-magic/lib/fraction"
)

// building infos where only the barracks (index 3) and city walls have a cost
func makeEconomyTestInfos() buildinglib.BuildingInfos {
    infos := make(buildinglib.BuildingInfos, 40)
    infos[infos.GetBuildingIndex(buildinglib.BuildingBarracks)] = buildinglib.BuildingInfo{Name: "Barracks", ConstructionCost: 30}
    infos[infos.GetBuildingIndex(buildinglib.BuildingCityWalls)] = buildinglib.BuildingInfo{Name: "City Walls", ConstructionCost: 60}
    return infos
}

func makeEconomyTestCity() *City {
    reign := NoReign{TaxRate: fraction.Make(3, 2)}
    city := MakeCity("Test City", 10, 10, data.RaceHighMen, makeEconomyTestInfos(), &Catchment{Map: makeSimpleMap()}, &NoCities{}, &reign)
    city.Population = 6000
    city.Farmers = 6
    city.ResetCitizens()
    return city
}

func TestSellBuilding(test *testing.T) {
    city := makeEconomyTestCity()
    city.Buildings.Insert(buildinglib.BuildingBarracks)
    city.Buildings.Insert(buildinglib.BuildingCityWalls)

    if !city.CanSellBuilding(buildinglib.BuildingBarracks) {
        test.Errorf("barracks should be sellable")
    }
    if city.SellValue(buildinglib.BuildingCityWalls) != 10 {
        test.Errorf("city walls sell for a sixth of their cost, got %v", city.SellValue(buildinglib.BuildingCityWalls))
    }

    gold := city.SellBuilding(buildinglib.BuildingBarracks)
    if gold != 10 {
        test.Errorf("barracks sell for a third of their cost, got %v", gold)
    }
    if city.Buildings.Contains(buildinglib.BuildingBarracks) || !city.SoldBuilding {
        test.Errorf("barracks should be gone and the city marked as having sold this turn")
    }
}

func TestBuyCost(test *testing.T) {
    city := makeEconomyTestCity()

    city.ProducingBuilding = buildinglib.BuildingBarracks
    city.Production = 0
    gold, production := city.BuyCost()
    if gold != 120 || production != 30 {
        test.Errorf("nothing built yet costs 4 gold per production: got %v gold for %v", gold, production)
    }

    city.Production = 20
    gold, production = city.BuyCost()
    if gold != 20 || production != 10 {
        test.Errorf("two thirds built costs 2 gold per production: got %v gold for %v", gold, production)
    }

    city.ProducingBuilding = buildinglib.BuildingTradeGoods
    city.ProducingUnit = units.UnitNone
    if gold, production = city.BuyCost(); gold != 0 || production != 0 {
        test.Errorf("trade goods can't be bought, got %v %v", gold, production)
    }
}

func TestSetFarmers(test *testing.T) {
    city := makeEconomyTestCity()
    city.SetFarmers(2)
    if city.Farmers != 2 || city.Workers != city.Citizens() - city.Rebels - 2 {
        test.Errorf("farmers=%v workers=%v rebels=%v citizens=%v", city.Farmers, city.Workers, city.Rebels, city.Citizens())
    }

    city.WorkersToFarmers(1)
    if city.Farmers != 3 {
        test.Errorf("one worker should become a farmer, farmers=%v", city.Farmers)
    }
}
