package game

import (
    "encoding/json"
    "slices"

    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
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
}

type CityView struct {
    CityRef
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
    Buildings []string `json:"buildings"`
    Enchantments []string `json:"enchantments,omitempty"`
}

type UnitView struct {
    Name string `json:"name"`
    Health int `json:"health"`
    MaxHealth int `json:"max-health"`
    MovesLeft float64 `json:"moves-left"`
    Hero bool `json:"hero,omitempty"`
}

type StackView struct {
    Location Location `json:"location"`
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

func makeStackView(stack *playerlib.UnitStack) StackView {
    view := StackView{
        Location: Location{Plane: stack.Plane(), X: stack.X(), Y: stack.Y()},
        Units: []UnitView{},
    }
    for _, unit := range stack.Units() {
        view.Units = append(view.Units, UnitView{
            Name: unit.GetName(),
            Health: unit.GetHealth(),
            MaxHealth: unit.GetMaxHealth(),
            MovesLeft: unit.GetMovesLeft(true).ToFloat(),
            Hero: unit.IsHero(),
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
        view.Stacks = append(view.Stacks, makeStackView(stack))
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
                })
            }
        }
    }

    return view
}

// one map tile as the player knows it
type TileView struct {
    Terrain string `json:"terrain"`
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

    for y := range mapUse.Height() {
        row := make([]*TileView, mapUse.Width())
        for x := range mapUse.Width() {
            if !player.IsExplored(x, y, plane) {
                continue
            }
            tile := mapUse.GetTile(x, y)
            tileView := &TileView{
                Terrain: tile.Tile.Name(),
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
