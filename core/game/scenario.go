package game

import (
    "fmt"
    "image"

    buildinglib "github.com/bfalese/civ-of-magic/core/building"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/artifact"
    "github.com/bfalese/civ-of-magic/core/data"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/maplib"
    "github.com/bfalese/civ-of-magic/core/units"
    "github.com/bfalese/civ-of-magic/lib/set"
)

// Scenarios set up a fresh game for tests, so they reach what they test at once instead of
// playing (and retrying) random games until it comes up. HostSettings.Scenario names one.
//
//   battle: a lair guarded by skeletons next to the human's capital (the only encounter next to
//           it), and a wizard who knows Wall of Stone, Disrupt and
//           Fire Bolt with mana and casting skill to spare: one move-stack starts a battle
//   spellbook: a wizard with the spells that take a choice (Spell Ward, Spell Binding,
//           Disjunction, Resurrection) and the item spells (Create Artifact, Enchant Item), mana
//           and casting skill for them this turn, a dead hero, and a known rival with Detect Magic
//   items:  a hero who can wield a sword and a shield at the human's fortress, a plain sword in the vault and a shield just arrived
//           (waiting to be placed), and mana to teleport items
//   siege:  an AI wizard's capital with city walls, a wall of fire and a wall of darkness, and a
//           human army next to it: one move-stack starts a battle at the walls
//   mastery: a wizard who knows the Spell of Mastery with the mana and casting skill to cast it
//           this turn (one cast-spell wins the game)
//   proposal: a known rival who likes the human and is keen on a pact, with the AI diplomacy's
//           dice loaded: the first end-turn brings a treaty-proposal
//   orders: map orders for the stack panel: engineers next to the capital, settlers on a tile
//           where an outpost can go, and swordsmen on an open tower (planar travel)
//   reveal: the whole map explored on both planes (to look at the map: the 3D terrain looks)
//   roads: roads out from the human's capital (east, north-east, south and west, up to 5 tiles
//          over land; the western one enchanted), the land around explored (to look at roads)
func applyScenario(host *GameHost, name string) error {
    switch name {
        case "":
            return nil
        case "battle":
            return scenarioBattle(host)
        case "siege":
            return scenarioSiege(host)
        case "spellbook":
            return scenarioSpellbook(host)
        case "items":
            return scenarioItems(host)
        case "orders":
            return scenarioOrders(host)
        case "mastery":
            return scenarioMastery(host)
        case "proposal":
            return scenarioProposal(host)
        case "reveal":
            host.Human.LiftFogAll(data.PlaneArcanus)
            host.Human.LiftFogAll(data.PlaneMyrror)
            return nil
        case "roads":
            return scenarioRoads(host)
    }
    return fmt.Errorf("unknown scenario %q", name)
}

// roads: roads out from the human's capital in four directions, over land
func scenarioRoads(host *GameHost) error {
    capital := host.Human.FindFortressCity()
    if capital == nil {
        return fmt.Errorf("roads scenario: the human has no capital")
    }
    mapUse := host.Model.GetMap(capital.Plane)
    for _, direction := range []struct{ dx, dy int; enchanted bool }{{1, 0, false}, {1, -1, false}, {0, 1, false}, {-1, 0, true}} {
        x, y := capital.X, capital.Y
        for range 5 {
            x, y = mapUse.WrapX(x + direction.dx), y + direction.dy
            if y < 0 || y >= mapUse.Height() {
                break
            }
            tile := mapUse.GetTile(x, y)
            if !tile.Valid() || !tile.Tile.IsLand() {
                break
            }
            mapUse.SetRoad(x, y, direction.enchanted)
        }
    }
    host.Human.LiftFogSquare(capital.X, capital.Y, 6, capital.Plane)
    return nil
}

