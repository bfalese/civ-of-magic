package game

import (
    "encoding/json"
    "io"
    "log"
    "os"
    "slices"
    "strings"
    "testing"

    "github.com/bfalese/civ-of-magic/core/artifact"
    "github.com/bfalese/civ-of-magic/core/combat"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/gamedata"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/spellbook"
)

func TestHostSettings(test *testing.T) {
    settings, err := DecodeHostSettings(nil)
    if err != nil || settings.Opponents != 3 || !settings.Raiders {
        test.Errorf("defaults: %+v %v", settings, err)
    }

    settings, err = DecodeHostSettings([]byte(`{"opponents": 2, "difficulty": "hard", "wizardName": "Tester"}`))
    if err != nil || settings.Opponents != 2 || settings.Difficulty != "hard" || settings.WizardName != "Tester" {
        test.Errorf("decoded: %+v %v", settings, err)
    }

    if _, err := parseDifficulty("nightmare"); err == nil {
        test.Errorf("unknown difficulty should be refused")
    }
}

// a human player through JSON commands for a few decades of AI turns. Needs the exported data
// tables: CIV_GAMEDATA=<dir with gamedata.json>; skipped otherwise.
func TestGameHost(test *testing.T) {
    dataPath := os.Getenv("CIV_GAMEDATA")
    if dataPath == "" {
        test.Skip("CIV_GAMEDATA not set")
    }

    gameData, err := gamedata.Load(dataPath)
    if err != nil {
        test.Fatalf("load: %v", err)
    }

    if !testing.Verbose() {
        log.SetOutput(io.Discard)
        defer log.SetOutput(os.Stderr)
    }

    host, err := MakeGameHost(gameData, HostSettings{Opponents: 2, Raiders: true, WizardName: "Tester"})
    if err != nil {
        test.Fatalf("setup: %v", err)
    }

    if host.Human.Wizard.Name != "Tester" || len(host.Human.Cities) != 1 || len(host.Human.Stacks) == 0 {
        test.Fatalf("the human should start with a capital and units")
    }
    if host.Model.CurrentPlayer != 0 {
        test.Fatalf("the human plays first")
    }

    view, err := host.PlayerView()
    if err != nil {
        test.Fatalf("player view: %v", err)
    }
    var decoded map[string]any
    if err := json.Unmarshal(view, &decoded); err != nil {
        test.Fatalf("player view JSON: %v", err)
    }
    // the magic screen's data, and the first research question at once
    you := host.Model.MakePlayerView(host.Human).You
    share := you.PowerDistribution
    if len(you.Spells) == 0 || len(you.ResearchCandidates) == 0 || you.Power <= 0 || share.Mana + share.Research + share.Skill < 0.99 {
        test.Errorf("magic view: %v spells, %v candidates, power %v, shares %+v", len(you.Spells), len(you.ResearchCandidates), you.Power, share)
    }
    askedResearch := false
    for _, encoded := range host.TakeEvents() {
        if strings.Contains(string(encoded), `"choose-research"`) {
            askedResearch = true
        }
    }
    if !askedResearch {
        test.Errorf("a new game should ask for the first research")
    }

    if _, err := host.MapView(data.PlaneArcanus); err != nil {
        test.Errorf("map view: %v", err)
    }
    if _, err := host.BattleView(); err == nil {
        test.Errorf("no battle yet")
    }

    if err := host.ExecuteJSON([]byte(`{"type": "set-tax-rate", "command": {"numerator": 1, "denominator": 1}}`)); err != nil {
        test.Errorf("tax: %v", err)
    }
    if err := host.ExecuteJSON([]byte(`{"type": "no-such-command", "command": {}}`)); err == nil {
        test.Errorf("unknown commands should be refused")
    }

    events := len(host.TakeEvents())
    start := host.Model.TurnNumber
    attacked := 0
    for range 30 {
        if host.GameOver() != "" {
            break
        }
        // answer anything left waiting (a finished spell's target is optional)
        if host.WaitingFor() != WaitingNothing {
            test.Fatalf("waiting for %v outside a command", host.WaitingFor())
        }
        if err := host.ExecuteJSON([]byte(`{"type": "end-turn", "command": {}}`)); err != nil {
            test.Fatalf("end turn at %v: %v", host.Model.TurnNumber, err)
        }
        // an AI player attacked: the AI turns wait for the battle
        for host.WaitingFor() == WaitingBattle {
            attacked += 1
            if err := host.Execute(&BattleAutoCommand{}); err != nil {
                test.Fatalf("auto battle: %v", err)
            }
        }
        if host.GameOver() == "" && host.Model.CurrentPlayer != 0 {
            test.Fatalf("after end-turn it should be the human's turn again, not player %v", host.Model.CurrentPlayer)
        }
        events += len(host.TakeEvents())
    }

    if host.Model.TurnNumber < start + 30 && host.GameOver() == "" {
        test.Errorf("expected 30 turns, played %v", host.Model.TurnNumber - start)
    }
    test.Logf("turn %v, %v events, game over %q, human cities %v, attacked %v times", host.Model.TurnNumber, events, host.GameOver(), len(host.Human.Cities), attacked)

    for _, err := range host.EventErrors {
        test.Errorf("event: %v", err)
    }
    if events == 0 {
        test.Errorf("expected some events")
    }
}

