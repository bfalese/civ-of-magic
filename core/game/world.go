package game

import (
    "image"
    "math/rand/v2"
    "log"
    "fmt"
    "slices"
    "github.com/bfalese/civ-of-magic/core/setup"
    "github.com/bfalese/civ-of-magic/core/units"
    "github.com/bfalese/civ-of-magic/core/terrain"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/artifact"
    "github.com/bfalese/civ-of-magic/core/combat"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/maplib"
    "github.com/bfalese/civ-of-magic/lib/set"
)

func ChooseUniqueWizard(players []*playerlib.Player, allSpells spellbook.Spells) (setup.WizardCustom, bool) {
    // pick a new wizard with an unused wizard base and banner color, and race
    // if on myrror then select a myrran race

    chooseBase := func() (setup.WizardSlot, bool) {
        choices := slices.Clone(setup.DefaultWizardSlots())
        choices = slices.DeleteFunc(choices, func (wizard setup.WizardSlot) bool {
            if wizard.Name == "Custom" {
                return true
            }

            for _, player := range players {
                if player.Wizard.Base == wizard.Base {
                    return true
                }
            }

            return false
        })

        if len(choices) == 0 {
            return setup.WizardSlot{}, false
        }

        return choices[rand.N(len(choices))], true
    }

    chooseRace := func(myrror bool) (data.Race, bool) {
        var choices []data.Race
        if myrror {
            choices = slices.Clone(data.MyrranRaces())
        } else {
            choices = slices.Clone(data.ArcanianRaces())
        }

        choices = slices.DeleteFunc(choices, func (race data.Race) bool {
            for _, player := range players {
                if player.Wizard.Race == race {
                    return true
                }
            }

            return false
        })

        if len(choices) == 0 {
            return data.RaceNone, false
        }

        return choices[rand.N(len(choices))], true
    }

    chooseBanner := func() (data.BannerType, bool) {
        choices := []data.BannerType{data.BannerGreen, data.BannerBlue, data.BannerRed, data.BannerPurple, data.BannerYellow}
        choices = slices.DeleteFunc(choices, func (banner data.BannerType) bool {
            for _, player := range players {
                if player.Wizard.Banner == banner {
                    return true
                }
            }

            return false
        })

        if len(choices) == 0 {
            return data.BannerGreen, false
        }

        return choices[rand.N(len(choices))], true
    }

    wizard, ok := chooseBase()

    if !ok {
        return setup.WizardCustom{}, false
    }

    race, ok := chooseRace(wizard.ExtraRetort == data.RetortMyrran)
    if !ok {
        return setup.WizardCustom{}, false
    }

    banner, ok := chooseBanner()
    if !ok {
        return setup.WizardCustom{}, false
    }

    var retorts []data.Retort
    if wizard.ExtraRetort != data.RetortNone {
        retorts = []data.Retort{wizard.ExtraRetort}
    }

    customWizard := setup.WizardCustom{
        Name: wizard.Name,
        Base: wizard.Base,
        Race: race,
        Books: slices.Clone(wizard.Books),
        Banner: banner,
        Retorts: retorts,
    }

    customWizard.StartingSpells.AddAllSpells(setup.GetStartingSpells(&customWizard, allSpells))
    return customWizard, true

}

func (model *GameModel) DoMeteorStorm() {
    immolate := func (unit units.StackUnit) int {
        damage, _ := combat.ApplyAreaDamage(&UnitDamageWrapper{StackUnit: unit}, 4, units.DamageImmolation, 0)
        return damage
    }

    if !model.HasEnchantment(data.EnchantmentMeteorStorm) {
        return
    }

    affectedPlayers := set.NewSet[*playerlib.Player]()
    for _, player := range model.Players {
        if player.Defeated {
            continue
        }

        // add every other player to the affected set.
        // if another player also cast meteor storm, then ultimately the current player will also
        // be added to the set, meaning both players' meteor storms affects the other
        if player.HasEnchantment(data.EnchantmentMeteorStorm) {
            for _, otherPlayer := range model.Players {
                if otherPlayer != player {
                    affectedPlayers.Insert(otherPlayer)
                }
            }
        }
    }

    entityInfo := model.ComputeCityStackInfo()

    for _, player := range model.Players {
        if player.Defeated {
            continue
        }

        // non-garrisoned units take immolation damage
        var toRemove []units.StackUnit
        for unit := range player.Units() {
            if entityInfo.FindCity(unit.GetX(), unit.GetY(), unit.GetPlane()) == nil {
                immolate(unit)
                if unit.GetHealth() <= 0 {
                    toRemove = append(toRemove, unit)
                }
            }
        }

        for _, unit := range toRemove {
            player.RemoveUnit(unit)
        }

        if affectedPlayers.Contains(player) {
            var removeCities []*citylib.City
            for _, city := range player.Cities {
                // chaos ward and consecration protect the city
                if city.HasEnchantment(data.CityEnchantmentChaosWard) || city.HasEnchantment(data.CityEnchantmentConsecration) {
                    continue
                }

                // destroy all outposts
                if city.Outpost {
                    removeCities = append(removeCities, city)
                    continue
                }

                // buildings hae a 1% chance of being destroyed
                var destroyedBuildings []buildinglib.Building
                for _, building := range city.Buildings.Values() {
                    if rand.N(100) == 0 {
                        // FIXME: include nightshade protection
                        destroyedBuildings = append(destroyedBuildings, building)
                    }
                }

                for _, building := range destroyedBuildings {
                    city.Buildings.Remove(building)
                }

            }

            for _, city := range removeCities {
                player.RemoveCity(city)
            }
        }
    }

}