// siege: an AI wizard's capital (with its fortress) gets city walls, a wall of fire and a wall
//        of darkness, and a human army of swordsmen stands next to it: one move-stack starts a
//        battle at the walls
func scenarioSiege(host *GameHost) error {
    model := host.Model
    human := host.Human
    var capital *citylib.City
    var enemy *playerlib.Player
    for _, player := range model.Players {
        if player != human && player.AIBehavior != nil && !player.IsNeutral() {
            if city := player.FindFortressCity(); city != nil {
                capital, enemy = city, player
                break
            }
        }
    }
    if capital == nil {
        return fmt.Errorf("siege scenario: no AI wizard with a capital")
    }

    capital.Buildings.Insert(buildinglib.BuildingCityWalls)
    capital.AddEnchantment(data.CityEnchantmentWallOfFire, enemy.GetBanner())
    capital.AddEnchantment(data.CityEnchantmentWallOfDarkness, enemy.GetBanner())
    // someone to fight at the walls
    enemy.AddUnit(units.MakeOverworldUnitFromUnit(units.HighMenSpearmen, capital.X, capital.Y, capital.Plane, enemy.Wizard.Banner, enemy.MakeExperienceInfo(), enemy.MakeUnitEnchantmentProvider()))

    mapUse := model.GetMap(capital.Plane)
    point, ok := scenarioFreeNeighbour(host, mapUse, capital.X, capital.Y)
    if !ok {
        return fmt.Errorf("siege scenario: no free land next to the enemy capital")
    }
    for range 4 {
        human.AddUnit(units.MakeOverworldUnitFromUnit(units.HighMenSwordsmen, point.X, point.Y, capital.Plane, human.Wizard.Banner, human.MakeExperienceInfo(), human.MakeUnitEnchantmentProvider()))
    }
    stack := human.FindStack(point.X, point.Y, capital.Plane)
    for _, unit := range stack.Units() {
        stack.SetActive(unit, true)
    }
    human.LiftFogSquare(point.X, point.Y, 3, capital.Plane)
    return nil
}

func scenarioBattle(host *GameHost) error {
    model := host.Model
    human := host.Human
    capital := human.FindFortressCity()
    if capital == nil {
        return fmt.Errorf("battle scenario: the human has no capital")
    }
    mapUse := model.GetMap(capital.Plane)

    // the game's own encounters next to the capital would be found first (some are unguarded)
    for _, offset := range scenarioNeighbours {
        x, y := mapUse.WrapX(capital.X + offset.X), capital.Y + offset.Y
        if y >= 0 && y < mapUse.Height() && mapUse.GetEncounter(x, y) != nil {
            mapUse.RemoveEncounter(x, y)
        }
    }

    point, ok := scenarioFreeNeighbour(host, mapUse, capital.X, capital.Y)
    if !ok {
        return fmt.Errorf("battle scenario: no free land next to the capital")
    }
    mapUse.ExtraMap[point][maplib.ExtraKindEncounter] = &maplib.ExtraEncounter{
        Type: maplib.EncounterTypeLair,
        Units: []units.Unit{units.Skeleton, units.Skeleton, units.Skeleton},
        ExploredBy: set.MakeSet[maplib.Wizard](),
    }
    human.LiftFogSquare(capital.X, capital.Y, 3, capital.Plane)

    // Fire Bolt for the battle, Disrupt (no walls at a lair: nothing to cast it on), Wall of
    // Stone to cast on the capital from the spellbook
    for _, name := range []string{"Fire Bolt", "Disrupt", "Wall of Stone"} {
        spell := model.AllSpells.FindByName(name)
        if spell.Valid() && !human.KnownSpells.Contains(spell) {
            human.KnownSpells.AddSpell(spell)
        }
    }
    // a casting skill of about 60, available this turn already
    human.Mana = max(human.Mana, 300)
    human.CastingSkillPower = max(human.CastingSkillPower, 4000)
    human.RemainingCastingSkill = human.ComputeOverworldCastingSkill()
    return nil
}

func scenarioSpellbook(host *GameHost) error {
    model := host.Model
    human := host.Human

    for _, name := range []string{"Spell Ward", "Spell Binding", "Disjunction", "Resurrection", "Create Artifact", "Enchant Item", "Fire Bolt"} {
        spell := model.AllSpells.FindByName(name)
        if !spell.Valid() {
            return fmt.Errorf("spellbook scenario: no spell %v", name)
        }
        if !human.KnownSpells.Contains(spell) {
            human.KnownSpells.AddSpell(spell)
        }
    }
    human.Mana = max(human.Mana, 5000)
    human.CastingSkillPower = max(human.CastingSkillPower, 5000000)
    human.RemainingCastingSkill = human.ComputeOverworldCastingSkill()

    // a hero who died, for Resurrection
    dead := false
    for _, hero := range human.HeroPool {
        if hero != nil && hero.Status == herolib.StatusAvailable && hero.HeroType != herolib.HeroTorin {
            hero.SetStatus(herolib.StatusDead)
            dead = true
            break
        }
    }
    if !dead {
        return fmt.Errorf("spellbook scenario: no hero to kill")
    }

    // a rival the human knows, with a global enchantment for Spell Binding / Disjunction
    for _, player := range model.Players {
        if player != human && player.AIBehavior != nil && !player.IsNeutral() {
            human.AwarePlayer(player)
            player.AwarePlayer(human)
            player.GlobalEnchantments.Insert(data.EnchantmentDetectMagic)
            return nil
        }
    }
    return fmt.Errorf("spellbook scenario: no AI wizard")
}