// an AI player's attack on the human during end-turn pauses the AI turns for the battle; the rest
// of the turn runs once the battle is over
func TestHostAiBattle(test *testing.T) {
    model, human, enemy := makeSessionTestModel()
    host := &GameHost{Model: model, Human: human, ai: &HeadlessGame{Model: model}}
    host.aiHandlers = &hostAiHandlers{HeadlessGame: host.ai, host: host}
    host.Session = MakeCommandSession(model, human)

    // the end of the turn: the enemy's spearmen attack the human's drakes, then the turn goes on
    finished := false
    var result combat.CombatState
    host.Session.EndTurn = func() {
        attackers := enemy.FindStack(4, 3, data.PlaneArcanus)
        defenders := human.FindStack(3, 3, data.PlaneArcanus)
        result = host.aiHandlers.DoCombat(enemy, attackers, human, defenders, combat.ZoneType{})
        finished = true
    }

    if err := host.Execute(&EndTurnCommand{}); err != nil {
        test.Fatalf("end turn: %v", err)
    }
    if host.WaitingFor() != WaitingBattle || finished {
        test.Fatalf("the attack should wait for the human's orders, waiting for %q", host.WaitingFor())
    }
    encoded, err := host.BattleView()
    if err != nil {
        test.Fatalf("battle view: %v", err)
    }
    var view BattleView
    if err := json.Unmarshal(encoded, &view); err != nil {
        test.Fatalf("battle view JSON: %v", err)
    }
    if view.YouAttack {
        test.Errorf("the human defends")
    }
    if err := host.Execute(&EndTurnCommand{}); err == nil {
        test.Errorf("other commands should wait for the battle")
    }

    // a few orders by hand, then the AI fights it out for the human
    for range 3 {
        if host.WaitingFor() != WaitingBattle {
            break
        }
        if err := host.Execute(&BattleDoneCommand{}); err != nil {
            test.Fatalf("done: %v", err)
        }
    }
    if host.WaitingFor() == WaitingBattle {
        if err := host.Execute(&BattleAutoCommand{}); err != nil {
            test.Fatalf("auto: %v", err)
        }
    }
    if host.WaitingFor() != WaitingNothing || !finished {
        test.Fatalf("after the battle the turn should finish, waiting for %q", host.WaitingFor())
    }
    if result != combat.CombatStateDefenderWin {
        test.Errorf("four great drakes should hold against spearmen, got %v", battleStateName(result))
    }
    if len(enemy.Stacks) != 0 || len(human.Stacks) == 0 {
        test.Errorf("the attackers should be gone, the drakes still there")
    }
}

// the battle scenario: a guarded lair next to the capital and Fire Bolt to cast there (10 games,
// since the maps are random)
func TestHostBattleScenario(test *testing.T) {
    dataPath := os.Getenv("CIV_GAMEDATA")
    if dataPath == "" {
        test.Skip("CIV_GAMEDATA not set")
    }
    gameData, err := gamedata.Load(dataPath)
    if err != nil {
        test.Fatalf("load: %v", err)
    }
    if !testing.Verbose() {
        log.SetOutput(io.Discard)
        defer log.SetOutput(os.Stderr)
    }

    if _, err := MakeGameHost(gameData, HostSettings{Opponents: 1, Scenario: "nonsense"}); err == nil {
        test.Errorf("an unknown scenario should be refused")
    }

    for range 10 {
        host, err := MakeGameHost(gameData, HostSettings{Opponents: 1, Scenario: "battle"})
        if err != nil {
            test.Fatalf("setup: %v", err)
        }
        capital := host.Human.FindFortressCity()
        mapUse := host.Model.GetMap(capital.Plane)
        var lair *Location
        for _, offset := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {-1, -1}, {1, -1}, {-1, 1}} {
            x, y := mapUse.WrapX(capital.X + offset[0]), capital.Y + offset[1]
            if mapUse.GetEncounter(x, y) != nil {
                lair = &Location{Plane: capital.Plane, X: x, Y: y}
            }
        }
        if lair == nil {
            test.Fatalf("no lair next to the capital")
        }

        stack := host.Human.FindStack(capital.X, capital.Y, capital.Plane)
        if err := host.Execute(&MoveStackCommand{From: Location{Plane: capital.Plane, X: capital.X, Y: capital.Y}, To: *lair}); err != nil {
            test.Fatalf("move: %v", err)
        }
        if host.WaitingFor() != WaitingEncounter || stack == nil {
            test.Fatalf("walking into the lair should ask first, waiting for %q", host.WaitingFor())
        }
        if err := host.Execute(&ConfirmEncounterCommand{Enter: true}); err != nil {
            test.Fatalf("enter: %v", err)
        }
        if host.WaitingFor() != WaitingBattle {
            test.Fatalf("entering should start a battle, waiting for %q", host.WaitingFor())
        }
        view := host.Session.Battle.View()
        castable := false
        for _, spell := range view.Magic.Spells {
            if spell.Name == "Fire Bolt" && spell.Castable {
                castable = true
            }
        }
        if !castable {
            test.Fatalf("fire bolt should be castable: %+v", view.Magic)
        }
        // no walls at a lair: Disrupt has nothing to hit
        for _, spell := range view.Magic.Spells {
            if spell.Name == "Disrupt" && (spell.Castable || spell.Unusable != "nothing to cast it on here") {
                test.Errorf("Disrupt at a lair: %+v", spell)
            }
        }
        if err := host.Execute(&BattleCastCommand{Spell: "Disrupt"}); err == nil || host.Session.Battle.View().Casting != nil {
            test.Errorf("casting Disrupt without walls should be refused: %v", err)
        }

        // Fire Bolt on a skeleton: the step names the target, its tile and the effect to play
        var enemy *BattleUnitView
        for i := range view.Units {
            if !view.Units[i].Yours {
                enemy = &view.Units[i]
                break
            }
        }
        if enemy == nil {
            test.Fatalf("no enemy on the field")
        }
        enemyId := enemy.Id
        if err := host.Execute(&BattleCastCommand{Spell: "Fire Bolt", Target: &BattleTarget{Unit: &enemyId}}); err != nil {
            test.Fatalf("Fire Bolt: %v", err)
        }
        if host.WaitingFor() == WaitingBattle {
            var spellStep *BattleStepView
            for _, step := range host.Session.Battle.View().Steps {
                if step.Kind == "spell" {
                    spellStep = &step
                }
            }
            if spellStep == nil || spellStep.Effect != "spell-effect/fire-bolt" || spellStep.Target == nil || *spellStep.Target != enemyId || spellStep.Tile == nil || *spellStep.Tile != [2]int{enemy.X, enemy.Y} {
                test.Errorf("the Fire Bolt step: %+v", spellStep)
            }
        } else {
            continue
        }
        if err := host.Execute(&BattleAutoCommand{}); err != nil {
            test.Fatalf("auto: %v", err)
        }
    }
}