func (model *GameModel) DisbandUnits(player *playerlib.Player) []string {
    /*
    if 2 > 1 {
        return nil
    }
    */

    // keep removing units until the upkeep value can be paid
    ok := false
    var disbandedMessages []string
    for player.UnitCount() > 0 && !ok {
        ok = true

        goldIssue, foodIssue, manaIssue := model.CheckDisband(player)

        if goldIssue || foodIssue || manaIssue {
            ok = false
            disbanded := false

            allUnits := slices.Collect(player.Units())

            // try to disband one unit that is taking up resources
            for i := len(allUnits) - 1; i >= 0; i-- {
                unit := allUnits[i]
                // disband the unit for the right reason
                if goldIssue && unit.GetUpkeepGold() > 0 {
                    log.Printf("Disband %v due to lack of gold", unit)
                    disbandedMessages = append(disbandedMessages, fmt.Sprintf("%v disbanded due to lack of gold", unit.GetName()))
                    player.RemoveUnit(unit)
                    disbanded = true
                    break
                }

                if foodIssue && unit.GetUpkeepFood() > 0 {
                    log.Printf("Disband %v due to lack of food", unit)
                    disbandedMessages = append(disbandedMessages, fmt.Sprintf("%v disbanded due to lack of food", unit.GetName()))
                    player.RemoveUnit(unit)
                    disbanded = true
                    break
                }

                if manaIssue && unit.GetUpkeepMana() > 0 {
                    log.Printf("Disband %v due to lack of mana", unit)
                    disbandedMessages = append(disbandedMessages, fmt.Sprintf("%v disbanded due to lack of mana", unit.GetName()))
                    player.RemoveUnit(unit)
                    disbanded = true
                    break
                }
            }

            if !disbanded {
                // fail safe to make sure we exit the loop in case somehow a unit was not disbanded
                break
            }

            var toRemove []units.StackUnit
            // check land walkers on ocean tiles that do not have valid transport
            // FIXME: handle not enough transports
            for _, stack := range player.Stacks {
                // if the stack can fly for whatever reason then no units will drown
                if stack.AllFlyers() {
                    continue
                }
                mapUse := model.GetMap(stack.Plane())
                hasTransport := stack.HasSailingUnits(false)
                if !hasTransport && !mapUse.GetTile(stack.X(), stack.Y()).Tile.IsLand() {
                    for _, unit := range stack.Units() {
                        if unit.IsLandWalker() {
                            toRemove = append(toRemove, unit)
                        }
                    }
                }
            }

            for _, unit := range toRemove {
                player.RemoveUnit(unit)
            }
        }
    }

    return disbandedMessages
}

// return true if the stack is going to build a road this turn
func (model *GameModel) MaybeBuildRoads(stack *playerlib.UnitStack, player *playerlib.Player) bool {
    var buildRoadUnits []units.StackUnit
    for _, unit := range stack.Units() {
        if len(unit.GetBuildRoadPath()) > 0 {
            buildRoadUnits = append(buildRoadUnits, unit)
        }
    }

    if len(buildRoadUnits) > 0 {
        if len(buildRoadUnits) < len(stack.Units()) {
            stack = player.SplitStack(stack, buildRoadUnits)
        }

        stack.EnableMovers()
        roadPath := buildRoadUnits[0].GetBuildRoadPath()
        stack.CurrentPath = nil

        useMap := model.GetMap(stack.Plane())

        hasRoad := useMap.ContainsRoad(stack.X(), stack.Y())

        // build a road here
        if !hasRoad {
            for _, unit := range buildRoadUnits {
                unit.SetBusy(units.BusyStatusBuildRoad)
                stack.SetActive(unit, false)
                unit.SetBuildRoadPath(roadPath[1:])
            }

            return true
        } else {

            // i=1
            // [R, X, X]
            // path = [R, X]
            // roadPath = [X, X]

            // move along the road path until we find a point that doesn't have a road
            i := 0
            for i < len(roadPath) {
                point := roadPath[i]
                hasRoad := useMap.ContainsRoad(point.X, point.Y)
                if !hasRoad {
                    break
                }

                i += 1

                // log.Printf("Unit stack %v is building road along path %v. Current %v, %v hasRoad=%v", stack, roadPath, stack.X(), stack.Y(), hasRoad)
            }

            if i >= len(roadPath) {
                i = len(roadPath) - 1
            }

            stack.CurrentPath = roadPath[:i+1]
            for _, unit := range buildRoadUnits {
                unit.SetBuildRoadPath(roadPath[i:])
            }
        }
    }

    return false
}

