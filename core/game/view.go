package game

import (
    "cmp"
    "encoding/json"
    "image"
    "slices"

    "github.com/bfalese/civ-of-magic/core/assets"
    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/maplib"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/units"
)

// Snapshots of the game as one player sees it, as plain data for JSON. A frontend that doesn't
// share memory with the model (wasm bridge, network) draws from these and asks for a new one
// after a "refresh" event. Only what the player can see is included: their own cities and stacks
// in full, other players' cities and stacks only where the player's fog shows them.

type WizardView struct {
    Player PlayerRef `json:"player"`
    Gold int `json:"gold"`
    GoldPerTurn int `json:"gold-per-turn"`
    Food int `json:"food-per-turn"`
    Mana int `json:"mana"`
    ManaPerTurn int `json:"mana-per-turn"`
    Fame int `json:"fame"`
    TaxRate string `json:"tax-rate"`
    CastingSkill int `json:"casting-skill"`
    Researching string `json:"researching,omitempty"`
    ResearchProgress int `json:"research-progress"`
    ResearchCost int `json:"research-cost"`
    Casting string `json:"casting,omitempty"`
    CastingProgress int `json:"casting-progress"`
    KnownSpells []string `json:"known-spells"`
    GlobalEnchantments []string `json:"global-enchantments,omitempty"`
    Banished bool `json:"banished,omitempty"`
    Defeated bool `json:"defeated,omitempty"`

    // the magic screen: power per turn and how it is shared (set-power-distribution)
    Power int `json:"power"`
    PowerDistribution PowerDistributionView `json:"power-distribution"`
    ResearchPerTurn int `json:"research-per-turn"`
    SkillPerTurn int `json:"skill-per-turn"`
    // casting skill left this turn: a spell is cast at once if it costs no more than this and mana
    RemainingCastingSkill int `json:"remaining-casting-skill"`
    // the full cost of the spell being cast over several turns
    CastingCost int `json:"casting-cost,omitempty"`
    // the spells set-research may pick
    ResearchCandidates []string `json:"research-candidates"`
    // the spellbook: overland spells cast-spell takes
    Spells []SpellView `json:"spells"`
    // alchemy converts at full rate (the Alchemy retort), otherwise at half
    AlchemyFullRate bool `json:"alchemy-full-rate,omitempty"`
    // the item designer's options, when the spellbook has Create Artifact or Enchant Item
    ItemDesign *ItemDesignView `json:"item-design,omitempty"`
    // the vault, newly arrived items and the heroes' equipment
    Items VaultView `json:"items"`
}

type PowerDistributionView struct {
    Mana float64 `json:"mana"`
    Research float64 `json:"research"`
    Skill float64 `json:"skill"`
}

// an overland spell in the spellbook
type SpellView struct {
    Name string `json:"name"`
    // the realm: Life, Death, Chaos, Nature, Sorcery or Arcane
    Magic string `json:"magic"`
    // Summoning, Unit Enchantments, City Enchantments, ...
    Section string `json:"section"`
    Cost int `json:"cost"`
    // what cast-spell needs as its target (choose-spell-target's names: friendly-city, enemy-unit,
    // wizard, ...), "" for none
    Target string `json:"target,omitempty"`
    // cast at once now (mana and the casting skill left this turn cover it); otherwise cast-spell
    // starts casting it over the next turns
    Now bool `json:"now"`
    // the item spells take a design (cast-spell's artifact): the player view's item-design
    NeedsDesign bool `json:"needs-design,omitempty"`
    // for item spells: the share of the item's cost the player pays (retorts, spellbooks)
    CostFactor float64 `json:"cost-factor,omitempty"`
    // spells cast with a choice (Spell Ward, Spell Binding, Disjunction, Resurrection): the
    // targets cast-spell accepts now (empty: nothing to cast it on)
    Choices []SpellChoiceView `json:"choices,omitempty"`
    ChoiceSpell bool `json:"choice-spell,omitempty"`
}

