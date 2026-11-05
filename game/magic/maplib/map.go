package maplib

import (
    "math"
    "math/rand/v2"
    "image"
    "slices"

    "github.com/bfalese/civ-of-magic/lib/fraction"
    "github.com/bfalese/civ-of-magic/lib/set"
    "github.com/bfalese/civ-of-magic/core/terrain"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/units"

)

type MiniMapCity interface {
    GetX() int
    GetY() int
    GetBanner() data.BannerType
}

type CityProvider interface {
    ContainsCity(x int, y int, plane data.Plane) bool
}

type Wizard interface {
    GetBanner() data.BannerType
}

type MagicNode int
const (
    MagicNodeNature MagicNode = iota
    MagicNodeSorcery
    MagicNodeChaos

    MagicNodeNone
)

func magicNodeFromString(str string) MagicNode {
    switch str {
        case "Nature Node": return MagicNodeNature
        case "Sorcery Node": return MagicNodeSorcery
        case "Chaos Node": return MagicNodeChaos
        default: return MagicNodeNone
    }
}

func (magicNode MagicNode) MagicType() data.MagicType {
    switch magicNode {
        case MagicNodeNature: return data.NatureMagic
        case MagicNodeSorcery: return data.SorceryMagic
        case MagicNodeChaos: return data.ChaosMagic
    }

    return data.MagicNone
}

func (magicNode MagicNode) Name() string {
    switch magicNode {
        case MagicNodeNature: return "Nature Node"
        case MagicNodeSorcery: return "Sorcery Node"
        case MagicNodeChaos: return "Chaos Node"
    }
    return ""
}

type ExtraTile interface {
    Serialize() map[string]any
}

type ExtraKind int
const (
    ExtraKindRoad ExtraKind = iota
    ExtraKindBonus
    ExtraKindMagicNode
    ExtraKindEncounter
    ExtraKindOpenTower
    ExtraKindVolcano
    ExtraKindCorruption
)

func (kind ExtraKind) String() string {
    switch kind {
        case ExtraKindRoad: return "Road"
        case ExtraKindBonus: return "Bonus"
        case ExtraKindMagicNode: return "Magic Node"
        case ExtraKindEncounter: return "Encounter"
        case ExtraKindOpenTower: return "Open Tower"
        case ExtraKindVolcano: return "Volcano"
        case ExtraKindCorruption: return "Corruption"
    }

    return "Unknown"
}

func extraKindFromString(str string) ExtraKind {
    switch str {
        case "Road": return ExtraKindRoad
        case "Bonus": return ExtraKindBonus
        case "Magic Node": return ExtraKindMagicNode
        case "Encounter": return ExtraKindEncounter
        case "Open Tower": return ExtraKindOpenTower
        case "Volcano": return ExtraKindVolcano
        case "Corruption": return ExtraKindCorruption
        default: return ExtraKindRoad
    }
}

var ExtraDrawOrder = []ExtraKind{
    ExtraKindVolcano,
    ExtraKindCorruption,
    ExtraKindRoad,
    ExtraKindBonus,
    ExtraKindMagicNode,
    ExtraKindEncounter,
    ExtraKindOpenTower,
}

type Direction int
const (
    DirectionNorth Direction = iota
    DirectionNorthEast
    DirectionEast
    DirectionSouthEast
    DirectionSouth
    DirectionSouthWest
    DirectionWest
    DirectionNorthWest
)

type ExtraRoad struct {
    Map *Map
    // either cast Enchant Road, or build the road in Myrror
    Enchanted bool
    X int
    Y int
}

// wild game, gold ore, mithril, etc
type ExtraBonus struct {
    Bonus data.BonusType
}

type EncounterType int

const (
    // lair, plane tower, ancient temple, fallen temple, ruins, abandoned keep, dungeon
    EncounterTypeLair EncounterType = iota
    EncounterTypeCave
    EncounterTypePlaneTower
    EncounterTypeAncientTemple
    EncounterTypeFallenTemple
    EncounterTypeRuins
    EncounterTypeAbandonedKeep
    EncounterTypeDungeon
    EncounterTypeChaosNode
    EncounterTypeNatureNode
    EncounterTypeSorceryNode
)

func encounterByName(name string) EncounterType {
    switch name {
        case "Monster Lair": return EncounterTypeLair
        case "Mysterious Cave": return EncounterTypeCave
        case "Tower": return EncounterTypePlaneTower
        case "Ancient Temple": return EncounterTypeAncientTemple
        case "Fallen Temple": return EncounterTypeFallenTemple
        case "Ruins": return EncounterTypeRuins
        case "Abandoned Keep": return EncounterTypeAbandonedKeep
        case "Dungeon": return EncounterTypeDungeon
        case "Chaos Node": return EncounterTypeChaosNode
        case "Nature Node": return EncounterTypeNatureNode
        case "Sorcery Node": return EncounterTypeSorceryNode
        default: return EncounterTypeLair
    }
}

func (encounterType EncounterType) Name() string {
    switch encounterType {
        case EncounterTypeLair: return "Monster Lair"
        case EncounterTypeCave: return "Mysterious Cave"
        case EncounterTypePlaneTower: return "Tower"
        case EncounterTypeAncientTemple: return "Ancient Temple"
        case EncounterTypeFallenTemple: return "Fallen Temple"
        case EncounterTypeRuins: return "Ruins"
        case EncounterTypeAbandonedKeep: return "Abandoned Keep"
        case EncounterTypeDungeon: return "Dungeon"
        case EncounterTypeChaosNode: return "Chaos Node"
        case EncounterTypeNatureNode: return "Nature Node"
        case EncounterTypeSorceryNode: return "Sorcery Node"
    }

    return ""
}