// turn off enchantments that can not be afforded
func (model *GameModel) DissipateEnchantments(player *playerlib.Player, power int) {
    // if time stop is in effect then only check that spell for dissipation, all other spells have no upkeep
    if player.HasEnchantment(data.EnchantmentTimeStop) {
        if player.Mana < data.EnchantmentTimeStop.UpkeepMana() {
            player.RemoveEnchantment(data.EnchantmentTimeStop)
        }

        return
    }

    isManaIssue := func() bool {
        manaPerTurn := player.ManaPerTurn(power, model)
        return player.Mana + manaPerTurn < 0
    }

    // keep removing enchantments until there is no more mana issue
    for player.GlobalEnchantments.Size() > 0 && isManaIssue() {
        enchantments := player.GlobalEnchantments.Values()
        enchantment := enchantments[rand.N(len(enchantments))]
        player.GlobalEnchantments.Remove(enchantment)
    }

    // keep removing city enchantments until there is no more mana issue
    for {
        enchantments := model.GetCityEnchantmentsByBanner(player.GetBanner())
        if len(enchantments) == 0 || !isManaIssue() {
            break
        }

        enchantment := enchantments[rand.N(len(enchantments))]
        enchantment.City.CancelEnchantment(enchantment.Enchantment.Enchantment, enchantment.Enchantment.Owner)
    }

    var enchantedUnits []units.StackUnit
    for unit := range player.Units() {
        if len(unit.GetEnchantments()) > 0 {
            enchantedUnits = append(enchantedUnits, unit)
        }
    }

    for len(enchantedUnits) > 0 && isManaIssue() {
        unit := enchantedUnits[rand.N(len(enchantedUnits))]
        enchantments := unit.GetEnchantments()
        enchantment := enchantments[rand.N(len(enchantments))]
        unit.RemoveEnchantment(enchantment)
        if len(unit.GetEnchantments()) == 0 {
            enchantedUnits = slices.DeleteFunc(enchantedUnits, func(u units.StackUnit) bool {
                return u == unit
            })
        }
    }
}

func (model *GameModel) DoChaosRift() {
    for _, city := range model.AllCities() {
        if city.HasEnchantment(data.CityEnchantmentChaosRift) {

            // do 5 magical attacks of strength 8 to units in the city
            stack, player := model.FindStack(city.X, city.Y, city.Plane)
            if stack != nil && !stack.IsEmpty() {
                stackUnits := stack.Units()
                for range 5 {
                    choice := stackUnits[rand.N(len(stackUnits))]

                    // regeneration units are never hurt by overland spells
                    if choice.HasAbility(data.AbilityRegeneration) {
                        continue
                    }

                    wrapper := &UnitDamageWrapper{StackUnit: choice}

                    combat.ApplyDamage(wrapper, []int{combat.ComputeRoll(8, 30)}, units.DamageRangedMagical, combat.DamageSourceSpell, combat.DamageModifiers{ArmorPiercing: true, Magic: data.ChaosMagic})
                }

                // check for dead units
                for _, unit := range stackUnits {
                    if unit.GetHealth() <= 0 {
                        player.RemoveUnit(unit)
                    }
                }
            }

            // each building has a 5% chance of being destroyed
            var destroyedBuildings []buildinglib.Building
            for _, building := range city.Buildings.Values() {
                if model.BuildingInfo.ProductionCost(building) != 0 {
                    if rand.N(100) < 5 {
                        destroyedBuildings = append(destroyedBuildings, building)
                    }
                }
            }

            for _, building := range destroyedBuildings {
                // emit a notice?
                city.Buildings.Remove(building)
            }
        }
    }
}

func (model *GameModel) FindValidCityLocationOnShore(plane data.Plane) (int, int, bool) {
    mapUse := model.GetMap(plane)
    continents := mapUse.Map.FindContinents()

    for i := 0; i < 10; i++ {
        continentIndex := rand.N(len(continents))
        continent := continents[continentIndex]
        if continent.Size() > 100 {

            var candidates []image.Point
            for _, point := range continent.Values() {
                x := point.X
                y := point.Y
                tile := terrain.GetTile(mapUse.Map.Terrain[x][y])
                if y > 3 && y < mapUse.Map.Rows() - 3 && tile.IsLand() && !tile.IsMagic() && mapUse.GetEncounter(x, y) == nil && !model.ContainsCity(x, y, plane) {

                    found := false
                    for dx := -1; dx <= 1; dx++ {
                        for dy := -1; dy <= 1; dy++ {
                            maybe := terrain.GetTile(mapUse.Map.Terrain[mapUse.WrapX(x+dx)][y+dy])
                            if maybe.TerrainType() == terrain.Shore {
                                found = true
                            }
                        }
                    }

                    if found {
                        candidates = append(candidates, point)
                    }
                }
            }

            if len(candidates) > 0 {
                choice := rand.N(len(candidates))
                return candidates[choice].X, candidates[choice].Y, true
            }
        }
    }

    return 0, 0, false
}