// the siege scenario: a human army next to an AI capital with walls, a wall of fire and darkness
func TestHostSiegeScenario(test *testing.T) {
    dataPath := os.Getenv("CIV_GAMEDATA")
    if dataPath == "" {
        test.Skip("CIV_GAMEDATA not set")
    }
    gameData, err := gamedata.Load(dataPath)
    if err != nil {
        test.Fatalf("load: %v", err)
    }
    if !testing.Verbose() {
        log.SetOutput(io.Discard)
        defer log.SetOutput(os.Stderr)
    }

    for range 10 {
        host, err := MakeGameHost(gameData, HostSettings{Opponents: 1, Scenario: "siege"})
        if err != nil {
            test.Fatalf("setup: %v", err)
        }
        // the scenario's army: a stack of the human's next to an enemy city
        var army *playerlib.UnitStack
        var target *Location
        for _, stack := range host.Human.Stacks {
            for _, player := range host.Model.Players {
                if player == host.Human {
                    continue
                }
                for _, city := range player.Cities {
                    // the map wraps east-west
                    width := host.Model.GetMap(city.Plane).Width()
                    dx := min(abs(city.X - stack.X()), width - abs(city.X - stack.X()))
                    if city.Plane == stack.Plane() && max(dx, abs(city.Y - stack.Y())) <= 1 {
                        army, target = stack, &Location{Plane: city.Plane, X: city.X, Y: city.Y}
                    }
                }
            }
        }
        if army == nil {
            test.Fatalf("no army of the human's next to an enemy city")
        }

        if err := host.Execute(&MoveStackCommand{From: Location{Plane: army.Plane(), X: army.X(), Y: army.Y()}, To: *target}); err != nil {
            test.Fatalf("move: %v", err)
        }
        if host.WaitingFor() != WaitingBattle {
            test.Fatalf("attacking the city should start a battle, waiting for %q", host.WaitingFor())
        }
        view := host.Session.Battle.View()
        stone, animated := 0, 0
        for _, row := range view.Tiles {
            for _, tile := range row {
                for _, wall := range tile.Walls {
                    if wall.Animated {
                        animated += 1
                    } else {
                        stone += 1
                    }
                }
            }
        }
        if stone == 0 || animated == 0 || view.Fortress == "" {
            test.Fatalf("walls %v, fire/darkness %v, fortress %q", stone, animated, view.Fortress)
        }
        if err := host.Execute(&BattleFleeCommand{}); err != nil {
            test.Fatalf("flee: %v", err)
        }
        if host.WaitingFor() == WaitingBattle {
            test.Fatalf("fleeing should end the battle")
        }
    }
}

// the spellbook scenario: spells cast with a choice from the view's own list, and an item made
// from the view's design options
func TestHostSpellbookScenario(test *testing.T) {
    dataPath := os.Getenv("CIV_GAMEDATA")
    if dataPath == "" {
        test.Skip("CIV_GAMEDATA not set")
    }
    gameData, err := gamedata.Load(dataPath)
    if err != nil {
        test.Fatalf("load: %v", err)
    }
    if !testing.Verbose() {
        log.SetOutput(io.Discard)
        defer log.SetOutput(os.Stderr)
    }

    host, err := MakeGameHost(gameData, HostSettings{Opponents: 1, Scenario: "spellbook"})
    if err != nil {
        test.Fatalf("setup: %v", err)
    }
    host.TakeEvents()

    spellView := func(name string) SpellView {
        for _, spell := range host.Model.MakePlayerView(host.Human).You.Spells {
            if spell.Name == name {
                return spell
            }
        }
        test.Fatalf("no %v in the spellbook", name)
        return SpellView{}
    }
    castChoice := func(name string, choice SpellChoiceView) {
        target := &CommandSpellTarget{Choice: choice.Name, Location: choice.Location, Wizard: choice.Wizard}
        if err := host.Execute(&CastSpellCommand{Spell: name, Target: target}); err != nil {
            test.Fatalf("cast %v on %+v: %v", name, choice, err)
        }
    }

    for _, name := range []string{"Spell Ward", "Spell Binding", "Disjunction", "Resurrection"} {
        spell := spellView(name)
        if !spell.ChoiceSpell || len(spell.Choices) == 0 || !spell.Now {
            test.Fatalf("%v: %+v", name, spell)
        }
    }
    if spellView("Create Artifact").ChoiceSpell {
        test.Errorf("Create Artifact takes no choice")
    }

    ward := spellView("Spell Ward").Choices[0]
    castChoice("Spell Ward", ward)
    city := host.Human.FindCity(ward.Location.X, ward.Location.Y, ward.Location.Plane)
    warded := false
    if city != nil {
        for _, enchantment := range city.Enchantments.Values() {
            warded = warded || enchantment.Enchantment.Name() == ward.Name
        }
    }
    if !warded {
        test.Errorf("Spell Ward didn't put %v on the city", ward.Name)
    }
    for _, choice := range spellView("Spell Ward").Choices {
        if choice.City == ward.City && choice.Name == ward.Name {
            test.Errorf("%v is still offered for %v", ward.Name, ward.City)
        }
    }

    hero := spellView("Resurrection").Choices[0]
    castChoice("Resurrection", hero)
    if len(spellView("Resurrection").Choices) != 0 {
        test.Errorf("the hero is still dead")
    }

    binding := spellView("Spell Binding").Choices[0]
    if binding.Wizard == "" || binding.Name != "Detect Magic" {
        test.Errorf("Spell Binding choice: %+v", binding)
    }
    castChoice("Spell Binding", binding)
    if host.Human.GlobalEnchantments.Contains(data.EnchantmentDetectMagic) && len(spellView("Spell Binding").Choices) != 0 {
        test.Errorf("Detect Magic is still offered after taking it")
    }

    if err := host.Execute(&CastSpellCommand{Spell: "Resurrection", Target: &CommandSpellTarget{Choice: "Nobody"}}); err == nil {
        test.Errorf("a choice that isn't offered should be refused")
    }

    // an item from the design options: a sword with the first attack power it can have
    create := spellView("Create Artifact")
    design := host.Model.MakePlayerView(host.Human).You.ItemDesign
    if !create.NeedsDesign || create.CostFactor <= 0 || design == nil || len(design.Types) != 10 || len(design.Powers) == 0 {
        test.Fatalf("item design: %+v %+v", create, design)
    }
    var sword ItemTypeView
    for _, kind := range design.Types {
        if kind.Name == "Sword" {
            sword = kind
        }
    }
    if len(sword.Images) == 0 || sword.Images[0].Asset != "item/0" {
        test.Errorf("sword pictures: %+v", sword.Images)
    }
    power := ""
    powerCost := 0
    for _, candidate := range design.Powers {
        if candidate.Group == "Attack" && slices.Contains(candidate.Types, "Sword") {
            power, powerCost = candidate.Name, candidate.Cost
            break
        }
    }
    if power == "" {
        test.Fatalf("no attack power for a sword")
    }
    manaBefore := host.Human.Mana
    host.TakeEvents()
    if err := host.Execute(&CastSpellCommand{Spell: "Create Artifact", Artifact: &artifact.Design{Type: "Sword", Image: sword.Images[1].Image, Powers: []string{power}}}); err != nil {
        test.Fatalf("Create Artifact: %v", err)
    }
    expected := int(float64(sword.BaseCost + powerCost) * create.CostFactor)
    if paid := manaBefore - host.Human.Mana; paid < expected - 1 || paid > expected + 1 {
        test.Errorf("paid %v for the sword, the view's prices say %v", paid, expected)
    }
    vault := false
    for _, encoded := range host.TakeEvents() {
        if strings.Contains(string(encoded), `"vault"`) {
            vault = true
        }
    }
    if !vault {
        test.Errorf("the sword didn't go to the vault")
    }
}