func (encounterType EncounterType) ShortName() string {
    switch encounterType {
        case EncounterTypeLair: return "Lair"
        case EncounterTypeCave: return "Cave"
        case EncounterTypePlaneTower: return "Tower"
        case EncounterTypeAncientTemple: return "Temple"
        case EncounterTypeFallenTemple: return "Temple"
        case EncounterTypeRuins: return "Ruins"
        case EncounterTypeAbandonedKeep: return "Keep"
        case EncounterTypeDungeon: return "Dungeon"
        case EncounterTypeChaosNode: return "Node"
        case EncounterTypeNatureNode: return "Node"
        case EncounterTypeSorceryNode: return "Node"
    }

    return ""
}

func (encounterType EncounterType) LbxIndex() int {
    switch encounterType {
        case EncounterTypeLair, EncounterTypeCave: return 71
        case EncounterTypePlaneTower: return 69
        case EncounterTypeAncientTemple: return 72
        case EncounterTypeFallenTemple: return 75
        case EncounterTypeRuins: return 74
        case EncounterTypeAbandonedKeep: return 73
        case EncounterTypeDungeon: return 74
    }

    return -1
}

func randomEncounterType() EncounterType {
    all := []EncounterType{
        EncounterTypeLair,
        EncounterTypeCave,
        EncounterTypeAncientTemple,
        EncounterTypeFallenTemple,
        EncounterTypeRuins,
        EncounterTypeAbandonedKeep,
        EncounterTypeDungeon,
    }

    return all[rand.N(len(all))]
}

// lair, plane tower, etc and also the initial enemies on nodes
type ExtraEncounter struct {
    Type EncounterType
    Units []units.Unit
    Budget int // used for treasure
    ExploredBy *set.Set[Wizard]
}

// choices is a map from a name to the chance of choosing that name, where all the int values should add up to 100
// an individual int is a percentage chance to choose the given key
// for example, choices might be the map {"a": 30, "b": 30, "c": 40}
// which means that a and b should both have a 30% chance of being picked, and c has a 40% chance of being picked
func chooseValue[T comparable](choices map[T]int) T {
    total := 0
    for _, value := range choices {
        total += value
    }

    pick := rand.N(total)
    for key, value := range choices {
        if pick < value {
            return key
        }

        pick -= value
    }

    var out T
    return out
}

func makeEncounter(encounterType EncounterType, difficulty data.DifficultySetting, weakStrength bool, plane data.Plane) *ExtraEncounter {
    var guardians []units.Unit
    var secondary []units.Unit

    budget := 0
    if weakStrength {
        if plane == data.PlaneArcanus || encounterType == EncounterTypePlaneTower {
            budget = (rand.N(20) + 1) * 30
        } else {
            budget = (rand.N(30) + 1) * 30
        }
    } else {
        // Wider spread and a lower floor so strong lairs vary more
        // (fewer uniformly-maxed high-end lairs), then scaled down ~25%.
        if plane == data.PlaneArcanus || encounterType == EncounterTypePlaneTower {
            budget = (rand.N(145) + 1) * 30 + 75
        } else {
            budget = (rand.N(160) + 1) * 30 + 75
        }
        budget = int(float64(budget) * 0.75)
    }

    bonus := float64(0)
    switch difficulty {
        case data.DifficultyIntro: bonus = -0.75
        case data.DifficultyEasy: bonus = -0.5
        case data.DifficultyAverage: bonus = -0.25
        case data.DifficultyHard: bonus = 0
        case data.DifficultyExtreme: bonus = 0.25
        case data.DifficultyImpossible: bonus = 0.50
    }

    budget = int(float64(budget) * (1.0 + bonus))

    chooseRealm := func() string {
        switch encounterType {
            case EncounterTypeLair: return chooseValue(map[string]int{"chaos": 40, "death": 40, "nature": 20})
            case EncounterTypeCave: return chooseValue(map[string]int{"chaos": 40, "death": 40, "nature": 20})
            case EncounterTypePlaneTower: return chooseValue(map[string]int{"chaos": 10, "death": 20, "nature": 10, "life": 10, "sorcery": 10})
            case EncounterTypeAncientTemple: return chooseValue(map[string]int{"death": 75, "life": 25})
            case EncounterTypeFallenTemple: return chooseValue(map[string]int{"death": 75, "life": 25})
            case EncounterTypeRuins: return chooseValue(map[string]int{"death": 75, "life": 25})
            case EncounterTypeAbandonedKeep: return chooseValue(map[string]int{"chaos": 40, "death": 40, "nature": 20})
            case EncounterTypeDungeon: return chooseValue(map[string]int{"chaos": 40, "death": 40, "nature": 20})
        }

        return ""
    }

    switch chooseRealm() {
        case "chaos":
            guardians, secondary = computeChaosNodeEnemies(budget)
        case "death":
            guardians, secondary = computeDeathNodeEnemies(budget)
        case "nature":
            guardians, secondary = computeNatureNodeEnemies(budget)
        case "life":
            guardians, secondary = computeLifeNodeEnemies(budget)
        case "sorcery":
            guardians, secondary = computeSorceryNodeEnemies(budget)
    }

    return &ExtraEncounter{
        Type: encounterType,
        Budget: budget,
        Units: append(guardians, secondary...),
        ExploredBy: set.MakeSet[Wizard](),
    }
}

type ExtraOpenTower struct {
}

type ExtraMagicNode struct {
    Kind MagicNode

    // list of points that are affected by the node
    // this could be a map to be more optimal for lookups
    Zone []image.Point

    // if this node is melded, then this player receives the power
    MeldingWizard Wizard
    // true if melded by a guardian spirit, otherwise false if melded by a magic spirit
    GuardianSpiritMeld bool

    Warped bool
    // the wizard that cast warp node (so it can be dispelled)
    WarpedOwner Wizard
}

func (node *ExtraMagicNode) ContainsPoint(x int, y int) bool {
    return slices.Contains(node.Zone, image.Pt(x, y))
}