func (model *GameModel) DumpInfo(x int, y int) {
    log.Printf("Info at (%v,%v):", x, y)
    city, owner := model.FindCity(x, y, model.Plane)
    if city != nil {
        log.Printf("City owned by %v", owner.Wizard.Name)
        log.Printf(" Population: %v", city.Population)
        log.Printf(" Buildings:")
        for _, building := range city.Buildings.Values() {
            log.Printf("  - %v", model.BuildingInfo.Name(building))
        }
        log.Printf(" Producing building: %v", model.BuildingInfo.Name(city.ProducingBuilding))
        log.Printf(" Producing unit: %v", city.ProducingUnit.Name)
    }

    stack, stackOwner := model.FindStack(x, y, model.Plane)

    if stack != nil {
        if stackOwner == nil {
            log.Printf("Stack owner is nil!")
        } else {
            log.Printf("Stack owned by %v", stackOwner.Wizard.Name)
            roadWork := stackOwner.RoadWorkArcanus
            if stack.Plane() == data.PlaneMyrror {
                roadWork = stackOwner.RoadWorkMyrror
            }

            work, ok := roadWork[image.Pt(x, y)]
            if ok {
                log.Printf(" Road work done: %v", work)
            } else {
                log.Printf(" No road work")
            }
        }
    }
}

// raises 4 to 6 volcanoes on random tiles
func (model *GameModel) DoArmageddon() {
    info := model.ComputeCityStackInfo()
    for _, player := range model.Players {
        if player.GlobalEnchantments.Contains(data.EnchantmentArmageddon) {
            // get a list of valid map tiles on both planes
            var points []data.PlanePoint
            catchment := player.GetAllCatchmentArea()
            mapObjects := []*maplib.Map{model.ArcanusMap, model.MyrrorMap}
            for _, mapObject := range mapObjects {
                for x := range mapObject.Map.Columns() {
                    for y := range mapObject.Map.Rows() {
                        point := data.PlanePoint{X: x, Y: y, Plane: mapObject.Plane}
                        tile := terrain.GetTile(mapObject.Map.Terrain[x][y])
                        if tile.IsWater() || tile.IsRiver() || mapObject.HasVolcano(x, y) || mapObject.HasMagicNode(x, y) || catchment.Contains(point) {
                            continue
                        }

                        city := info.FindCity(x, y, mapObject.Plane)
                        if city != nil && (city.HasEnchantment(data.CityEnchantmentConsecration) || city.HasEnchantment(data.CityEnchantmentChaosWard)) {
                            continue
                        }

                        points = append(points, point)
                    }
                }
            }

            // create 4 to 6 volcanoes
            for _, index := range rand.Perm(len(points))[:min(len(points), 4 + rand.IntN(2))] {
                point := points[index]
                model.GetMap(point.Plane).SetVolcano(point.X, point.Y, player)
            }
        }
    }
}

// corrupts 3-6 random tiles
func (model *GameModel) DoGreatWasting() {
    info := model.ComputeCityStackInfo()
    for _, player := range model.Players {
        if player.GlobalEnchantments.Contains(data.EnchantmentGreatWasting) {
            // get a list of valid map tiles on both planes
            var points []data.PlanePoint
            catchment := player.GetAllCatchmentArea()
            mapObjects := []*maplib.Map{model.ArcanusMap, model.MyrrorMap}
            for _, mapObject := range mapObjects {
                for x := range mapObject.Map.Columns() {
                    for y := range mapObject.Map.Rows() {
                        point := data.PlanePoint{X: x, Y: y, Plane: mapObject.Plane}
                        tile := terrain.GetTile(mapObject.Map.Terrain[x][y])
                        if tile.IsWater() || tile.IsRiver() || mapObject.HasCorruption(x, y) || catchment.Contains(point) {
                            continue
                        }

                        city := info.FindCity(x, y, mapObject.Plane)
                        if city != nil && (city.HasEnchantment(data.CityEnchantmentConsecration) || city.HasEnchantment(data.CityEnchantmentChaosWard)) {
                            continue
                        }

                        points = append(points, point)
                    }
                }
            }

            // corrupt 3 to 6 tiles
            for _, index := range rand.Perm(len(points))[:min(len(points), 3 + rand.IntN(3))] {
                point := points[index]
                model.GetMap(point.Plane).SetCorruption(point.X, point.Y)
            }
        }
    }
}

// 5% chance to destroy a building in every city, and 15% chance to destroy a garrisoned unit
func (model *GameModel) MaybeDoNaturesWrath(caster *playerlib.Player) {
    if model.HasRivalEnchantment(caster, data.EnchantmentNaturesWrath) {
        for _, city := range caster.Cities {
            var toRemove []buildinglib.Building
            for _, building := range city.Buildings.Values() {
                if rand.N(100) < 5 {
                    toRemove = append(toRemove, building)
                }
            }

            // FIXME: emit a message for each building that was destroyed
            for _, building := range toRemove {
                log.Printf("Natures wrath: destroy building %v", building)
                city.RemoveBuilding(building)
            }

            stack := caster.FindStack(city.X, city.Y, city.Plane)
            var removeUnits []units.StackUnit
            if stack != nil {
                for _, unit := range stack.Units() {
                    if rand.N(100) < 15 {
                        log.Printf("Natures wrath: destroy unit %v", unit.GetName())
                        removeUnits = append(removeUnits, unit)
                    }
                }
            }

            for _, unit := range removeUnits {
                caster.RemoveUnit(unit)
            }
        }
    }
}

