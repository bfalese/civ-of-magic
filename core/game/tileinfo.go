package game

import (
    "errors"
    "image"

    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/maplib"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
)

// What the surveyor tells about an explored tile: its terrain and bonuses, a node, lair or city on
// it, and whether a city could be founded there (and how good a site it is). The desktop surveyor
// and the web page's tile info both show this.
type TileInfoView struct {
    Location Location `json:"location"`
    // the terrain's name (a node shows the land it stands on: Forest, Grasslands, Mountain)
    Terrain string `json:"terrain"`
    Corrupted bool `json:"corrupted,omitempty"`
    // what the tile adds to a city working it: food, and production / gold percentages
    Food float64 `json:"food,omitempty"`
    // the food bonus as a fraction ("1/2", "1 1/2"), for display
    FoodText string `json:"food-text,omitempty"`
    ProductionBonus int `json:"production-bonus,omitempty"`
    GoldBonus int `json:"gold-bonus,omitempty"`
    Bonus *TileBonusView `json:"bonus,omitempty"`
    Node *TileNodeView `json:"node,omitempty"`
    Encounter *TileEncounterView `json:"encounter,omitempty"`
    // a city on the tile: "<size> of <name>"
    City string `json:"city,omitempty"`
    CityOwner *PlayerRef `json:"city-owner,omitempty"`
    Site CitySiteView `json:"site"`
}

// a mineral or other special on the tile
type TileBonusView struct {
    Name string `json:"name"`
    Food int `json:"food,omitempty"`
    Gold int `json:"gold,omitempty"`
    Power int `json:"power,omitempty"`
    // percent off normal units' cost
    UnitCostReduction int `json:"unit-cost-reduction,omitempty"`
}

type TileNodeView struct {
    // "Nature Node", "Sorcery Node", "Chaos Node"
    Kind string `json:"kind"`
    Warped bool `json:"warped,omitempty"`
    // who melded it, and with what ("Magic Spirit", "Guardian Spirit")
    MeldedBy *PlayerRef `json:"melded-by,omitempty"`
    Spirit string `json:"spirit,omitempty"`
}

type TileEncounterView struct {
    // the lair's kind ("Cave", "Ruins", ...; a node's guard keeps the node's name)
    Type string `json:"type"`
    // "Unexplored" until the player has looked in, then "Empty" or the first guard's name
    Guards string `json:"guards"`
}

// whether a city could be founded on the tile, and what it would have
type CitySiteView struct {
    CanBuild bool `json:"can-build"`
    // why not: water, too close to a city
    Reason string `json:"reason,omitempty"`
    MaximumPopulation int `json:"maximum-population,omitempty"`
    ProductionBonus int `json:"production-bonus,omitempty"`
    GoldBonus int `json:"gold-bonus,omitempty"`
}

var errUnexplored = errors.New("the tile isn't explored")

// the surveyor's view of a tile the player has explored
func (model *GameModel) MakeTileInfo(player *playerlib.Player, location Location) (TileInfoView, error) {
    mapObject := model.GetMap(location.Plane)
    if mapObject == nil || location.Y < 0 || location.Y >= mapObject.Height() {
        return TileInfoView{}, errors.New("no such tile")
    }
    x, y := mapObject.WrapX(location.X), location.Y
    location.X = x
    if !player.IsExplored(x, y, location.Plane) {
        return TileInfoView{}, errUnexplored
    }

    tile := mapObject.GetTile(x, y)
    node := mapObject.GetMagicNode(x, y)
    info := TileInfoView{
        Location: location,
        Terrain: tile.Name(mapObject),
        Corrupted: tile.Corrupted(),
        ProductionBonus: tile.ProductionBonus(false),
        GoldBonus: tile.GoldBonus(mapObject),
    }
    if node != nil {
        switch node.Kind {
            case maplib.MagicNodeNature: info.Terrain = "Forest"
            case maplib.MagicNodeSorcery: info.Terrain = "Grasslands"
            case maplib.MagicNodeChaos: info.Terrain = "Mountain"
        }
    }
    if food := tile.FoodBonus(); !food.IsZero() {
        info.Food = food.ToFloat()
        info.FoodText = food.NormalString()
    }

    if bonus := tile.GetBonus(); bonus != data.BonusNone {
        info.Bonus = &TileBonusView{
            Name: bonus.String(),
            Food: bonus.FoodBonus(),
            Gold: bonus.GoldBonus(),
            Power: bonus.PowerBonus(),
            UnitCostReduction: bonus.UnitReductionBonus(),
        }
    }

    if node != nil {
        info.Node = &TileNodeView{Kind: node.Kind.Name(), Warped: node.Warped}
        if node.MeldingWizard != nil && !node.Warped {
            if melder := model.GetPlayerByBanner(node.MeldingWizard.GetBanner()); melder != nil {
                info.Node.MeldedBy = MakePlayerRef(melder)
            }
            info.Node.Spirit = "Magic Spirit"
            if node.GuardianSpiritMeld {
                info.Node.Spirit = "Guardian Spirit"
            }
        }
    }

    if encounter := mapObject.GetEncounter(x, y); encounter != nil {
        guards := "Unexplored"
        if encounter.ExploredBy.Contains(player) {
            guards = "Empty"
            if len(encounter.Units) > 0 {
                guards = encounter.Units[0].Name
            }
        }
        info.Encounter = &TileEncounterView{Type: encounter.Type.Name(), Guards: guards}
    }

    if city, owner := model.FindCity(x, y, location.Plane); city != nil {
        info.City = city.String()
        if owner != nil {
            info.CityOwner = MakePlayerRef(owner)
        }
    }

    switch {
        case !tile.Tile.IsLand():
            info.Site.Reason = "Cannot build cities on water."
        case info.City == "" && model.NearCity(image.Pt(x, y), 3, location.Plane):
            info.Site.Reason = "Cities cannot be built less than 3 squares from any other city."
        default:
            // the resources as the desktop surveyor shows them (also for a city's own tile), and
            // whether settlers could found a city here now
            info.Site = CitySiteView{
                CanBuild: info.City == "" && model.IsSettlableLocation(x, y, location.Plane),
                MaximumPopulation: model.ComputeMaximumPopulation(x, y, location.Plane),
                ProductionBonus: model.CityProductionBonus(x, y, location.Plane),
                GoldBonus: model.CityGoldBonus(x, y, location.Plane),
            }
            if info.City == "" && !info.Site.CanBuild {
                info.Site.Reason = "Cities cannot be built on a node, a lair or corrupted land."
            }
    }

    return info, nil
}