func (node *ExtraMagicNode) Meld(meldingWizard Wizard, spirit units.Unit) bool {
    if node.Warped {
        return false
    } else if node.MeldingWizard == nil {
        node.MeldingWizard = meldingWizard
        if spirit.Equals(units.GuardianSpirit) {
            node.GuardianSpiritMeld = true
        } else {
            node.GuardianSpiritMeld = false
        }

        return true
    } else {
        // can't meld the same node twice
        if node.MeldingWizard == meldingWizard {
            return false
        }

        successful := true
        // 25% chance to meld if guardian spirit already melded it
        if node.GuardianSpiritMeld && rand.IntN(4) != 0 {
            successful = false
        }

        if successful {
            node.MeldingWizard = meldingWizard
            if spirit.Equals(units.GuardianSpirit) {
                node.GuardianSpiritMeld = true
            } else {
                node.GuardianSpiritMeld = false
            }

            return true
        }

        return false
    }
}

func (node *ExtraMagicNode) GetPower(magicBonus float64) float64 {
    if node.Warped {
        return -5
    }

    return float64(len(node.Zone)) * magicBonus
}

type ExtraVolcano struct {
    CastingWizard Wizard
}

type ExtraCorruption struct {
}

type FullTile struct {
    Extras map[ExtraKind]ExtraTile
    Tile terrain.Tile
    X int
    Y int
    // shared between multiple cities
    IsShared bool
}

type TraverseType int
const (
    TraverseWater TraverseType = iota
    TraverseLand
)

// true if this tile can be traversed moving in the given direction with the given traverse type.
// this is like saying 'can we move out of this tile in the given direction using the given traverse type?'
// example:
//  tile=water, direction=north, traverseType=land => false
//  tile=water, direction=north, traverseType=water => true
//  tile=shore with land on east side, direction=east, traverseType=land => true
//  tile=shore with land on east side, direction=east, traverseType=water => false
func (tile *FullTile) CanTraverse(direction terrain.Direction, traverseType TraverseType) bool {
    switch tile.Tile.TerrainType() {
        case terrain.Ocean: return traverseType == TraverseWater
        case terrain.Shore:
            match := make(map[terrain.Direction]terrain.TerrainType)
            switch traverseType {
                case TraverseLand:
                    match[direction] = terrain.TileOcean.TerrainType()
                    return !tile.Tile.Matches(match)
                case TraverseWater:
                    // shore tiles do not have any compatibilities for the corner directions,
                    // which usually means those directions can contain land but not water
                    _, has := tile.Tile.Compatibilities[direction]
                    if !has {
                        return false
                    }

                    match[direction] = terrain.TileOcean.TerrainType()
                    return tile.Tile.Matches(match)
            }

        default: return traverseType == TraverseLand
    }

    return false
}

func (tile *FullTile) Name(mapObject *Map) string {
    if tile.IsRiverMouth(mapObject) {
        return "River Mouth"
    }

    return tile.Tile.Name()
}

func (tile *FullTile) Valid() bool {
    return tile.Tile.Valid()
}

func (tile *FullTile) Corrupted() bool {
    _, ok := tile.Extras[ExtraKindCorruption]
    return ok
}

func (tile *FullTile) HasEncounter() bool {
    _, ok := tile.Extras[ExtraKindEncounter]
    return ok
}

func (tile *FullTile) HasRoad() bool {
    _, ok := tile.Extras[ExtraKindRoad]
    return ok
}

func (tile *FullTile) FoodBonus() fraction.Fraction {
    if tile.Corrupted() {
        return fraction.Zero()
    }

    if tile.Tile.IsLakeWithFlow() {
        return fraction.FromInt(2)
    }
    // FIXME: Shore with two river deltas = 2
    // FIXME: Shore with single river delta surrounded by ocean = 2

    switch tile.Tile.TerrainType() {
        case terrain.Ocean: return fraction.Zero()
        case terrain.Grass: return fraction.Make(3, 2)
        case terrain.Forest: return fraction.Make(1, 2)
        case terrain.Mountain: return fraction.Zero()
        case terrain.Desert: return fraction.Zero()
        case terrain.Swamp: return fraction.Zero()
        case terrain.Tundra: return fraction.Zero()
        case terrain.SorceryNode: return fraction.FromInt(2)
        case terrain.NatureNode: return fraction.Make(5, 2)
        case terrain.ChaosNode: return fraction.Zero()
        case terrain.Hill: return fraction.Make(1, 2)
        case terrain.Volcano: return fraction.Zero()
        case terrain.Lake: return fraction.Zero()
        case terrain.River: return fraction.FromInt(2)
        case terrain.Shore: return fraction.Make(1, 2)
    }

    return fraction.Zero()
}

// percent bonus increase, 3 = 3%
func (tile *FullTile) GoldBonus(mapObject *Map) int {
    if tile.Corrupted() {
        return 0
    }

    switch {
        case tile.IsRiverMouth(mapObject): return 30
        case tile.Tile.TerrainType() == terrain.River: return 20
        case tile.IsTouchingShore(mapObject, false): return 10
    }

    return 0
}

// percent bonus increase, 3 = 3%
func (tile *FullTile) ProductionBonus(hasGaiasBlessing bool) int {
    if tile.Corrupted() {
        return 0
    }

    terrainType := tile.Tile.TerrainType()
    switch {
        case terrainType == terrain.Forest && hasGaiasBlessing: return 6
        case terrainType == terrain.Forest: return 3
        case terrainType == terrain.Mountain: return 5
        case terrainType == terrain.Desert: return 3
        case terrainType == terrain.NatureNode: return 3
        case terrainType == terrain.ChaosNode: return 5
        case terrainType == terrain.Hill: return 3
    }

    return 0
}

func (tile *FullTile) IsRiverMouth(mapObject *Map) bool {
    return tile.Tile.TerrainType() == terrain.River && tile.IsTouchingShore(mapObject, true)
}