// the items scenario: place a new item and a vault item on a hero, destroy one, and split and
// disband units through the view's ids
func TestHostItemsScenario(test *testing.T) {
    dataPath := os.Getenv("CIV_GAMEDATA")
    if dataPath == "" {
        test.Skip("CIV_GAMEDATA not set")
    }
    gameData, err := gamedata.Load(dataPath)
    if err != nil {
        test.Fatalf("load: %v", err)
    }
    if !testing.Verbose() {
        log.SetOutput(io.Discard)
        defer log.SetOutput(os.Stderr)
    }

    host, err := MakeGameHost(gameData, HostSettings{Opponents: 1, Scenario: "items"})
    if err != nil {
        test.Fatalf("setup: %v", err)
    }
    items := func() VaultView {
        return host.Model.MakePlayerView(host.Human).You.Items
    }
    view := items()
    if len(view.Vault) != 4 || view.Vault[0] == nil || view.Vault[0].Type != "Sword" || view.Vault[1] != nil {
        test.Fatalf("vault: %+v", view.Vault)
    }
    if len(view.New) != 1 || view.New[0].Type != "Shield" || view.New[0].Asset == "" {
        test.Fatalf("new items: %+v", view.New)
    }
    if len(view.Heroes) != 1 || len(view.Heroes[0].Slots) != 3 || view.Fortress == nil || view.Heroes[0].Location != *view.Fortress {
        test.Fatalf("heroes: %+v", view.Heroes)
    }
    hero := view.Heroes[0]
    slotFor := func(kind string) int {
        for index, slot := range hero.Slots {
            if slot.Item == nil && slices.Contains(slot.Takes, kind) {
                return index
            }
        }
        return -1
    }
    armor, weapon := slotFor("Shield"), slotFor("Sword")
    if armor < 0 || weapon < 0 {
        test.Fatalf("%v has no slot for a shield or a sword: %+v", hero.Name, hero.Slots)
    }

    zero, heroIndex := 0, hero.Index
    mana := host.Human.Mana
    if err := host.Execute(&MoveItemCommand{From: ItemSlot{New: &zero}, To: ItemSlot{Hero: &heroIndex, Slot: armor}}); err != nil {
        test.Fatalf("place the shield: %v", err)
    }
    if err := host.Execute(&MoveItemCommand{From: ItemSlot{Vault: &zero}, To: ItemSlot{Hero: &heroIndex, Slot: weapon}}); err != nil {
        test.Fatalf("equip the sword: %v", err)
    }
    view = items()
    hero = view.Heroes[0]
    if len(view.New) != 0 || view.Vault[0] != nil || hero.Slots[armor].Item == nil || hero.Slots[weapon].Item == nil || host.Human.Mana != mana {
        test.Errorf("after equipping at the fortress (free): new %v, vault %v, slots %+v, mana %v -> %v", view.New, view.Vault[0], hero.Slots, mana, host.Human.Mana)
    }
    if err := host.Execute(&MoveItemCommand{From: ItemSlot{Hero: &heroIndex, Slot: weapon}, To: ItemSlot{Hero: &heroIndex, Slot: armor}}); err == nil {
        test.Errorf("a sword shouldn't fit the armor slot")
    }

    value := hero.Slots[weapon].Item.Value
    if err := host.Execute(&DestroyItemCommand{Item: ItemSlot{Hero: &heroIndex, Slot: weapon}}); err != nil {
        test.Fatalf("destroy: %v", err)
    }
    if host.Human.Mana != mana + value || items().Heroes[0].Slots[weapon].Item != nil {
        test.Errorf("destroying the sword should give %v mana: %v -> %v", value, mana, host.Human.Mana)
    }

    // the hero and the garrison share the fortress tile: leave the hero behind, then disband another unit
    var stack StackView
    for _, candidate := range host.Model.MakePlayerView(host.Human).Stacks {
        if candidate.Location == hero.Location {
            stack = candidate
        }
    }
    if len(stack.Units) < 2 {
        test.Fatalf("the fortress stack: %+v", stack)
    }
    for _, unit := range stack.Units {
        if !unit.Active || unit.Details.Defense <= 0 || unit.Details.Figures <= 0 {
            test.Errorf("unit %v: active %v, details %+v", unit.Name, unit.Active, unit.Details)
        }
    }
    if err := host.Execute(&SetUnitActiveCommand{Unit: hero.Unit, Active: false}); err != nil {
        test.Fatalf("set-unit-active: %v", err)
    }
    other := -1
    for _, candidate := range host.Model.MakePlayerView(host.Human).Stacks {
        for _, unit := range candidate.Units {
            if unit.Id == hero.Unit && unit.Active {
                test.Errorf("the hero is still active")
            }
            if unit.Id != hero.Unit && candidate.Location == hero.Location {
                other = unit.Id
            }
        }
    }
    if err := host.Execute(&DisbandUnitCommand{Unit: other}); err != nil {
        test.Fatalf("disband: %v", err)
    }
    if _, _, err := host.Model.FindUnitById(host.Human, other); err == nil {
        test.Errorf("the disbanded unit is still there")
    }
}