func (model *GameModel) CheckDisband(player *playerlib.Player) (bool, bool, bool) {

    unitsNeedGold := false
    unitsNeedFood := false
    unitsNeedMana := false

    for unit := range player.Units() {
        // dont need to keep checking in this case
        if unitsNeedGold && unitsNeedFood && unitsNeedMana {
            break
        }

        unitsNeedGold = unitsNeedGold || unit.GetUpkeepGold() > 0
        unitsNeedFood = unitsNeedFood || unit.GetUpkeepFood() > 0
        unitsNeedMana = unitsNeedMana || unit.GetUpkeepMana() > 0
    }

    goldPerTurn := player.GoldPerTurn()
    goldIssue := player.Gold + goldPerTurn < 0 && unitsNeedGold
    foodIssue := player.FoodPerTurn() < 0 && unitsNeedFood

    // FIXME: can the power be passed in so it doesn't have to be computed multiple times?
    manaPerTurn := player.ManaPerTurn(model.ComputePower(player), model)

    manaIssue := player.Mana + manaPerTurn < 0 && unitsNeedMana

    return goldIssue, foodIssue, manaIssue
}

func (model *GameModel) GetCombatLandscape(x int, y int, plane data.Plane) combat.CombatLandscape {
    tile := model.GetMap(plane).GetTile(x, y)

    switch tile.Tile.TerrainType() {
        case terrain.Hill, terrain.Grass,
             terrain.Forest, terrain.River, terrain.Shore,
             terrain.Swamp: return combat.CombatLandscapeGrass

        case terrain.Desert: return combat.CombatLandscapeDesert
        case terrain.Mountain: return combat.CombatLandscapeMountain
        case terrain.Tundra: return combat.CombatLandscapeTundra

        // FIXME: these cases are special
        case terrain.Ocean: return combat.CombatLandscapeWater
        case terrain.Volcano: return combat.CombatLandscapeGrass
        case terrain.Lake: return combat.CombatLandscapeGrass
        case terrain.NatureNode: return combat.CombatLandscapeGrass
        case terrain.SorceryNode: return combat.CombatLandscapeGrass
        case terrain.ChaosNode: return combat.CombatLandscapeMountain
    }

    return combat.CombatLandscapeGrass
}

// the turn as a readable date, such as June 1450
func (model *GameModel) TurnDate() string {
    base := uint64(1400)

    month := model.TurnNumber % 12
    years := model.TurnNumber / 12

    monthNames := []string{
        "January",
        "February",
        "March",
        "April",
        "May",
        "June",
        "July",
        "August",
        "September",
        "October",
        "November",
        "December",
    }

    return fmt.Sprintf("%s %v", monthNames[month], base + years)
}

func ComputeUnitBuildPowers(stack *playerlib.UnitStack) UnitBuildPowers {
    var powers UnitBuildPowers

    for _, check := range stack.ActiveUnits() {
        if check.HasAbility(data.AbilityCreateOutpost) {
            powers.CreateOutpost = true
        }

        if check.HasAbility(data.AbilityMeld) {
            powers.Meld = true
        }

        if check.HasAbility(data.AbilityConstruction) {
            powers.BuildRoad = true
        }

        if check.HasAbility(data.AbilityPurify) {
            powers.Purify = true
        }
    }

    return powers
}

// given a list of allNames 'A', 'B', 'C', 'A 1', 'B 1', and a list of choices 'A', 'B', 'C'
// choose the next name that is not in allNames but possibly with some monotonically increasing counter
// In the above example we would choose 'C 1'
func ChooseCityName(allNames []string, choices []string) string {

    // try to find a unused city name
    for _, i := range rand.Perm(len(choices)) {
        name := choices[i]
        if !slices.Contains(allNames, name) {
            return name
        }
    }

    // find a name by appending some number to a predefined choice
    suffix := 1
    for {
        for _, i := range rand.Perm(len(choices)) {
            name := fmt.Sprintf("%s %v", choices[i], suffix)
            if !slices.Contains(allNames, name) {
                return name
            }
        }

        suffix += 1
    }
}

func (model *GameModel) ComputeWizardPower(player *playerlib.Player) playerlib.WizardPower {
    armyStrength := 0

    for _, stack := range player.Stacks {
        for _, unit := range stack.Units() {
            armyStrength += ComputeUnitPower(unit)
        }
    }

    manaPerTurn := player.ManaPerTurn(model.ComputePower(player), model)

    research := 0
    for _, spell := range player.KnownSpells.Spells {
        research += spell.ResearchCost
    }

    return playerlib.WizardPower{
        Army: armyStrength / 2,
        Magic: max(0, player.Mana / 2 + manaPerTurn),
        SpellResearch: research / 20,
    }
}