func (tile *FullTile) IsTouchingShore(mapObject *Map, cardinal bool) bool {
    for dx := -1; dx <= 1; dx++ {
        for dy := -1; dy <= 1; dy++ {
            if dx == 0 && dy == 0 {
                continue
            }

            if cardinal && dx != 0 && dy != 0 {
                continue
            }

            tile := mapObject.GetTile(tile.X + dx, tile.Y + dy)
            if tile.Tile.IsShore() {
                return true
            }
        }
    }

    return false
}

func (tile *FullTile) IsByRiver(mapObject *Map) bool {
    for dx := -1; dx <= 1; dx++ {
        for dy := -1; dy <= 1; dy++ {
            tile := mapObject.GetTile(tile.X + dx, tile.Y + dy)
            if tile.Tile.IsRiver() {
                return true
            }
        }
    }

    return false
}

func (tile *FullTile) GetBonus() data.BonusType {
    if tile.Corrupted() {
        return data.BonusNone
    }

    bonus, ok := tile.Extras[ExtraKindBonus]
    if ok {
        return bonus.(*ExtraBonus).Bonus
    }

    return data.BonusNone
}

type Map struct {
    Map *terrain.Map

    Plane data.Plane

    CityProvider CityProvider

    // contains information about map squares that contain extra features on top
    // such as a road, enchantment, encounter place (plane tower, lair, etc)
    ExtraMap map[image.Point]map[ExtraKind]ExtraTile

    Data *terrain.TerrainData

    // rendering state owned by the frontend (e.g. mapview's tile and minimap caches).
    // core code never reads it.
    RenderCache any
}

func getLandSize(size int) (int, int) {
    switch size {
        case 0: return 50, 50
        case 1: return 100, 100
        case 2: return 200, 150
    }

    return 100, 100
}