// city screen extras: tax rates, renaming, and cancelling your own enchantments (the spellbook
// scenario casts a ward on a city of yours; the rival's Detect Magic isn't yours to cancel)
func TestHostCityExtras(test *testing.T) {
    dataPath := os.Getenv("CIV_GAMEDATA")
    if dataPath == "" {
        test.Skip("CIV_GAMEDATA not set")
    }
    gameData, err := gamedata.Load(dataPath)
    if err != nil {
        test.Fatalf("load: %v", err)
    }
    if !testing.Verbose() {
        log.SetOutput(io.Discard)
        defer log.SetOutput(os.Stderr)
    }

    host, err := MakeGameHost(gameData, HostSettings{Opponents: 1, Scenario: "spellbook"})
    if err != nil {
        test.Fatalf("setup: %v", err)
    }
    view := host.Model.MakePlayerView(host.Human)

    rates := view.You.TaxRates
    if len(rates) != 7 || rates[0].Label != "0" || rates[0].Unrest != 0 || rates[6].Label != "3" || rates[6].Unrest != 0.75 || rates[2].Label != "1" {
        test.Fatalf("tax rates: %+v", rates)
    }
    for _, rate := range rates {
        if err := host.Execute(&SetTaxRateCommand{Numerator: rate.Numerator, Denominator: rate.Denominator}); err != nil {
            test.Errorf("tax rate %+v: %v", rate, err)
        }
    }
    if host.Model.MakePlayerView(host.Human).You.TaxRate != "3/1" {
        test.Errorf("tax rate now %v", host.Model.MakePlayerView(host.Human).You.TaxRate)
    }

    city := view.Cities[0]
    if err := host.Execute(&RenameCityCommand{City: city.Location, Name: "Testburg"}); err != nil {
        test.Fatalf("rename: %v", err)
    }
    if host.Model.MakePlayerView(host.Human).Cities[0].Name != "Testburg" {
        test.Errorf("the city wasn't renamed")
    }

    ward := ""
    for _, spell := range view.You.Spells {
        if spell.Name == "Spell Ward" {
            for _, choice := range spell.Choices {
                if *choice.Location == city.Location {
                    ward = choice.Name
                }
            }
        }
    }
    if err := host.Execute(&CastSpellCommand{Spell: "Spell Ward", Target: &CommandSpellTarget{Location: &city.Location, Choice: ward}}); err != nil {
        test.Fatalf("Spell Ward: %v", err)
    }
    enchantments := host.Model.MakePlayerView(host.Human).Cities[0].Enchantments
    if len(enchantments) != 1 || enchantments[0].Name != ward || !enchantments[0].Yours || enchantments[0].Upkeep <= 0 {
        test.Fatalf("city enchantments: %+v", enchantments)
    }
    if err := host.Execute(&CancelEnchantmentCommand{Enchantment: ward, City: &city.Location}); err != nil {
        test.Fatalf("cancel %v: %v", ward, err)
    }
    if len(host.Model.MakePlayerView(host.Human).Cities[0].Enchantments) != 0 {
        test.Errorf("%v is still on the city", ward)
    }

    // a global enchantment of yours, cancelled; Detect Magic of the rival's isn't yours
    host.Human.GlobalEnchantments.Insert(data.EnchantmentNatureAwareness)
    globals := host.Model.MakePlayerView(host.Human).You.GlobalEnchantments
    if len(globals) != 1 || globals[0].Name != "Nature Awareness" || !globals[0].Yours {
        test.Fatalf("global enchantments: %+v", globals)
    }
    if err := host.Execute(&CancelEnchantmentCommand{Enchantment: "Nature Awareness"}); err != nil || host.Human.GlobalEnchantments.Contains(data.EnchantmentNatureAwareness) {
        test.Errorf("cancel Nature Awareness: %v", err)
    }
    if err := host.Execute(&CancelEnchantmentCommand{Enchantment: "Detect Magic"}); err == nil {
        test.Errorf("cancelled a global enchantment you don't have")
    }

    // through JSON, as the page sends it
    encoded := []byte(`{"type": "set-tax-rate", "command": {"numerator": 1, "denominator": 2}}`)
    if err := host.ExecuteJSON(encoded); err != nil || host.Model.MakePlayerView(host.Human).You.TaxRate != "1/2" {
        test.Errorf("set-tax-rate from JSON: %v", err)
    }
}