type CityView struct {
    CityRef
    // the city's sprite on the map
    Asset string `json:"asset"`
    Population int `json:"population"`
    Citizens int `json:"citizens"`
    Farmers int `json:"farmers"`
    Workers int `json:"workers"`
    Rebels int `json:"rebels"`
    MaximumSize int `json:"maximum-size"`
    FoodSurplus int `json:"food-surplus"`
    ProductionPerTurn float32 `json:"production-per-turn"`
    GoldSurplus int `json:"gold-surplus"`
    Power int `json:"power"`
    Research int `json:"research"`
    Outpost bool `json:"outpost,omitempty"`
    Producing string `json:"producing"`
    Production float32 `json:"production"`
    ProductionCost int `json:"production-cost"`
    BuyCost int `json:"buy-cost"`
    SoldBuildingThisTurn bool `json:"sold-building-this-turn,omitempty"`
    // the fewest farmers that feed the city (set-city-farmers refuses fewer)
    SubsistenceFarmers int `json:"subsistence-farmers"`
    // what set-city-production accepts: buildings (prerequisites met, plus Trade Goods and
    // Housing) in the game's order, then units by name
    CanBuild []ProductionOption `json:"can-build"`
    // gold each building sells for (only buildings that can be sold)
    SellValues map[string]int `json:"sell-values"`
    Buildings []string `json:"buildings"`
    Enchantments []string `json:"enchantments,omitempty"`
}

type UnitView struct {
    // the unit's id, for commands (see GameModel.UnitId)
    Id int `json:"id"`
    Name string `json:"name"`
    Health int `json:"health"`
    MaxHealth int `json:"max-health"`
    MovesLeft float64 `json:"moves-left"`
    Hero bool `json:"hero,omitempty"`
    // the overland sprite (core/assets)
    Asset string `json:"asset"`
    // moves with the stack (set-unit-active); inactive units stay behind when it moves
    Active bool `json:"active"`
    Details UnitDetailsView `json:"details"`
}

type StackView struct {
    Location Location `json:"location"`
    // the square drawn behind the stack's leader (the first unit)
    Background string `json:"background"`
    Units []UnitView `json:"units"`
    // remaining path, if the stack is walking somewhere
    Path []Location `json:"path,omitempty"`
}

// another player's city or stack, as far as the player can see it
type ForeignView struct {
    Owner *PlayerRef `json:"owner"`
    Location Location `json:"location"`
    // city name, or the first unit's name for a stack
    Name string `json:"name"`
    // citizens for a city, units for a stack
    Size int `json:"size"`
    // the city's sprite, or the leading unit's
    Asset string `json:"asset"`
    // the square behind a stack's leader
    Background string `json:"background,omitempty"`
}

type PlayerView struct {
    Turn uint64 `json:"turn"`
    Date string `json:"date"`
    CurrentPlayer *PlayerRef `json:"current-player"`
    You WizardView `json:"you"`
    // the wizards this player knows about
    KnownWizards []PlayerRef `json:"known-wizards"`
    Cities []CityView `json:"cities"`
    Stacks []StackView `json:"stacks"`
    ForeignCities []ForeignView `json:"foreign-cities"`
    ForeignStacks []ForeignView `json:"foreign-stacks"`
}

func (model *GameModel) makeCityView(city *citylib.City) CityView {
    view := CityView{
        CityRef: *MakeCityRef(city),
        Asset: cityAsset(city),
        Population: city.Population,
        Citizens: city.Citizens(),
        Farmers: city.Farmers,
        Workers: city.Workers,
        Rebels: city.Rebels,
        MaximumSize: city.MaximumCitySize(),
        FoodSurplus: city.SurplusFood(),
        ProductionPerTurn: city.WorkProductionRate(),
        GoldSurplus: city.GoldSurplus(),
        Power: city.ComputePower(),
        Research: city.ResearchProduction(),
        Outpost: city.Outpost,
        Production: city.Production,
        SoldBuildingThisTurn: city.SoldBuilding,
        Buildings: []string{},
    }

    view.BuyCost, _ = city.BuyCost()
    view.SubsistenceFarmers = city.ComputeSubsistenceFarmers()
    view.CanBuild = model.productionOptions(city)
    view.SellValues = make(map[string]int)
    for _, building := range city.Buildings.Values() {
        if city.CanSellBuilding(building) {
            view.SellValues[model.BuildingInfo.Name(building)] = city.SellValue(building)
        }
    }

    if !city.ProducingUnit.IsNone() {
        view.Producing = city.ProducingUnit.Name
        view.ProductionCost = city.UnitProductionCost(&city.ProducingUnit)
    } else if city.ProducingBuilding != buildinglib.BuildingNone {
        view.Producing = model.BuildingInfo.Name(city.ProducingBuilding)
        view.ProductionCost = model.BuildingInfo.ProductionCost(city.ProducingBuilding)
    }

    for _, building := range sortedBuildings(city) {
        view.Buildings = append(view.Buildings, model.BuildingInfo.Name(building))
    }

    for _, enchantment := range city.Enchantments.Values() {
        view.Enchantments = append(view.Enchantments, enchantment.Enchantment.Name())
    }

    return view
}