func MakeMap(terrainData *terrain.TerrainData, landSize int, magicSetting data.MagicSetting, difficulty data.DifficultySetting, plane data.Plane, cityProvider CityProvider, planeTowers []image.Point) *Map {
    landWidth, landHeight := getLandSize(landSize)

    map_ := terrain.GenerateLandCellularAutomata(landWidth, landHeight, terrainData, plane)

    extraMap := make(map[image.Point]map[ExtraKind]ExtraTile)

    // place towers, and then re-resolve tiles
    for _, point := range planeTowers {
        map_.Terrain[point.X][point.Y] = terrain.GetTile(terrain.IndexGrass1).Index(plane)
        extraMap[point] = make(map[ExtraKind]ExtraTile)
        extraMap[point][ExtraKindEncounter] = makeEncounter(EncounterTypePlaneTower, difficulty, false, plane)
    }

    map_.ResolveTiles(terrainData, plane)

    for x := range landWidth {
        for y := range landHeight {
            point := image.Pt(x, y)
            _, exists := extraMap[point]
            if !exists {
                extraMap[point] = make(map[ExtraKind]ExtraTile)
            }

            tile := terrainData.Tiles[map_.Terrain[x][y]].Tile
            switch tile.TerrainType() {
                case terrain.SorceryNode:
                    extraMap[point][ExtraKindMagicNode], extraMap[point][ExtraKindEncounter] = MakeMagicNode(MagicNodeSorcery, magicSetting, difficulty, plane)
                case terrain.NatureNode:
                    extraMap[point][ExtraKindMagicNode], extraMap[point][ExtraKindEncounter] = MakeMagicNode(MagicNodeNature, magicSetting, difficulty, plane)
                case terrain.ChaosNode:
                    extraMap[point][ExtraKindMagicNode], extraMap[point][ExtraKindEncounter] = MakeMagicNode(MagicNodeChaos, magicSetting, difficulty, plane)
            }
        }
    }

    canPlaceEncounter := func (x int, y int) bool {
        // an encounter can be placed if the specified tile is plain land, and is not near some other node/encounter
        // original MoM never put lairs on the polar ice row
        if terrain.IsPolarIceRow(y, landHeight) {
            return false
        }

        tile := terrainData.Tiles[map_.Terrain[x][y]].Tile
        if !tile.IsLand() || tile.IsMagic() {
            return false
        }

        switch tile.TerrainType() {
            // FIXME: what types of terrain should we not place encounters on?
            case terrain.River: return false
        }

        // check that no surrounding tile is special
        for dx := -3; dx <= 3; dx++ {
            for dy := -3; dy <= 3; dy++ {
                cx := map_.WrapX(dx + x)
                cy := dy + y

                if cy < 0 || cy >= landHeight {
                    continue
                }

                extra, hasExtra := extraMap[image.Pt(cx, cy)]
                if hasExtra && extra != nil && len(extra) > 0 {
                    return false
                }
            }
        }

        return true
    }

    // returns a map of bonus types and the percent chance to get that bonus
    // https://masterofmagic.fandom.com/wiki/Mineral
    bonusTypeMap := func (x int, y int) map[data.BonusType]int {
        out := make(map[data.BonusType]int)

        cast := func (v float64) int {
            return int(v)
        }

        tile := terrainData.Tiles[map_.Terrain[x][y]].Tile
        switch tile.TerrainType() {
            case terrain.Hill:
                if plane == data.PlaneArcanus {
                    out[data.BonusIronOre] = cast(2 * 1000 / 6)
                    out[data.BonusCoal] = cast(1 * 1000 / 6)
                    out[data.BonusSilverOre] = cast(1.33 * 1000 / 6)
                    out[data.BonusGoldOre] = cast(1.33 * 1000 / 6)
                    out[data.BonusMithrilOre] = cast(0.33 * 1000 / 6)
                } else {
                    out[data.BonusIronOre] = cast(1 * 100 / 10)
                    out[data.BonusCoal] = cast(1 * 100 / 10)
                    out[data.BonusSilverOre] = cast(1 * 100 / 10)
                    out[data.BonusGoldOre] = cast(4 * 100 / 10)
                    out[data.BonusMithrilOre] = cast(2 * 100 / 10)
                    out[data.BonusAdamantiumOre] = cast(1 * 100 / 10)
                }
            case terrain.Forest:
                out[data.BonusWildGame] = 100
            case terrain.Mountain:
                if plane == data.PlaneArcanus {
                    out[data.BonusSilverOre] = cast(1 * 1000 / 6)
                    out[data.BonusGoldOre] = cast(1 * 1000 / 6)
                    out[data.BonusIronOre] = cast(1.33 * 1000 / 6)
                    out[data.BonusCoal] = cast(1.67 * 1000 / 6)
                    out[data.BonusMithrilOre] = cast(1 * 1000 / 6)
                } else {
                    out[data.BonusSilverOre] = cast(1 * 100 / 10)
                    out[data.BonusGoldOre] = cast(2 * 100 / 10)
                    out[data.BonusIronOre] = cast(1 * 100 / 10)
                    out[data.BonusCoal] = cast(1 * 100 / 10)
                    out[data.BonusMithrilOre] = cast(3 * 100 / 10)
                    out[data.BonusAdamantiumOre] = cast(2 * 100 / 10)
                }

                /*
            case terrain.Grass:
                if plane == data.PlaneArcanus {
                    out[data.BonusGoldOre] = 100
                } else {
                    out[data.BonusGoldOre] = int(1 * 100 / 2)
                    out[data.BonusCoal] = int(1 * 100 / 2)
                }
                */

            case terrain.Swamp:
                out[data.BonusNightshade] = 100
            case terrain.Desert:
                if plane == data.PlaneArcanus {
                    out[data.BonusGem] = cast(4 * 100 / 6)
                    out[data.BonusQuorkCrystal] = cast(2 * 100 / 6)
                } else {
                    out[data.BonusGem] = cast(2 * 100 / 10)
                    out[data.BonusQuorkCrystal] = cast(6 * 100 / 10)
                    out[data.BonusCrysxCrystal] = cast(2 * 100 / 10)
                }

            case terrain.Tundra: // none

            case terrain.Volcano, terrain.Lake, terrain.Ocean, terrain.River,
                terrain.Shore, terrain.NatureNode, terrain.SorceryNode, terrain.ChaosNode:
                // none

        }

        return out
    }

    canContainMineral := func (x int, y int) bool {
        tile := terrainData.Tiles[map_.Terrain[x][y]].Tile
        if !tile.IsLand() || tile.IsMagic() {
            return false
        }

        // FIXME: not 100% sure on this, can there be a bonus under a lair?
        _, hasLair := extraMap[image.Pt(x, y)][ExtraKindEncounter]
        if hasLair {
            return false
        }

        switch tile.TerrainType() {
            case terrain.Hill, terrain.Forest, terrain.Mountain,
                 terrain.Swamp, terrain.Desert: return true
        }

        return false
    }

    continents := map_.FindContinents()

    // place some encounter nodes down (lair, cave, etc)
    for i := range len(continents) {

        // try to place N encounters. if we can't place them all, then we just place as many as we can
        maxEncounters := continents[i].Size() / 10
        points := continents[i].Values()
        for _, index := range rand.Perm(len(points)) {

            if maxEncounters == 0 {
                break
            }

            x, y := points[index].X, points[index].Y

            if canPlaceEncounter(x, y) {
                // log.Printf("Place encounter at %v, %v", x, y)
                extraMap[image.Pt(x, y)][ExtraKindEncounter] = makeEncounter(randomEncounterType(), difficulty, rand.N(2) == 0, plane)
                maxEncounters -= 1
            }
        }

        var candidates []image.Point
        for _, point := range points {
            if canContainMineral(point.X, point.Y) {
                candidates = append(candidates, point)
            }
        }

        fraction := 0.06
        if plane == data.PlaneMyrror {
            fraction = 0.10
        }

        maxBonuses := int(float64(len(candidates)) * fraction)
        // log.Printf("Candidates %v max bonuses %v", len(candidates), maxBonuses)
        for count, index := range rand.Perm(len(candidates)) {
            if count > maxBonuses {
                break
            }

            point := candidates[index]
            x, y := point.X, point.Y

            bonusTypes := bonusTypeMap(x, y)
            if len(bonusTypes) > 0 {
                chosen := chooseValue(bonusTypes)
                // if one didn't get picked because the values in the map don't add to 100 then just pick one randomly
                if chosen == data.BonusNone {
                    var choices []data.BonusType
                    for choice, _ := range bonusTypes {
                        choices = append(choices, choice)
                    }
                    chosen = choices[rand.N(len(choices))]
                }
                extraMap[point][ExtraKindBonus] = &ExtraBonus{Bonus: chosen}
            }
        }
    }

    return &Map{
        Data: terrainData,
        Map: map_,
        Plane: plane,
        ExtraMap: extraMap,
        CityProvider: cityProvider,
    }
}

func GeneratePlaneTowerPositions(landSize int, count int) []image.Point {
    width, height := getLandSize(landSize)

    var out []image.Point

    for range count {

        success := false
        tries := 100
        for !success && tries > 0 {
            tries -= 1
            x := rand.N(width)
            y := rand.N(height)

            // not too close to the edges
            if y < 3 || y >= height - 3 {
                continue
            }

            point := image.Pt(x, y)

            ok := true
            for _, other := range out {
                vector := other.Sub(point)
                distance := math.Sqrt(float64(vector.X * vector.X + vector.Y * vector.Y))
                if distance < 10 {
                    ok = false
                    break
                }
            }

            if ok {
                out = append(out, point)
                success = true
            }
        }
    }

    return out
}

func (mapObject *Map) GetContinents() []terrain.Continent {
    return mapObject.Map.FindContinents()
}

// get all the tiles that are part of the continent that contains the given x, y
func (mapObject *Map) GetContinentTiles(x int, y int) []FullTile {
    continent := mapObject.Map.FindContinent(x, y)
    var out []FullTile
    for _, point := range continent.Values() {
        tile := mapObject.GetTile(point.X, point.Y)
        out = append(out, tile)
    }
    return out
}