func scenarioMastery(host *GameHost) error {
    human := host.Human
    mastery := host.Model.AllSpells.FindByName("Spell of Mastery")
    if !mastery.Valid() {
        return fmt.Errorf("mastery scenario: no Spell of Mastery")
    }
    human.KnownSpells.AddSpell(mastery)
    cost := human.ComputeEffectiveSpellCost(mastery, true)
    human.Mana = max(human.Mana, cost)
    human.RemainingCastingSkill = max(human.RemainingCastingSkill, cost)
    return nil
}

func scenarioProposal(host *GameHost) error {
    human := host.Human
    for _, player := range host.Model.Players {
        if player != human && player.AIBehavior != nil && !player.IsNeutral() {
            human.AwarePlayer(player)
            player.AwarePlayer(human)
            relationship, _ := player.GetDiplomaticRelation(human)
            relationship.VisibleRelation = 30
            relationship.StartingRelation = 30
            relationship.TreatyInterest = 100
            host.Model.DiplomacyRoll = func(int) int { return 0 }
            return nil
        }
    }
    return fmt.Errorf("proposal scenario: no AI wizard")
}

func scenarioOrders(host *GameHost) error {
    model := host.Model
    human := host.Human
    capital := human.FindFortressCity()
    if capital == nil {
        return fmt.Errorf("orders scenario: the human has no capital")
    }
    plane := capital.Plane
    mapUse := model.GetMap(plane)
    add := func(unit units.Unit, point image.Point) {
        human.AddUnit(units.MakeOverworldUnitFromUnit(unit, point.X, point.Y, plane, human.Wizard.Banner, human.MakeExperienceInfo(), human.MakeUnitEnchantmentProvider()))
    }

    // swordsmen on an open tower, where the other plane has free land too
    other := model.GetMap(plane.Opposite())
    tower := image.Point{}
    found := false
    // nearest first; on some maps the nearest such tile is far from the capital
    for distance := 1; distance <= mapUse.Width() && !found; distance++ {
        for dy := -distance; dy <= distance && !found; dy++ {
            for dx := -distance; dx <= distance && !found; dx++ {
                x, y := mapUse.WrapX(capital.X + dx), capital.Y + dy
                if max(abs(dx), abs(dy)) == distance && scenarioTileFree(host, mapUse, x, y) && scenarioTileFree(host, other, x, y) {
                    tower, found = image.Pt(x, y), true
                }
            }
        }
    }
    if !found {
        return fmt.Errorf("orders scenario: no place for a tower")
    }
    mapUse.SetPlaneTower(tower.X, tower.Y)
    other.SetPlaneTower(tower.X, tower.Y)
    add(units.HighMenSwordsmen, tower)

    engineers, ok := scenarioFreeNeighbour(host, mapUse, capital.X, capital.Y)
    if !ok {
        return fmt.Errorf("orders scenario: no free land next to the capital")
    }
    add(units.HighMenEngineer, engineers)

    // settlers on the nearest tile where an outpost can go
    settlers := image.Point{}
    found = false
    for distance := 4; distance <= mapUse.Width() / 2 && !found; distance++ {
        for dy := -distance; dy <= distance && !found; dy++ {
            for dx := -distance; dx <= distance && !found; dx++ {
                x, y := mapUse.WrapX(capital.X + dx), capital.Y + dy
                if y < 0 || y >= mapUse.Height() || max(abs(dx), abs(dy)) != distance {
                    continue
                }
                tile := mapUse.GetTile(x, y)
                if tile.Valid() && tile.Tile.IsLand() && model.IsSettlableLocation(x, y, plane) && human.FindStack(x, y, plane) == nil {
                    occupied := false
                    for _, player := range model.Players {
                        if player.FindStack(x, y, plane) != nil {
                            occupied = true
                        }
                    }
                    if !occupied {
                        settlers, found = image.Pt(x, y), true
                    }
                }
            }
        }
    }
    if !found {
        return fmt.Errorf("orders scenario: nowhere to settle")
    }
    add(units.HighMenSettlers, settlers)

    human.LiftFogSquare(capital.X, capital.Y, 3, plane)
    human.LiftFogSquare(settlers.X, settlers.Y, 2, plane)
    human.LiftFogSquare(tower.X, tower.Y, 1, plane)
    human.LiftFogSquare(tower.X, tower.Y, 1, plane.Opposite())
    return nil
}