func sortedBuildings(city *citylib.City) []buildinglib.Building {
    buildings := city.Buildings.Values()
    slices.Sort(buildings)
    return buildings
}

func cityAsset(city *citylib.City) string {
    return assets.City(city.GetSize(), city.HasWall(), city.GetBanner())
}

func unitAsset(unit units.StackUnit) string {
    raw := unit.GetRawUnit()
    return assets.Unit(raw.Race, raw.Name, unit.GetBanner())
}

func (model *GameModel) makeStackView(stack *playerlib.UnitStack) StackView {
    view := StackView{
        Location: Location{Plane: stack.Plane(), X: stack.X(), Y: stack.Y()},
        Units: []UnitView{},
    }
    if leader := stack.Leader(); leader != nil {
        view.Background = assets.UnitBackground(leader.GetBanner())
    }
    for _, unit := range stack.Units() {
        view.Units = append(view.Units, UnitView{
            Id: model.UnitId(unit),
            Name: unit.GetName(),
            Health: unit.GetHealth(),
            MaxHealth: unit.GetMaxHealth(),
            MovesLeft: unit.GetMovesLeft(true).ToFloat(),
            Hero: unit.IsHero(),
            Asset: unitAsset(unit),
            Active: stack.IsActive(unit),
            Details: makeUnitDetails(unit),
        })
    }
    for _, point := range stack.CurrentPath {
        view.Path = append(view.Path, Location{Plane: stack.Plane(), X: point.X, Y: point.Y})
    }
    return view
}

// the game as player sees it
func (model *GameModel) MakePlayerView(player *playerlib.Player) PlayerView {
    power := model.ComputePower(player)

    you := WizardView{
        Player: *MakePlayerRef(player),
        Gold: player.Gold,
        GoldPerTurn: player.GoldPerTurn(),
        Food: player.FoodPerTurn(),
        Mana: player.Mana,
        ManaPerTurn: player.ManaPerTurn(power, model),
        Fame: player.Fame,
        TaxRate: player.TaxRate.String(),
        CastingSkill: player.ComputeOverworldCastingSkill(),
        ResearchProgress: player.ResearchProgress,
        CastingProgress: player.CastingSpellProgress,
        Banished: player.Banished,
        Defeated: player.Defeated,
        KnownSpells: []string{},
    }
    if player.ResearchingSpell.Valid() {
        you.Researching = player.ResearchingSpell.Name
        you.ResearchCost = player.ResearchingSpell.ResearchCost
    }
    if player.CastingSpell.Valid() {
        you.Casting = player.CastingSpell.Name
    }
    for _, spell := range player.KnownSpells.Spells {
        you.KnownSpells = append(you.KnownSpells, spell.Name)
    }
    for _, enchantment := range player.GlobalEnchantments.Values() {
        you.GlobalEnchantments = append(you.GlobalEnchantments, enchantment.String())
    }

    you.Power = power
    you.PowerDistribution = PowerDistributionView{Mana: player.PowerDistribution.Mana, Research: player.PowerDistribution.Research, Skill: player.PowerDistribution.Skill}
    you.ResearchPerTurn = player.ComputeEffectiveResearchPerTurn(player.SpellResearchPerTurn(power), player.ResearchingSpell)
    you.SkillPerTurn = player.CastingSkillPerTurn(power)
    you.RemainingCastingSkill = player.RemainingCastingSkill
    if player.CastingSpell.Valid() {
        you.CastingCost = player.ComputeEffectiveSpellCost(player.CastingSpell, true)
    }
    you.AlchemyFullRate = player.Wizard.RetortEnabled(data.RetortAlchemy)
    you.Items = model.makeVaultView(player)
    you.ResearchCandidates = []string{}
    for _, spell := range player.ResearchCandidateSpells.Spells {
        you.ResearchCandidates = append(you.ResearchCandidates, spell.Name)
    }
    you.Spells = []SpellView{}
    for _, spell := range player.KnownSpells.OverlandSpells().Spells {
        if spell.Name == "Spell of Return" {
            continue
        }
        cost := player.ComputeEffectiveSpellCost(spell, true)
        spellView := SpellView{
            Name: spell.Name,
            Magic: spell.Magic.String(),
            Section: spell.Section.Name(),
            Cost: cost,
            Now: cost <= player.Mana && cost <= player.RemainingCastingSkill,
            NeedsDesign: spell.Name == "Create Artifact" || spell.Name == "Enchant Item",
        }
        if target, ok := model.SpellTargetType(player, spell); ok {
            spellView.Target = target.Name()
        }
        if spellView.NeedsDesign {
            spellView.CostFactor = itemCostFactor(player, spell)
            if you.ItemDesign == nil {
                you.ItemDesign = model.itemDesignView(player)
            }
        }
        if choices, ok := model.spellChoices(player, spell); ok {
            spellView.ChoiceSpell = true
            spellView.Choices = choices
        }
        you.Spells = append(you.Spells, spellView)
    }

    // lists are empty rather than null in JSON
    view := PlayerView{
        Turn: model.TurnNumber,
        Date: model.TurnDate(),
        You: you,
        KnownWizards: []PlayerRef{},
        Cities: []CityView{},
        Stacks: []StackView{},
        ForeignCities: []ForeignView{},
        ForeignStacks: []ForeignView{},
    }
    if model.CurrentPlayer >= 0 && model.CurrentPlayer < len(model.Players) {
        view.CurrentPlayer = MakePlayerRef(model.Players[model.CurrentPlayer])
    }

    for _, city := range player.Cities {
        view.Cities = append(view.Cities, model.makeCityView(city))
    }
    for _, stack := range player.Stacks {
        view.Stacks = append(view.Stacks, model.makeStackView(stack))
    }

    for _, other := range model.Players {
        if other == player {
            continue
        }
        if player.IsAwareOf(other) {
            view.KnownWizards = append(view.KnownWizards, *MakePlayerRef(other))
        }
        for _, city := range other.Cities {
            if player.IsExplored(city.X, city.Y, city.Plane) {
                view.ForeignCities = append(view.ForeignCities, ForeignView{
                    Owner: MakePlayerRef(other),
                    Location: Location{Plane: city.Plane, X: city.X, Y: city.Y},
                    Name: city.Name,
                    Size: city.Citizens(),
                    Asset: cityAsset(city),
                })
            }
        }
        for _, stack := range other.Stacks {
            if len(stack.Units()) > 0 && player.IsVisible(stack.X(), stack.Y(), stack.Plane()) {
                view.ForeignStacks = append(view.ForeignStacks, ForeignView{
                    Owner: MakePlayerRef(other),
                    Location: Location{Plane: stack.Plane(), X: stack.X(), Y: stack.Y()},
                    Name: stack.Units()[0].GetName(),
                    Size: len(stack.Units()),
                    Asset: unitAsset(stack.Leader()),
                    Background: assets.UnitBackground(stack.Leader().GetBanner()),
                })
            }
        }
    }

    return view
}