func (mapObject *Map) IsWater(x int, y int) bool {
    tile := mapObject.GetTile(x, y)
    return tile.Tile.IsWater()
}

func (mapObject *Map) GetWaterBodies() []*set.Set[image.Point] {
    var sets []*set.Set[image.Point]
    // compute bodies of water by choosing a water tile and doing a flood fill to find all connected water tiles

    visited := set.NewSet[image.Point]()
    for tx := range mapObject.Width() {
        for ty := range mapObject.Height() {
            if visited.Contains(image.Pt(tx, ty)) {
                continue
            }

            tile := mapObject.GetTile(tx, ty)
            if !tile.Tile.IsWater() {
                visited.Insert(image.Pt(tx, ty))
                continue
            }

            toCheck := []image.Point{image.Pt(tx, ty)}

            newSet := set.NewSet[image.Point]()

            // floodfill
            for len(toCheck) > 0 {
                check := toCheck[0]
                toCheck = toCheck[1:]

                if visited.Contains(check) {
                    continue
                }

                newSet.Insert(check)
                visited.Insert(check)

                fromTile := mapObject.GetTile(check.X, check.Y)

                for dx := -1; dx <= 1; dx++ {
                    for dy := -1; dy <= 1; dy++ {
                        cx := mapObject.WrapX(check.X + dx)
                        cy := check.Y + dy

                        tile := mapObject.GetTile(cx, cy)
                        // shouldn't need this, but just in case
                        /* && tile.CanTraverse(terrain.ToDirection(-dx, -dy), TraverseWater) */
                        if tile.Valid() && tile.Tile.IsWater() && fromTile.CanTraverse(terrain.ToDirection(dx, dy), TraverseWater)  {
                            toCheck = append(toCheck, image.Pt(cx, cy))
                        }
                    }
                }
            }

            sets = append(sets, newSet)
            // log.Printf("Found water body with tiles %v", newSet.Values())
        }
    }

    return sets
}

// returns a map where for each direction, if the value is true then there is a road there
func (mapObject *Map) GetRoadNeighbors(x int, y int) map[Direction]bool {
    out := make(map[Direction]bool)

    convert := func(dx int, dy int) Direction {
        switch {
            case dx == -1 && dy == -1: return DirectionNorthWest
            case dx == -1 && dy == 0: return DirectionWest
            case dx == -1 && dy == 1: return DirectionSouthWest

            case dx == 1 && dy == -1: return DirectionNorthEast
            case dx == 1 && dy == 0: return DirectionEast
            case dx == 1 && dy == 1: return DirectionSouthEast

            case dx == 0 && dy == -1: return DirectionNorth
            case dx == 0 && dy == 1: return DirectionSouth
        }

        return DirectionNorth
    }

    for dx := -1; dx <= 1; dx++ {
        for dy := -1; dy <= 1; dy++ {
            // skip center
            if dx == 0 && dy == 0 {
                continue
            }

            out[convert(dx, dy)] = mapObject.ContainsRoad(mapObject.WrapX(x + dx), y + dy) || mapObject.CityProvider.ContainsCity(mapObject.WrapX(x + dx), y + dy, mapObject.Plane)
        }
    }

    return out
}

func (mapObject *Map) GetMeldedNodes(melder Wizard) []*ExtraMagicNode {
    var out []*ExtraMagicNode

    for _, extras := range mapObject.ExtraMap {
        node, ok := extras[ExtraKindMagicNode]
        if ok {
            magic := node.(*ExtraMagicNode)
            if magic.MeldingWizard == melder {
                out = append(out, magic)
            }
        }
    }

    return out
}

func (mapObject *Map) GetCastedVolcanoes(caster Wizard) []*ExtraVolcano {
    var out []*ExtraVolcano

    for _, extras := range mapObject.ExtraMap {
        extra, ok := extras[ExtraKindVolcano]
        if ok {
            volcano := extra.(*ExtraVolcano)
            if volcano.CastingWizard == caster {
                out = append(out, volcano)
            }
        }
    }

    return out
}

func (mapObject *Map) RemoveBonus(x int, y int) {
    delete(mapObject.ExtraMap[image.Pt(x, y)], ExtraKindBonus)
}

func (mapObject *Map) SetBonus(x int, y int, bonus data.BonusType) {
    point := image.Pt(x, y)
    extras := mapObject.ExtraMap[point]
    extras[ExtraKindBonus] = &ExtraBonus{Bonus: bonus}
}

func getExtra[T any](extras map[ExtraKind]ExtraTile, kind ExtraKind) T {
    if obj, ok := extras[kind]; ok {
        if obj != nil {
            return obj.(T)
        }
    }

    var out T
    return out
}

func (mapObject *Map) SetRoad(x int, y int, enchanted bool) {
    point := image.Pt(x, y)

    extras, exists := mapObject.ExtraMap[point]
    if !exists {
        extras = make(map[ExtraKind]ExtraTile)
        mapObject.ExtraMap[point] = extras
    }

    extras[ExtraKindRoad] = &ExtraRoad{Map: mapObject, X: x, Y: y, Enchanted: enchanted}
}

func (mapObject *Map) ContainsRoad(x int, y int) bool {
    _, ok := mapObject.ExtraMap[image.Pt(x, y)][ExtraKindRoad]
    return ok
}

func (mapObject *Map) RemoveRoad(x int, y int) {
    delete(mapObject.ExtraMap[image.Pt(x, y)], ExtraKindRoad)
}

func (mapObject *Map) GetEncounter(x int, y int) *ExtraEncounter {
    return getExtra[*ExtraEncounter](mapObject.ExtraMap[image.Pt(x, y)], ExtraKindEncounter)
}

func (mapObject *Map) GetEncounterLocations() []image.Point {
    var out []image.Point

    for point, extras := range mapObject.ExtraMap {
        _, exists := extras[ExtraKindEncounter]
        if exists {
            out = append(out, point)
        }
    }

    return out
}