func (model *GameModel) FindValidCityLocation(plane data.Plane) (int, int, bool) {
    mapUse := model.GetMap(plane)
    continents := mapUse.Map.FindContinents()

    for i := 0; i < 10; i++ {
        continentIndex := rand.IntN(len(continents))
        continent := continents[continentIndex]
        if continent.Size() > 100 {
            points := continent.Values()
            index := rand.IntN(len(points))
            x := points[index].X
            y := points[index].Y

            tile := terrain.GetTile(mapUse.Map.Terrain[x][y])
            if y > 3 && y < mapUse.Map.Rows() - 3 && tile.IsLand() && !tile.IsMagic() && mapUse.GetEncounter(x, y) == nil && !model.ContainsCity(x, y, plane) {
                return x, y, true
            }
        }
    }

    return 0, 0, false
}

// tryMeldNode melds the magic node the given stack is standing on, if the stack
// contains a unit able to meld (a magic spirit) and the node is unowned by this
// player and not warped. Returns true if a meld happened.
func (model *GameModel) TryMeldNode(stack *playerlib.UnitStack, player *playerlib.Player) bool {
    useMap := model.GetMap(stack.Plane())
    if useMap == nil {
        return false
    }
    if useMap.GetEncounter(stack.X(), stack.Y()) != nil {
        return false
    }
    node := useMap.GetMagicNode(stack.X(), stack.Y())
    if node == nil || node.Warped || node.MeldingWizard == player {
        return false
    }
    for _, unit := range stack.Units() {
        if unit.HasAbility(data.AbilityMeld) {
            if model.DoMeld(unit, player, node) {
                return true
            }
        }
    }
    return false
}

func (model *GameModel) MakeCityValidArea(plane data.Plane) CityValidArea {
    out := make(CityValidArea)

    mapUse := model.GetMap(plane)
    continents := mapUse.Map.FindContinents()

    for _, continent := range continents {
        if continent.Size() > 100 {
            for _, point := range continent.Values() {
                tile := terrain.GetTile(mapUse.Map.Terrain[point.X][point.Y])
                if point.Y > 3 && point.Y < mapUse.Map.Rows() - 3 && tile.IsLand() && !tile.IsMagic() && mapUse.GetEncounter(point.X, point.Y) == nil {
                    out[point] = true
                }
            }
        }
    }

    return out
}

func (model *GameModel) ApplyChaosChannels(unit *units.OverworldUnit) {
    choices := set.NewSet(
        data.UnitEnchantmentChaosChannelsDemonSkin,
        data.UnitEnchantmentChaosChannelsDemonWings,
        data.UnitEnchantmentChaosChannelsFireBreath,
    )

    if unit.IsFlying() {
        choices.Remove(data.UnitEnchantmentChaosChannelsDemonWings)
    }

    if unit.GetAbilityValue(data.AbilityFireBreath) > 0 {
        choices.Remove(data.UnitEnchantmentChaosChannelsFireBreath)
    }

    slice := choices.Values()
    choice := slice[rand.N(len(slice))]
    unit.AddEnchantment(choice)
}

func (model *GameModel) FindValidCityLocationOnContinent(plane data.Plane, x int, y int) (int, int) {
    mapUse := model.GetMap(plane)
    continents := mapUse.Map.FindContinents()

    for _, continent := range continents {
        if continent.Contains(image.Pt(x, y)) {
            points := continent.Values()
            for _, index := range rand.Perm(len(points)) {
                tile := mapUse.GetTile(points[index].X, points[index].Y)
                if tile.Tile.IsLand() && !tile.Tile.IsMagic() {
                    return points[index].X, points[index].Y
                }
            }
        }
    }

    return 0, 0
}

// the amount of experience a unit in a stack should get at the end of each turn
// if there is a hero in the stack then the hero's armsmaster ability applies
func (model *GameModel) GetExperienceBonus(stack *playerlib.UnitStack) int {
    base := 1
    bonus := 0

    // only the highest armsmaster bonus applies
    for _, unit := range stack.Units() {
        if unit.GetRace() == data.RaceHero {
            hero := unit.(*herolib.Hero)
            more := hero.GetAbilityExperienceBonus()
            if more > bonus {
                bonus = more
            }
        }
    }

    return base + bonus
}

func (model *GameModel) CastingDetectableByHuman(caster *playerlib.Player) bool {
    for _, player := range model.Players {
        if player.IsHuman() {
            if player.GlobalEnchantments.Contains(data.EnchantmentDetectMagic) {
                for _, enemy := range player.GetKnownPlayers() {
                    if enemy == caster {
                        return true
                    }
                }
            }
        }
    }
    return false
}

func (model *GameModel) RevertVolcanos() {
    mapObjects := []*maplib.Map{model.ArcanusMap, model.MyrrorMap}
    for _, mapObject := range mapObjects {
        for location, _ := range mapObject.ExtraMap {
            if mapObject.HasVolcano(location.X, location.Y) {
                if rand.N(100) < 2 {
                    mapObject.RemoveVolcano(location.X, location.Y)
                }
            }
        }
    }
}

func (model *GameModel) CityProductionBonus(x int, y int, plane data.Plane) int {
    mapUse := model.GetMap(plane)
    catchment := mapUse.GetCatchmentArea(x, y)

    production := 0

    for _, tile := range catchment {
        production += tile.ProductionBonus(false)
    }

    return production
}