// diplomacy with the spellbook scenario's known rival: a pact, tributes and a spell trade, all
// from the view's own lists
func TestHostDiplomacy(test *testing.T) {
    dataPath := os.Getenv("CIV_GAMEDATA")
    if dataPath == "" {
        test.Skip("CIV_GAMEDATA not set")
    }
    gameData, err := gamedata.Load(dataPath)
    if err != nil {
        test.Fatalf("load: %v", err)
    }
    if !testing.Verbose() {
        log.SetOutput(io.Discard)
        defer log.SetOutput(os.Stderr)
    }

    host, err := MakeGameHost(gameData, HostSettings{Opponents: 1, Scenario: "spellbook"})
    if err != nil {
        test.Fatalf("setup: %v", err)
    }
    human := host.Human
    human.Gold = 1000
    rival := func() DiplomacyView {
        views := host.Model.MakePlayerView(human).Diplomacy
        if len(views) != 1 {
            test.Fatalf("diplomacy: %+v", views)
        }
        return views[0]
    }
    view := rival()
    if view.Mood == "" || view.Treaty != "none" || view.TributeGold != 100 || len(view.Spells) == 0 || len(view.GlobalEnchantments) != 1 {
        test.Fatalf("the rival: %+v", view)
    }
    banner := view.Wizard.Banner

    if err := host.Execute(&ProposeTreatyCommand{Wizard: banner, Treaty: "pact"}); err != nil || rival().Treaty != "pact" {
        test.Errorf("pact: %v, treaty %v", err, rival().Treaty)
    }

    // a tribute raises how they'll feel (the visible mood drifts there over the next turns)
    var theirs *playerlib.Relationship
    for _, other := range host.Model.Players {
        if other.GetBanner().String() == banner {
            theirs, _ = other.GetDiplomaticRelation(human)
        }
    }
    baseline := theirs.StartingRelation
    if err := host.Execute(&OfferTributeCommand{Wizard: banner, Gold: true}); err != nil || human.Gold != 900 {
        test.Errorf("gold tribute: %v, gold %v", err, human.Gold)
    }
    if theirs.StartingRelation <= baseline && baseline < 100 {
        test.Errorf("the tribute didn't please them: %v -> %v", baseline, theirs.StartingRelation)
    }
    gift := rival().Spells[0]
    if err := host.Execute(&OfferTributeCommand{Wizard: banner, Spell: gift}); err != nil || slices.Contains(rival().Spells, gift) {
        test.Errorf("spell tribute %v: %v", gift, err)
    }

    // a trade they're sure to want
    for _, other := range host.Model.Players {
        if other.GetBanner().String() == banner {
            if relationship, ok := other.GetDiplomaticRelation(human); ok {
                relationship.TradeInterest = 100
            }
            // something to teach
            for _, spell := range host.Model.AllSpells.Spells {
                if !human.KnownSpells.Contains(spell) && !other.KnownSpells.Contains(spell) {
                    other.KnownSpells.AddSpell(spell)
                    break
                }
            }
        }
    }
    if !rival().CanTeach {
        test.Fatalf("the rival should know a spell you don't")
    }
    give := rival().Spells[0]
    if err := host.Execute(&TradeSpellsCommand{Wizard: banner, Spell: give}); err != nil {
        test.Fatalf("trade: %v", err)
    }
    offer := rival().TradeOffer
    if offer == nil || offer.Give != give {
        test.Fatalf("trade offer: %+v", offer)
    }
    if err := host.Execute(&AnswerTradeCommand{Accept: true}); err != nil {
        test.Fatalf("accept: %v", err)
    }
    learned := host.Model.MakePlayerView(human).You.KnownSpells
    if !slices.Contains(learned, offer.Receive) || rival().TradeOffer != nil {
        test.Errorf("didn't learn %v from the trade", offer.Receive)
    }
}

// a new game with a chosen premade wizard, race and banner (the start screen's settings)
func TestHostStartChoice(test *testing.T) {
    options := MakeStartOptions()
    if len(options.Wizards) != 14 || len(options.Races) != 14 || len(options.Banners) != 5 || len(options.Difficulties) != 6 {
        test.Fatalf("start options: %v wizards, %v races, %v banners, %v difficulties", len(options.Wizards), len(options.Races), len(options.Banners), len(options.Difficulties))
    }
    if options.Wizards[0].Name != "Merlin" || options.Wizards[0].Books["Life"] != 5 || options.Wizards[0].Retort == "" {
        test.Errorf("Merlin: %+v", options.Wizards[0])
    }
    myrran := ""
    for _, wizard := range options.Wizards {
        if wizard.Myrran {
            myrran = wizard.Name
        }
    }
    if myrran == "" {
        test.Errorf("no wizard with the Myrran retort")
    }

    decoded, err := DecodeHostSettings([]byte(`{"opponents": 1, "wizard": "Merlin", "race": "High Elf", "banner": "red", "difficulty": "easy"}`))
    if err != nil || decoded.WizardChoice != "Merlin" || decoded.Race != "High Elf" || decoded.Banner != "red" {
        test.Fatalf("decode: %+v %v", decoded, err)
    }

    dataPath := os.Getenv("CIV_GAMEDATA")
    if dataPath == "" {
        test.Skip("CIV_GAMEDATA not set")
    }
    gameData, err := gamedata.Load(dataPath)
    if err != nil {
        test.Fatalf("load: %v", err)
    }
    if !testing.Verbose() {
        log.SetOutput(io.Discard)
        defer log.SetOutput(os.Stderr)
    }

    host, err := MakeGameHost(gameData, decoded)
    if err != nil {
        test.Fatalf("setup: %v", err)
    }
    human := host.Human
    if human.Wizard.Name != "Merlin" || human.Wizard.Race != data.RaceHighElf || human.GetBanner() != data.BannerRed || human.Wizard.MagicLevel(data.LifeMagic) != 5 {
        test.Errorf("the human wizard: %+v", human.Wizard)
    }
    if capital := human.FindFortressCity(); capital == nil || capital.Race != data.RaceHighElf {
        test.Errorf("the capital should be High Elf")
    }
    for _, other := range host.Model.Players {
        if other != human && other.GetBanner() == data.BannerRed {
            test.Errorf("an opponent also has the red banner")
        }
    }

    // random race and banner; a Myrran race only for a Myrran wizard
    if _, err := MakeGameHost(gameData, HostSettings{Opponents: 1, WizardChoice: "Raven"}); err != nil {
        test.Errorf("random race and banner: %v", err)
    }
    if _, err := MakeGameHost(gameData, HostSettings{Opponents: 1, WizardChoice: "Merlin", Race: "Dwarf"}); err == nil {
        test.Errorf("Merlin can't play Dwarves")
    }
    if host, err := MakeGameHost(gameData, HostSettings{Opponents: 1, WizardChoice: myrran, Race: "Dwarf"}); err != nil || host.Human.FindFortressCity().Plane != data.PlaneMyrror {
        test.Errorf("%v with Dwarves should start on Myrror: %v", myrran, err)
    }
    for _, bad := range []HostSettings{{Opponents: 1, WizardChoice: "Nobody"}, {Opponents: 1, WizardChoice: "Merlin", Banner: "pink"}, {Opponents: 1, Race: "Orc"}} {
        if _, err := MakeGameHost(gameData, bad); err == nil {
            test.Errorf("%+v should be refused", bad)
        }
    }
}