func (mapObject *Map) GetMagicNodeLocations() []image.Point {
    var out []image.Point

    for point, extras := range mapObject.ExtraMap {
        _, exists := extras[ExtraKindMagicNode]
        if exists {
            out = append(out, point)
        }
    }

    return out
}

// GetOpenTowerLocations returns every tile on this map that holds an open
// (already-cleared) plane tower. A unit standing on an open tower can planar
// travel to the same tile on the opposite plane.
func (mapObject *Map) GetOpenTowerLocations() []image.Point {
    var out []image.Point

    for point, extras := range mapObject.ExtraMap {
        _, exists := extras[ExtraKindOpenTower]
        if exists {
            out = append(out, point)
        }
    }

    return out
}

func (mapObject *Map) RemoveEncounter(x int, y int) {
    _, exists := mapObject.ExtraMap[image.Pt(x, y)]
    if !exists {
        return
    }

    _, exists = mapObject.ExtraMap[image.Pt(x, y)][ExtraKindEncounter]
    if exists {
        delete(mapObject.ExtraMap[image.Pt(x, y)], ExtraKindEncounter)
    }
}

// an open plane tower
func (mapObject *Map) SetPlaneTower(x int, y int) {
    mapObject.ExtraMap[image.Pt(x, y)][ExtraKindOpenTower] = &ExtraOpenTower{}
}

func (mapObject *Map) HasOpenTower(x int, y int) bool {
    _, exists := mapObject.ExtraMap[image.Pt(x, y)][ExtraKindOpenTower]
    return exists
}

func (mapObject *Map) HasMagicNode(x int, y int) bool {
    _, exists := mapObject.ExtraMap[image.Pt(x, y)][ExtraKindMagicNode]
    return exists
}

func (mapObject *Map) GetMagicNode(x int, y int) *ExtraMagicNode {
    return getExtra[*ExtraMagicNode](mapObject.ExtraMap[image.Pt(x, y)], ExtraKindMagicNode)
}

func (mapObject *Map) GetAllMagicNodeLocations() []image.Point {
    var points []image.Point

    for point, extras := range mapObject.ExtraMap {
        _, exists := extras[ExtraKindMagicNode]
        if exists {
            points = append(points, point)
        }
    }

    return points
}

// return the node that contains x/y in its influence zone
// this is a bit slow in that it checks the entire map
func (mapObject* Map) GetMagicInfluence(x int, y int) *ExtraMagicNode {
    for point, extras := range mapObject.ExtraMap {
        magicNode := getExtra[*ExtraMagicNode](extras, ExtraKindMagicNode)
        if magicNode != nil && magicNode.ContainsPoint(x - point.X, y - point.Y) {
            return magicNode
        }
    }

    return nil
}

func (mapObject *Map) SetVolcano(x int, y int, caster Wizard) {
    mapObject.ExtraMap[image.Pt(x, y)][ExtraKindVolcano] = &ExtraVolcano{CastingWizard: caster}
    mapObject.Map.SetTerrainAt(x, y, terrain.Volcano, mapObject.Data, mapObject.Plane)
}

func (mapObject *Map) HasVolcano(x int, y int) bool {
    _, exists := mapObject.ExtraMap[image.Pt(x, y)][ExtraKindVolcano]
    return exists
}

func (mapObject *Map) RemoveVolcano(x int, y int) {
    _, exists := mapObject.ExtraMap[image.Pt(x, y)][ExtraKindVolcano]
    if exists {
        delete(mapObject.ExtraMap[image.Pt(x, y)], ExtraKindVolcano)

        mapObject.Map.SetTerrainAt(x, y, terrain.Mountain, mapObject.Data, mapObject.Plane)

        // chance of 5% to generate a mineral
        if rand.N(100) < 5 {
            choices := []data.BonusType{data.BonusSilverOre, data.BonusGoldOre, data.BonusIronOre, data.BonusCoal, data.BonusMithrilOre}
            if mapObject.Plane == data.PlaneMyrror {
                choices = append(choices, data.BonusAdamantiumOre)
            }
            mapObject.SetBonus(x, y, choices[rand.IntN(len(choices))])
        }
    }
}

func (mapObject *Map) GetBonusTile(x int, y int) data.BonusType {
    bonus := getExtra[*ExtraBonus](mapObject.ExtraMap[image.Pt(x, y)], ExtraKindBonus)
    if bonus != nil {
        return bonus.Bonus
    }

    return data.BonusNone
}

func (mapObject *Map) CreateEncounter(x int, y int, encounterType EncounterType, difficulty data.DifficultySetting, weakStrength bool, plane data.Plane) bool {
    if mapObject.GetEncounter(x, y) != nil {
        return false
    }

    mapObject.ExtraMap[image.Pt(x, y)][ExtraKindEncounter] = makeEncounter(encounterType, difficulty, weakStrength, plane)
    return true
}

func (mapObject *Map) CreateEncounterRandom(x int, y int, difficulty data.DifficultySetting, plane data.Plane) bool {
    return mapObject.CreateEncounter(x, y, randomEncounterType(), difficulty, rand.N(2) == 0, plane)
}

// for testing purposes
func (mapObject *Map) CreateNode(x int, y int, node MagicNode, plane data.Plane, magicSetting data.MagicSetting, difficulty data.DifficultySetting) *ExtraMagicNode {
    tileType := 0
    switch node {
        case MagicNodeNature: tileType = terrain.TileNatureForest.Index(plane)
        case MagicNodeSorcery: tileType = terrain.TileSorceryLake.Index(plane)
        case MagicNodeChaos: tileType = terrain.TileChaosVolcano.Index(plane)
    }

    mapObject.Map.Terrain[x][y] = tileType

    out, _ := MakeMagicNode(node, magicSetting, difficulty, plane)

    mapObject.ExtraMap[image.Pt(x, y)][ExtraKindMagicNode] = out

    return out
}