// stolen from combat/model.go
func ComputeUnitPower(unit units.StackUnit) int {
    power := 0

    power += unit.GetMaxHealth()
    power += unit.GetDefense()
    power += unit.GetResistance()
    power += unit.GetRangedAttackPower() * unit.GetCount()
    power += unit.GetMeleeAttackPower() * unit.GetCount()

    return power
}

// enemy wizards, but not including the raider ai
func (model *GameModel) GetEnemyWizards() []*playerlib.Player {
    var out []*playerlib.Player

    for _, player := range model.Players {
        if !player.IsHuman() && player.Wizard.Banner != data.BannerBrown {
            out = append(out, player)
        }
    }

    return out
}

// FIXME: probably make this an iterator
func (model *GameModel) AllUnits() []units.StackUnit {
    var out []units.StackUnit

    for _, player := range model.Players {
        for unit := range player.Units() {
            out = append(out, unit)
        }
    }

    return out
}

func HandleStasis(stack *playerlib.UnitStack) {
    // this seems like a reasonable place to handle stasis, but it could be moved elsewhere
    for _, unit := range stack.Units() {
        if unit.GetBusy() == units.BusyStatusStasis {
            if rand.N(10) + 1 < combat.GetResistanceFor(unit, data.SorceryMagic) - 5 {
                unit.SetBusy(units.BusyStatusNone)
            }
        }
    }
}

// get the kind of magic that is influencing the given tile
func (model *GameModel) GetInfluenceMagic(x int, y int, plane data.Plane) data.MagicType {
    map_ := model.GetMap(plane)

    node := map_.GetMagicInfluence(x, y)
    if node != nil {
        return node.Kind.MagicType()
    }

    return data.MagicNone
}

func (model *GameModel) GetCityOwner(city *citylib.City) *playerlib.Player {
    for _, player := range model.Players {
        if player.OwnsCity(city) {
            return player
        }
    }

    return nil
}

func (model *GameModel) GetStackOwner(stack *playerlib.UnitStack) *playerlib.Player {
    for _, player := range model.Players {
        if player.OwnsStack(stack) {
            return player
        }
    }

    return nil
}


func (model *GameModel) DoMeld(unit units.StackUnit, player *playerlib.Player, node *maplib.ExtraMagicNode) bool {
    melded := node.Meld(player, unit.GetRawUnit())
    // whether melding succeeded or not the melding unit is removed
    player.RemoveUnit(unit)
    return melded
}

func (model *GameModel) CityGoldBonus(x int, y int, plane data.Plane) int {
    mapObject := model.GetMap(plane)
    tile := mapObject.GetTile(x, y)
    return tile.GoldBonus(mapObject)
}

func (model *GameModel) OnWaterTile(stack *playerlib.UnitStack) bool {
    mapUse := model.GetMap(stack.Plane())
    tile := mapUse.GetTile(stack.X(), stack.Y())
    return tile.Tile.IsWater()
}

type UnitBuildPowers struct {
    CreateOutpost bool
    Meld bool
    BuildRoad bool
    Purify bool
}

type CityValidArea map[image.Point]bool

func (area CityValidArea) FindLocation() (int, int, bool) {
    choices := make([]image.Point, 0, len(area))
    for point, ok := range area {
        if ok {
            choices = append(choices, point)
        }
    }

    if len(choices) == 0 {
        return 0, 0, false
    }

    point := choices[rand.N(len(choices))]
    return point.X, point.Y, true
}

func (model *GameModel) MaybeHireMercenaries(player *playerlib.Player) {
    if model.TurnNumber <= 30 {
        return
    }

    fortressCity := player.FindFortressCity()
    if fortressCity == nil {
        return  // player is being banished
    }

    // chance to create an event
    chance := 1 + player.GetFame() / 20
    if player.Wizard.RetortEnabled(data.RetortFamous) {
        chance *= 2
    }
    if chance > 10 {
        chance = 10
    }

    if rand.N(100) >= chance {
        return
    }

    // unit type
    presentOnArcanus := false
    presentOnMyrror := false
    for _, city := range player.Cities {
        if city.Plane == data.PlaneArcanus {
            presentOnArcanus = true
        }
        if city.Plane == data.PlaneMyrror {
            presentOnMyrror = true
        }
    }

    var unitCandidates []*units.Unit
    for _, unit := range units.AllUnits {
        if unit.Race == data.RaceFantastic || unit.Race == data.RaceHero {
            continue
        }

        if unit.IsSettlers() {
            continue
        }

        if unit.Name == "Trireme" || unit.Name == "Galley" || unit.Name == "Warship" {
            continue
        }

        isFromPresentPlane := false
        if presentOnArcanus {
            for _, race := range data.ArcanianRaces() {
                if unit.Race == race {
                    isFromPresentPlane = true
                    break
                }
            }
        }
        if !isFromPresentPlane && presentOnMyrror {
            for _, race := range data.MyrranRaces() {
                if unit.Race == race {
                    isFromPresentPlane = true
                    break
                }
            }
        }
        if !isFromPresentPlane {
            continue
        }

        unitCandidates = append(unitCandidates, &unit)
    }
    if len(unitCandidates) == 0 {
        return
    }

    unit := unitCandidates[rand.IntN(len(unitCandidates))]

    // number of units
    count := 1
    countRoll := rand.N(100) + player.GetFame()
    switch {
        case countRoll > 90: count = 3
        case countRoll > 60: count = 2
    }

    // experience
    level := 1
    experience := 20
    experienceRoll := rand.N(100) + player.GetFame()
    switch {
        case experienceRoll > 90:
            level = 3
            experience = 120
        case experienceRoll > 60:
            level = 2
            experience = 60
    }

    // cost
    cost := count * unit.ProductionCost * (level + 3) / 2
    if player.Wizard.RetortEnabled(data.RetortCharismatic) {
        cost /= 2
    }
    if player.Gold < cost {
        return
    }

    // create units
    var overworldUnits []*units.OverworldUnit
    for range count {
        overworldUnit := units.MakeOverworldUnitFromUnit(*unit, fortressCity.X, fortressCity.Y, fortressCity.Plane, player.Wizard.Banner, player.MakeExperienceInfo(), player.MakeUnitEnchantmentProvider())
        overworldUnit.Experience = experience
        overworldUnits = append(overworldUnits, overworldUnit)
    }

    model.SendOffer(&GameEventHireMercenaries{Cost: cost, Units: overworldUnits, Player: player})
}