// the AI's spells replay as named spell steps (with their effect), not anonymous health changes
func TestHostAISpellSteps(test *testing.T) {
    dataPath := os.Getenv("CIV_GAMEDATA")
    if dataPath == "" {
        test.Skip("CIV_GAMEDATA not set")
    }
    gameData, err := gamedata.Load(dataPath)
    if err != nil {
        test.Fatalf("load: %v", err)
    }
    if !testing.Verbose() {
        log.SetOutput(io.Discard)
        defer log.SetOutput(os.Stderr)
    }

    found := false
    for attempt := 0; attempt < 10 && !found; attempt++ {
        host, err := MakeGameHost(gameData, HostSettings{Opponents: 1, Scenario: "siege"})
        if err != nil {
            test.Fatalf("setup: %v", err)
        }
        // the defending wizard knows only Fire Bolt, with mana to spare
        var enemy *playerlib.Player
        for _, player := range host.Model.Players {
            if player != host.Human && player.AIBehavior != nil && !player.IsNeutral() {
                enemy = player
            }
        }
        enemy.KnownSpells = spellbook.Spells{}
        enemy.KnownSpells.AddSpell(host.Model.AllSpells.FindByName("Fire Bolt"))
        enemy.Mana = 10000
        enemy.CastingSkillPower = max(enemy.CastingSkillPower, 100000)

        capital := enemy.FindFortressCity()
        var army *playerlib.UnitStack
        for _, stack := range host.Human.Stacks {
            width := host.Model.GetMap(capital.Plane).Width()
            dx := min(abs(capital.X - stack.X()), width - abs(capital.X - stack.X()))
            if stack.Plane() == capital.Plane && max(dx, abs(capital.Y - stack.Y())) <= 1 {
                army = stack
            }
        }
        if err := host.Execute(&MoveStackCommand{From: Location{Plane: army.Plane(), X: army.X(), Y: army.Y()}, To: Location{Plane: capital.Plane, X: capital.X, Y: capital.Y}}); err != nil || host.WaitingFor() != WaitingBattle {
            test.Fatalf("attack: %v, waiting for %q", err, host.WaitingFor())
        }
        enemyArmy := host.Session.Battle.Battle.Combat.GetArmyForPlayer(enemy)
        enemyArmy.ManaPool = 1000

        check := func() {
            for _, step := range host.Session.Battle.View().Steps {
                if step.Kind == "effect" && len(step.Changes) > 0 {
                    // health changed by something else (walls of fire): fine
                    continue
                }
                if step.Kind == "spell" {
                    if step.Spell != "Fire Bolt" || step.Effect != "spell-effect/fire-bolt" || step.Target == nil {
                        test.Errorf("the AI's spell step: %+v", step)
                    }
                    found = true
                }
            }
        }
        check()
        for turn := 0; turn < 40 && !found && host.WaitingFor() == WaitingBattle; turn++ {
            if err := host.Execute(&BattleDoneCommand{}); err != nil {
                break
            }
            check()
        }
    }
    if !found {
        test.Errorf("the AI never cast Fire Bolt in 10 sieges")
    }
}