// one map tile as the player knows it
type TileView struct {
    Terrain string `json:"terrain"`
    // the terrain sprite (animated tiles have several frames in the asset manifest)
    Asset string `json:"asset"`
    // sprites drawn over the terrain, in order: road pieces, bonus, lair/node/tower, corruption,
    // a melded node's zone
    Overlays []string `json:"overlays,omitempty"`
    Road bool `json:"road,omitempty"`
    Bonus string `json:"bonus,omitempty"`
    Encounter string `json:"encounter,omitempty"`
    Corrupted bool `json:"corrupted,omitempty"`
    // true if the player sees the tile now (false: explored earlier, may be out of date)
    Visible bool `json:"visible,omitempty"`
}

type MapView struct {
    Plane data.Plane `json:"plane"`
    Width int `json:"width"`
    Height int `json:"height"`
    // Tiles[y][x]; nil for tiles the player hasn't explored
    Tiles [][]*TileView `json:"tiles"`
}

// the map of a plane as player has explored it
func (model *GameModel) MakeMapView(player *playerlib.Player, plane data.Plane) MapView {
    mapUse := model.GetMap(plane)
    view := MapView{
        Plane: plane,
        Width: mapUse.Width(),
        Height: mapUse.Height(),
    }

    // melded nodes colour their zone with the melder's sparkles
    zones := make(map[image.Point]data.BannerType)
    for point, extras := range mapUse.ExtraMap {
        if node, ok := extras[maplib.ExtraKindMagicNode].(*maplib.ExtraMagicNode); ok && node.MeldingWizard != nil {
            for _, offset := range node.Zone {
                zones[image.Pt(mapUse.WrapX(point.X + offset.X), point.Y + offset.Y)] = node.MeldingWizard.GetBanner()
            }
        }
    }

    for y := range mapUse.Height() {
        row := make([]*TileView, mapUse.Width())
        for x := range mapUse.Width() {
            if !player.IsExplored(x, y, plane) {
                continue
            }
            tile := mapUse.GetTile(x, y)
            tileView := &TileView{
                Terrain: tile.Tile.Name(),
                Asset: assets.TerrainTile(mapUse.Map.Terrain[x][y]),
                Road: tile.HasRoad(),
                Corrupted: tile.Corrupted(),
                Visible: player.IsVisible(x, y, plane),
            }
            if bonus := tile.GetBonus(); bonus != data.BonusNone {
                tileView.Bonus = bonus.String()
            }
            if encounter := mapUse.GetEncounter(x, y); encounter != nil {
                tileView.Encounter = encounter.Type.Name()
            }
            tileView.Overlays = tileOverlays(mapUse, x, y, zones)
            row[x] = tileView
        }
        view.Tiles = append(view.Tiles, row)
    }

    return view
}