func (mapObject *Map) HasCorruption(x int, y int) bool {
    _, exists := mapObject.ExtraMap[image.Pt(x, y)]
    if exists {
        _, exists = mapObject.ExtraMap[image.Pt(x, y)][ExtraKindCorruption]
        return exists
    }
    return false
}

func (mapObject *Map) SetCorruption(x int, y int) {
    mapObject.ExtraMap[image.Pt(x, y)][ExtraKindCorruption] = &ExtraCorruption{}
}

func (mapObject *Map) RemoveCorruption(x int, y int) {
    if mapObject.HasCorruption(x, y) {
        delete(mapObject.ExtraMap[image.Pt(x, y)], ExtraKindCorruption)
    }
}

func (mapObject *Map) Width() int {
    return mapObject.Map.Columns()
}

func (mapObject *Map) Height() int {
    return mapObject.Map.Rows()
}

func (mapObject *Map) TileWidth() int {
    return mapObject.Data.TileWidth()
}

func (mapObject *Map) TileHeight() int {
    return mapObject.Data.TileHeight()
}

func (mapObject *Map) WrapX(x int) int {
    return mapObject.Map.WrapX(x)
}

// distance in tiles between two points
func (mapObject *Map) TileDistance(x1 int, y1 int, x2 int, y2 int) int {
    dx := mapObject.XDistance(x1, x2)
    dy := y2 - y1

    return int(math.Sqrt(float64(dx * dx + dy * dy)))
}

// return the shortest x distance between two points, taking into account the map wrapping
// result: WrapX(x1 + distance) = x2
func (mapObject *Map) XDistance(x1 int, x2 int) int {
    abs := func(x int) int {
        if x < 0 {
            return -x
        }

        return x
    }

    value := x2 - x1

    /*
    absValue := value
    if absValue < 0 {
        absValue = -absValue
    }
    */

    // cross over map boundary from x1 towards x2
    value2 := (mapObject.Map.Columns() - x1) + x2

    // cross over map boundary from x2 towards x1
    value3 := -x1 - (mapObject.Map.Columns() - x2)

    if abs(value) < abs(value2) && abs(value) < abs(value3) {
        return value
    } else if abs(value2) < abs(value3) && abs(value2) < abs(value) {
        return value2
    } else {
        return value3
    }
}

func (mapObject *Map) GetTile(tileX int, tileY int) FullTile {
    tileX = mapObject.WrapX(tileX)

    if tileX >= 0 && tileX < mapObject.Map.Columns() && tileY >= 0 && tileY < mapObject.Map.Rows() {
        tile := mapObject.Data.Tiles[mapObject.Map.Terrain[tileX][tileY]].Tile

        extras, ok := mapObject.ExtraMap[image.Pt(tileX, tileY)]
        if !ok {
            extras = make(map[ExtraKind]ExtraTile)
        }

        return FullTile{
            Tile: tile,
            X: tileX,
            Y: tileY,
            Extras: extras,
        }
    }

    return FullTile{
        Tile: terrain.InvalidTile(),
    }
}

func (mapObject *Map) OnShore(x int, y int) bool {
    tile := mapObject.GetTile(x, y)
    return tile.IsTouchingShore(mapObject, false)
}

func (mapObject *Map) ByRiver(x int, y int) bool {
    tile := mapObject.GetTile(x, y)
    return tile.IsByRiver(mapObject)
}

func (mapObject *Map) GetGoldBonus(x int, y int) int {
    tile := mapObject.GetTile(x, y)
    return tile.GoldBonus(mapObject)
}

// Returns city catchment area (5x5 square minus the corners)
func (mapObject *Map) GetCatchmentArea(x int, y int) map[image.Point]FullTile {

    area := make(map[image.Point]FullTile)

    for dx := -2; dx <= 2; dx++ {
        for dy := -2; dy <= 2; dy++ {
            // ignore corners
            if int(math.Abs(float64(dx)) + math.Abs(float64(dy))) == 4 {
                continue
            }

            tileX := mapObject.WrapX(x + dx)
            tileY := y + dy

            tile := mapObject.GetTile(tileX, tileY)
            if tile.Valid() {
                // if any of the tiles are within another city's catchment area, mark this tile as shared

                shared_loop:
                for sharedX := -2; sharedX <= 2; sharedX++ {
                    for sharedY := -2; sharedY <= 2; sharedY++ {
                        // ignore corners
                        if int(math.Abs(float64(sharedX)) + math.Abs(float64(sharedY))) == 4 {
                            continue
                        }

                        sharedTileX := mapObject.WrapX(tileX + sharedX)
                        sharedTileY := tileY + sharedY

                        // if the new tile is the same as the original tile, skip
                        if sharedTileX == x && sharedTileY == y {
                            continue
                        }

                        if mapObject.CityProvider.ContainsCity(sharedTileX, sharedTileY, mapObject.Plane) {
                            tile.IsShared = true
                            break shared_loop
                        }
                    }
                }

                area[image.Pt(tileX, tileY)] = tile
            }
        }
    }

    return area
}

// Returns a square with a side of (radius * 2 + 1), and x, y in the center. May be useful for some spells or area effects
// Radius has to be >= 0
func (mapObject *Map) GetSquareAreaAround(x int, y int, radius int) map[image.Point]FullTile {

    area := make(map[image.Point]FullTile)

    for dx := -radius; dx <= radius; dx++ {
        for dy := -radius; dy <= radius; dy++ {
            tileX := mapObject.WrapX(x + dx)
            tileY := y + dy

            tile := mapObject.GetTile(tileX, tileY)
            if tile.Valid() {
                area[image.Pt(tileX, tileY)] = tile
            }
        }
    }

    return area
}

func (mapObject *Map) TilesPerRow(screenWidth int) int {
    return int(math.Ceil(float64(screenWidth) / float64(mapObject.TileWidth())))
}

func (mapObject *Map) TilesPerColumn(screenHeight int) int {
    return int(math.Ceil(float64(screenHeight) / float64(mapObject.TileHeight())))
}

/* an experiment to draw the entire minimap as a paletted image
*/