// the orders scenario: settle an outpost, start a road, travel through a tower and patrol, all
// through commands, as the stack panel sends them (5 games, the maps are random)
func TestHostOrdersScenario(test *testing.T) {
    dataPath := os.Getenv("CIV_GAMEDATA")
    if dataPath == "" {
        test.Skip("CIV_GAMEDATA not set")
    }
    gameData, err := gamedata.Load(dataPath)
    if err != nil {
        test.Fatalf("load: %v", err)
    }
    if !testing.Verbose() {
        log.SetOutput(io.Discard)
        defer log.SetOutput(os.Stderr)
    }

    for range 5 {
        host, err := MakeGameHost(gameData, HostSettings{Opponents: 1, Scenario: "orders"})
        if err != nil {
            test.Fatalf("setup: %v", err)
        }
        view := host.Model.MakePlayerView(host.Human)
        find := func(view PlayerView, check func(StackView) bool) *StackView {
            for index := range view.Stacks {
                if check(view.Stacks[index]) {
                    return &view.Stacks[index]
                }
            }
            return nil
        }
        settlers := find(view, func(stack StackView) bool { return stack.Build == "settle" })
        engineers := find(view, func(stack StackView) bool { return stack.Road })
        traveller := find(view, func(stack StackView) bool { return stack.PlanarTravel })
        if settlers == nil || engineers == nil || traveller == nil {
            test.Fatalf("the view should offer settle, road and planar travel: %+v", view.Stacks)
        }
        host.TakeEvents()

        cities := len(host.Human.Cities)
        if err := host.Execute(&BuildActionCommand{Stack: settlers.Location}); err != nil {
            test.Fatalf("settle: %v", err)
        }
        if len(host.Human.Cities) != cities + 1 {
            test.Errorf("settling should found an outpost")
        }
        outpost := false
        for _, event := range host.TakeEvents() {
            if strings.Contains(string(event), `"new-outpost"`) {
                outpost = true
            }
        }
        if !outpost {
            test.Errorf("settling should send a new-outpost event")
        }

        // a road two tiles away from the engineers, on explored land
        var road []Location
        for _, offset := range [][2]int{{2, 0}, {-2, 0}, {0, 2}, {0, -2}, {2, 2}, {-2, -2}, {2, -2}, {-2, 2}} {
            to := Location{Plane: engineers.Location.Plane, X: host.Model.GetMap(engineers.Location.Plane).WrapX(engineers.Location.X + offset[0]), Y: engineers.Location.Y + offset[1]}
            if path, err := host.Model.RoadPathFor(host.Human, engineers.Location, to); err == nil {
                road = path
                break
            }
        }
        if road == nil {
            test.Fatalf("no road path from the engineers")
        }
        if err := host.Execute(&BuildRoadCommand{Stack: engineers.Location, To: road[len(road) - 1]}); err != nil {
            test.Fatalf("build-road: %v", err)
        }

        start := traveller.Location
        if err := host.Execute(&PlanarTravelCommand{Stack: start}); err != nil {
            test.Fatalf("planar travel: %v", err)
        }
        arrived := host.Human.FindStack(start.X, start.Y, start.Plane.Opposite())
        if arrived == nil || host.Human.FindStack(start.X, start.Y, start.Plane) != nil {
            test.Fatalf("the swordsmen should be on the other plane")
        }

        there := Location{Plane: start.Plane.Opposite(), X: start.X, Y: start.Y}
        if err := host.Execute(&PatrolCommand{Stack: there}); err != nil {
            test.Fatalf("patrol: %v", err)
        }
        view = host.Model.MakePlayerView(host.Human)
        patrol := find(view, func(stack StackView) bool { return stack.Location == there })
        if patrol == nil || patrol.Units[0].Busy != "patrolling" {
            test.Errorf("the swordsmen should patrol: %+v", patrol)
        }
        building := find(view, func(stack StackView) bool {
            return slices.ContainsFunc(stack.Units, func(unit UnitView) bool { return unit.Busy == "building road" })
        })
        if building == nil {
            test.Errorf("the engineers should be building the road")
        }
    }
}

// a stack sent further than its moves reach walks on along the rest of its path at the start of
// the next turn, without a new order
func TestHostContinuesMoves(test *testing.T) {
    dataPath := os.Getenv("CIV_GAMEDATA")
    if dataPath == "" {
        test.Skip("CIV_GAMEDATA not set")
    }
    gameData, err := gamedata.Load(dataPath)
    if err != nil {
        test.Fatalf("load: %v", err)
    }
    if !testing.Verbose() {
        log.SetOutput(io.Discard)
        defer log.SetOutput(os.Stderr)
    }

    host, err := MakeGameHost(gameData, HostSettings{Opponents: 1})
    if err != nil {
        test.Fatalf("setup: %v", err)
    }
    host.TakeEvents()
    human := host.Human
    stack := human.Stacks[0]
    unit := stack.Units()[0]
    moves := int(unit.GetMovesLeft(true).ToFloat())

    // a destination two or more turns away, on explored land
    mapUse := host.Model.GetMap(stack.Plane())
    fog := human.GetFog(stack.Plane())
    target := Location{Plane: stack.Plane(), X: -1}
    search:
    for distance := moves + 2; distance <= moves + 6; distance++ {
        for dy := -distance; dy <= distance; dy++ {
            for dx := -distance; dx <= distance; dx++ {
                x, y := mapUse.WrapX(stack.X() + dx), stack.Y() + dy
                if y < 0 || y >= mapUse.Height() {
                    continue
                }
                path, ok := host.Model.FindPath(stack.X(), stack.Y(), x, y, human, stack, fog)
                if ok && len(path) > moves + 1 {
                    target.X, target.Y = x, y
                    break search
                }
            }
        }
    }
    if target.X < 0 {
        test.Skip("no destination two turns away on this map")
    }

    from := Location{Plane: stack.Plane(), X: stack.X(), Y: stack.Y()}
    if err := host.Execute(&MoveStackCommand{From: from, To: target}); err != nil {
        test.Fatalf("move: %v", err)
    }
    walking := human.FindStackByUnit(unit)
    left := len(walking.CurrentPath)
    if left == 0 {
        test.Fatalf("the stack should have a path left after its moves")
    }
    afterFirst := Location{Plane: walking.Plane(), X: walking.X(), Y: walking.Y()}

    if err := host.Execute(&EndTurnCommand{}); err != nil {
        test.Fatalf("end turn: %v", err)
    }
    for host.WaitingFor() == WaitingBattle {
        if err := host.Execute(&BattleAutoCommand{}); err != nil {
            test.Fatalf("auto battle: %v", err)
        }
    }
    walking = human.FindStackByUnit(unit)
    if walking == nil {
        test.Skip("the unit died on the way")
    }
    now := Location{Plane: walking.Plane(), X: walking.X(), Y: walking.Y()}
    if now == afterFirst || len(walking.CurrentPath) >= left {
        test.Errorf("the stack should walk on at the start of the turn: at %v with %v steps left, now %v with %v", afterFirst, left, now, len(walking.CurrentPath))
    }
}