func (model *GameModel) EncodePlayerView(player *playerlib.Player) ([]byte, error) {
    return json.Marshal(model.MakePlayerView(player))
}

func (model *GameModel) EncodeMapView(player *playerlib.Player, plane data.Plane) ([]byte, error) {
    return json.Marshal(model.MakeMapView(player, plane))
}

// the sprites over a tile's terrain, in drawing order (see TileView.Overlays)
func tileOverlays(mapUse *maplib.Map, x int, y int, zones map[image.Point]data.BannerType) []string {
    var out []string
    extras := mapUse.ExtraMap[image.Pt(x, y)]

    if road, ok := extras[maplib.ExtraKindRoad].(*maplib.ExtraRoad); ok {
        neighbors := mapUse.GetRoadNeighbors(x, y)
        connected := false
        for _, direction := range []maplib.Direction{maplib.DirectionNorth, maplib.DirectionNorthEast, maplib.DirectionEast, maplib.DirectionSouthEast, maplib.DirectionSouth, maplib.DirectionSouthWest, maplib.DirectionWest, maplib.DirectionNorthWest} {
            if neighbors[direction] {
                out = append(out, assets.Road(assets.RoadDirection(direction), road.Enchanted))
                connected = true
            }
        }
        if !connected {
            out = append(out, assets.Road("center", road.Enchanted))
        }
    }

    _, corrupted := extras[maplib.ExtraKindCorruption]

    // corruption cancels the bonus (as TileView.Bonus)
    if bonus, ok := extras[maplib.ExtraKindBonus].(*maplib.ExtraBonus); ok && !corrupted && bonus.Bonus.LbxIndex() != -1 {
        out = append(out, assets.Bonus(bonus.Bonus))
    }

    if encounter, ok := extras[maplib.ExtraKindEncounter].(*maplib.ExtraEncounter); ok && encounter.Type.LbxIndex() != -1 {
        out = append(out, assets.Encounter(encounter.Type))
    } else if _, ok := extras[maplib.ExtraKindOpenTower]; ok {
        out = append(out, assets.OpenTower)
    }

    if corrupted {
        out = append(out, assets.Corruption)
    }

    if banner, ok := zones[image.Pt(x, y)]; ok {
        out = append(out, assets.NodeZone(banner))
    }

    return out
}

// something a city can produce
type ProductionOption struct {
    Name string `json:"name"`
    // a unit (otherwise a building)
    Unit bool `json:"unit,omitempty"`
    // production cost (0 for Trade Goods and Housing)
    Cost int `json:"cost"`
}

// the buildings a city may start now, in the game's building order (as the build screen)
func possibleBuildings(city *citylib.City) []buildinglib.Building {
    return slices.SortedFunc(slices.Values(city.ComputePossibleBuildings(false).Values()), func(a, b buildinglib.Building) int {
        return cmp.Compare(city.BuildingInfo.GetBuildingIndex(a), city.BuildingInfo.GetBuildingIndex(b))
    })
}

// what the city can produce: buildings, then units by name (the build screen's lists)
func (model *GameModel) productionOptions(city *citylib.City) []ProductionOption {
    out := []ProductionOption{}
    for _, building := range possibleBuildings(city) {
        out = append(out, ProductionOption{Name: model.BuildingInfo.Name(building), Cost: model.BuildingInfo.ProductionCost(building)})
    }
    possibleUnits := city.ComputePossibleUnits()
    slices.SortFunc(possibleUnits, func(a, b units.Unit) int {
        return cmp.Compare(a.Name, b.Name)
    })
    for _, unit := range possibleUnits {
        out = append(out, ProductionOption{Name: unit.Name, Unit: true, Cost: city.UnitProductionCost(&unit)})
    }
    return out
}