// whether one of the hero's equipment slots takes the item type
func heroTakes(hero *herolib.Hero, kind artifact.ArtifactType) bool {
    for _, slot := range hero.GetArtifactSlots() {
        if slot.CompatibleWith(kind) {
            return true
        }
    }
    return false
}

func scenarioItems(host *GameHost) error {
    model := host.Model
    human := host.Human

    hired := false
    for _, hero := range human.HeroPool {
        if hero != nil && hero.Status == herolib.StatusAvailable && hero.HeroType != herolib.HeroTorin && heroTakes(hero, artifact.ArtifactTypeSword) && heroTakes(hero, artifact.ArtifactTypeShield) {
            if !human.AddHeroToFortress(hero) {
                return fmt.Errorf("items scenario: the hero didn't join")
            }
            hero.SetStatus(herolib.StatusEmployed)
            hired = true
            break
        }
    }
    if !hired {
        return fmt.Errorf("items scenario: no hero to hire")
    }

    makeItem := func(kind string, name string) (*artifact.Artifact, error) {
        return artifact.BuildDesign(artifact.Design{Type: kind, Name: name}, false, &human.Wizard, human.KnownSpells, nil, nil)
    }
    sword, err := makeItem("Sword", "Scenario Sword")
    if err != nil {
        return err
    }
    human.VaultEquipment[0] = sword
    shield, err := makeItem("Shield", "Scenario Shield")
    if err != nil {
        return err
    }
    model.recordVaultItem(&GameEventVault{CreatedArtifact: shield, Player: human})
    human.Mana = max(human.Mana, 200)
    return nil
}

var scenarioNeighbours = []image.Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, -1}, {1, -1}, {-1, 1}}

// whether (x, y) is land with nothing on it: no city, stack, encounter or other extra (and its
// extras map exists, for adding one)
func scenarioTileFree(host *GameHost, mapUse *maplib.Map, x int, y int) bool {
    if y < 0 || y >= mapUse.Height() {
        return false
    }
    tile := mapUse.GetTile(x, y)
    if !tile.Valid() || !tile.Tile.IsLand() || len(tile.Extras) > 0 || host.Model.ContainsCity(x, y, mapUse.Plane) {
        return false
    }
    for _, player := range host.Model.Players {
        if player.FindStack(x, y, mapUse.Plane) != nil {
            return false
        }
    }
    if mapUse.ExtraMap[image.Pt(x, y)] == nil {
        mapUse.ExtraMap[image.Pt(x, y)] = make(map[maplib.ExtraKind]maplib.ExtraTile)
    }
    return true
}

// a land tile next to (x, y) with nothing on it: no city, stack, encounter or other extra
func scenarioFreeNeighbour(host *GameHost, mapUse *maplib.Map, x int, y int) (image.Point, bool) {
    model := host.Model
    for _, offset := range scenarioNeighbours {
        tileX := mapUse.WrapX(x + offset.X)
        tileY := y + offset.Y
        if tileY < 0 || tileY >= mapUse.Height() {
            continue
        }
        tile := mapUse.GetTile(tileX, tileY)
        if !tile.Valid() || !tile.Tile.IsLand() || len(tile.Extras) > 0 || model.ContainsCity(tileX, tileY, mapUse.Plane) {
            continue
        }
        occupied := false
        for _, player := range model.Players {
            if player.FindStack(tileX, tileY, mapUse.Plane) != nil {
                occupied = true
            }
        }
        if occupied {
            continue
        }
        if mapUse.ExtraMap[image.Pt(tileX, tileY)] == nil {
            mapUse.ExtraMap[image.Pt(tileX, tileY)] = make(map[maplib.ExtraKind]maplib.ExtraTile)
        }
        return image.Pt(tileX, tileY), true
    }
    return image.Point{}, false
}
