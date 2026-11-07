// Package assets names the game's images with asset IDs: strings that frontends resolve to
// sprites through an asset manifest (see util/export-assets, which writes one from an original
// install). Core and the views refer to images only through these IDs, so the art behind an ID can
// be replaced without code changes.
//
// IDs are lowercase paths: "terrain/123", "unit/high-men/spearmen/blue", "city/town/walled/red".
package assets

import (
    "fmt"
    "strings"

    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/maplib"
    "github.com/bfalese/civ-of-magic/core/units"
)

// lowercase, words joined with '-', only letters, digits and '-'
func Slug(name string) string {
    var out strings.Builder
    dash := false
    for _, character := range strings.ToLower(name) {
        if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') {
            if dash && out.Len() > 0 {
                out.WriteByte('-')
            }
            dash = false
            out.WriteRune(character)
        } else {
            dash = true
        }
    }
    return out.String()
}

// a map tile by its terrain tile index (the value in maplib.Map's terrain grid; Myrror tiles come
// after terrain.MyrrorStart). Animated tiles have several frames.
func TerrainTile(index int) string {
    return fmt.Sprintf("terrain/%v", index)
}

// a unit's overland sprite, in its owner's colours
func Unit(race data.Race, name string, banner data.BannerType) string {
    return fmt.Sprintf("unit/%v/%v/%v", Slug(race.String()), Slug(name), banner.String())
}

// the coloured square drawn behind an overland unit
func UnitBackground(banner data.BannerType) string {
    return fmt.Sprintf("unit-background/%v", banner.String())
}

func citySizeName(size citylib.CitySize) string {
    switch size {
        case citylib.CitySizeHamlet: return "hamlet"
        case citylib.CitySizeVillage: return "village"
        case citylib.CitySizeTown: return "town"
        case citylib.CitySizeCity: return "city"
        case citylib.CitySizeCapital: return "capital"
    }
    return "hamlet"
}

var CitySizes = []citylib.CitySize{citylib.CitySizeHamlet, citylib.CitySizeVillage, citylib.CitySizeTown, citylib.CitySizeCity, citylib.CitySizeCapital}

// a city on the overland map
func City(size citylib.CitySize, walled bool, banner data.BannerType) string {
    wall := "open"
    if walled {
        wall = "walled"
    }
    return fmt.Sprintf("city/%v/%v/%v", citySizeName(size), wall, banner.String())
}

// a road piece: direction is "center" (an unconnected road) or one of RoadDirections
func Road(direction string, enchanted bool) string {
    if enchanted {
        return "road-enchanted/" + direction
    }
    return "road/" + direction
}

var RoadDirections = []string{"center", "n", "ne", "e", "se", "s", "sw", "w", "nw"}

// the road direction name of a map direction
func RoadDirection(direction maplib.Direction) string {
    switch direction {
        case maplib.DirectionNorth: return "n"
        case maplib.DirectionNorthEast: return "ne"
        case maplib.DirectionEast: return "e"
        case maplib.DirectionSouthEast: return "se"
        case maplib.DirectionSouth: return "s"
        case maplib.DirectionSouthWest: return "sw"
        case maplib.DirectionWest: return "w"
        case maplib.DirectionNorthWest: return "nw"
    }
    return "center"
}

// a mineral or other bonus on a tile
func Bonus(bonus data.BonusType) string {
    return "bonus/" + Slug(bonus.String())
}

// a lair, ruin, tower... on the map
func Encounter(encounterType maplib.EncounterType) string {
    return "encounter/" + Slug(encounterType.Name())
}

// a tower whose guardians were beaten
const OpenTower = "encounter/open-tower"

// a corrupted tile
const Corruption = "corruption"

// the sparkles over a node's zone once a wizard melded it (animated)
func NodeZone(banner data.BannerType) string {
    return "node-zone/" + banner.String()
}

// battlefield images, by the LBX entry the combat model names (core/combat Tile, TileTop): a base
// tile (all frames), or a tree/rock/building on top (cropped to its content, as drawn)
func BattleTile(lbxFile string, index int) string {
    return fmt.Sprintf("battle-tile/%v/%v", strings.TrimSuffix(lbxFile, ".lbx"), index)
}

func BattleTop(lbxFile string, index int) string {
    return fmt.Sprintf("battle-top/%v/%v", strings.TrimSuffix(lbxFile, ".lbx"), index)
}

// the mud a tile turns into (Earth to Mud), animated
const BattleMud = "battle-mud"

// the road through a town on the battlefield, and the clouds under a flying fortress
const BattleRoad = "battle-road"
const BattleClouds = "battle-clouds"

var Facings = []units.Facing{units.FacingUp, units.FacingUpRight, units.FacingRight, units.FacingDownRight, units.FacingDown, units.FacingDownLeft, units.FacingLeft, units.FacingUpLeft}

func FacingName(facing units.Facing) string {
    switch facing {
        case units.FacingUp: return "up"
        case units.FacingUpRight: return "up-right"
        case units.FacingRight: return "right"
        case units.FacingDownRight: return "down-right"
        case units.FacingDown: return "down"
        case units.FacingDownLeft: return "down-left"
        case units.FacingLeft: return "left"
        case units.FacingUpLeft: return "up-left"
    }
    return "right"
}

// a unit's combat figure facing a direction, all its frames (walking, then attacking). The banner
// colour isn't baked in: pixels in the manifest's "bannerMarker" colours are replaced with the
// owner's "bannerColors" by the frontend.
func BattleUnit(race data.Race, name string, facing units.Facing) string {
    return fmt.Sprintf("battle-unit/%v/%v/%v", Slug(race.String()), Slug(name), FacingName(facing))
}