func (model *GameModel) MaybeBuyFromMerchant(player *playerlib.Player) {
    // chance to create an event
    chance := 2 + player.GetFame() / 25
    if player.Wizard.RetortEnabled(data.RetortFamous) {
        chance *= 2
    }
    if chance > 10 {
        chance = 10
    }

    if rand.N(100) >= chance {
        return
    }

    var artifactCandidates []*artifact.Artifact
    for _, candidate := range model.ArtifactPool.AvailableArtifacts() {
        requirementsMet := true
        for _, requirement := range candidate.Requirements {
            if requirement.Amount > 12 {
                requirementsMet = false
                break
            }
        }
        if !requirementsMet {
            continue
        }

        artifactCandidates = append(artifactCandidates, candidate)
    }
    if len(artifactCandidates) == 0 {
        return
    }

    artifact := artifactCandidates[rand.IntN(len(artifactCandidates))]

    // cost
    cost := artifact.Cost
    if player.Wizard.RetortEnabled(data.RetortCharismatic) {
        cost /= 2
    }
    if player.Gold < cost {
        return
    }

    model.SendOffer(&GameEventMerchant{Cost: cost, Artifact: artifact, Player: player})
}

func (model *GameModel) MaybeHireHero(player *playerlib.Player) {
    if len(player.AliveHeroes()) >= 6 {
        return
    }
    // chance as an integer between 0-100

    // every 25 fame increases chance by 1
    // every hero reduces chance by a fraction (1 hero = halve chance. 2 heroes = 1/3 chance)
    chance := (3 + player.GetFame() / 25) / ((len(player.AliveHeroes()) + 3) / 2)
    if player.Wizard.RetortEnabled(data.RetortFamous) {
        chance *= 2
    }

    if chance > 10 {
        chance = 10
    }

    if rand.N(100) < chance {
        var heroCandidates []*herolib.Hero
        for _, hero := range player.HeroPool {
            // torin can never be hired
            if hero.HeroType == herolib.HeroTorin {
                continue
            }

            if hero.Status == herolib.StatusAvailable {
                if hero.GetRequiredFame() <= player.GetFame() {
                    heroCandidates = append(heroCandidates, hero)
                }
            }
        }

        if len(heroCandidates) > 0 {
            hero := heroCandidates[rand.N(len(heroCandidates))]

            fee := hero.GetHireFee()
            if fee > player.Gold {
                // hero gains a level if the player can't afford to hire them
                hero.GainLevel(units.ExperienceChampionHero)
            } else {
                model.SendOffer(&GameEventHireHero{Cost: fee, Hero: hero, Player: player})
            }
        }
    }
}

// show all scroll events for this turn, or a message that no events occurred
func (model *GameModel) DoChancellor() {
    if len(model.ScrollEvents) == 0 {
        event := &GameEventScroll{
            Title: "NO EVENTS THIS MONTH",
        }
        select {
            case model.Events <- event:
            default:
        }
    } else {
        for _, event := range model.ScrollEvents {
            event.Old = true
            select {
                case model.Events <- event:
                default:
            }
        }
    }
}

func (model *GameModel) AddExperience(player *playerlib.Player, unit units.StackUnit, amount int) {
    if player.IsHuman() && unit.IsHero() {
        level_before := unit.GetHeroExperienceLevel()

        unit.AddExperience(amount)

        level_after := unit.GetHeroExperienceLevel()

        if level_before != level_after {
            hero := unit.(*herolib.Hero)
            model.Events <- &GameEventHeroLevelUp{
                Hero: hero,
            }
        }
    } else {
        unit.AddExperience(amount)
    }
}


// FIXME: add a "reason" of fizzling, like "Spell X was fizzled because of Y"
func (model *GameModel) ShowFizzleSpell(spell spellbook.Spell, caster *playerlib.Player) {
    if caster.IsHuman() {
        model.Events <- &GameEventNotice{
            // FIXME: align this message with how dos mom does it
            Message: fmt.Sprintf("The spell %v has fizzled.", spell.Name),
        }
    }
}
