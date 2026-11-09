package game

import (
    "encoding/json"
    "errors"
    "fmt"
    "slices"
    "strings"

    "github.com/bfalese/civ-of-magic/core/assets"
    "github.com/bfalese/civ-of-magic/core/combat"
    "github.com/bfalese/civ-of-magic/core/data"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/core/units"
    "github.com/bfalese/civ-of-magic/lib/fraction"
)

// A tactical battle played through commands (see CommandSession). The AI moves its own units
// (and the player's when auto is on); when one of the player's units is up, the session waits
// for a battle command. Battle commands act on the selected unit, like the buttons and clicks of
// the Ebitengine battle screen, including casting combat spells (battle-cast).
type BattleSession struct {
    Battle *Battle
    Player *playerlib.Player
    // records what each command (and the AI moves after it) did, for the view's steps
    actions *battleRecorder
    spells *combat.ProxySpellSystem
    // the outcome once known: CombatModel.FinalState has side effects when the battle ends (the
    // flee rolls, FinishCombat), so it must only be asked until it reports an end
    finalState combat.CombatState
    // a spell waiting for its target (a battle-cast without one); battle-target answers it
    casting *pendingCast
}

// a cast whose spell asked for a target: the model's question, answered by battle-target
type pendingCast struct {
    question combat.CombatEvent
    spell spellbook.Spell
    caster *uint64
    // set once the model has taken the cost (the target was accepted, or the spell fizzled)
    paid *bool
}

func makeBattleSession(battle *Battle, player *playerlib.Player) *BattleSession {
    return &BattleSession{
        Battle: battle,
        Player: player,
        actions: makeBattleRecorder(battle.Combat),
        spells: &combat.ProxySpellSystem{Model: battle.Combat},
        finalState: combat.CombatStateRunning,
    }
}

// the battle's state; asks the combat model only while the battle is running
func (session *BattleSession) State() combat.CombatState {
    if session.finalState == combat.CombatStateRunning {
        session.finalState = session.Battle.Combat.FinalState()
    }
    return session.finalState
}

func (session *BattleSession) army() *combat.Army {
    return session.Battle.Combat.GetArmyForPlayer(session.Player)
}

// apply the combat events' effects (next unit, lightning bolts, walls, AI target choices) and
// resolve projectiles. Events asking the player for a spell target are returned for the caller.
func (session *BattleSession) drainEvents() []combat.CombatEvent {
    model := session.Battle.Combat
    var questions []combat.CombatEvent
    for {
        select {
            case event := <-model.Events:
                if !model.ApplyEvent(session.spells, event) {
                    questions = append(questions, event)
                }
            default:
                if len(model.Projectiles) == 0 {
                    return questions
                }
                session.actions.DoProjectiles()
        }
    }
}

// let the AI act until one of the player's units is up or the battle is over
func (session *BattleSession) advance() {
    model := session.Battle.Combat
    for steps := 0; steps < 100000; steps++ {
        if session.State() != combat.CombatStateRunning {
            return
        }

        if model.SelectedUnit == nil {
            model.NextUnit()
            continue
        }

        if !model.IsAIControlled(model.SelectedUnit) {
            return
        }

        // let magic vortexes move around randomly, the ai won't move them on its own (as combat.Run)
        for _, vortex := range model.MagicVortexes {
            if !vortex.Moved {
                model.MoveMagicVortex(vortex, session.actions, &combat.FakeDamageIndicators{}, false, 0, 0)
                vortex.Moved = true
            }
        }

        model.Update(session.spells, session.actions, false, 0, 0)
        session.drainEvents()
    }
}

// the AI finishes the battle for the player
func (session *BattleSession) auto() {
    session.army().Auto = true
}

// apply a battle command to the player's selected unit
func (session *BattleSession) apply(command Command) error {
    model := session.Battle.Combat
    if model.SelectedUnit == nil || !model.InControl(session.Player) {
        return errors.New("none of your units is up")
    }

    // the view's steps show what this command and the AI moves after it did
    session.actions.reset()

    // a spell waiting for its target is answered by battle-target, dropped by anything else
    switch command.(type) {
        case *BattleTargetCommand:
            if session.casting == nil {
                return errors.New("no spell is waiting for a target")
            }
            target := command.(*BattleTargetCommand).BattleTarget
            return session.finishCast(session.casting, &target)
        case *BattleCancelCommand:
            if session.casting == nil {
                return errors.New("no spell is waiting for a target")
            }
            session.casting = nil
            return nil
    }
    session.casting = nil

    switch command.(type) {
        case *BattleActCommand:
            act := command.(*BattleActCommand)
            unit := model.SelectedUnit
            before := unit.MovesLeft
            beforeX, beforeY := unit.X, unit.Y
            model.Update(session.spells, session.actions, true, act.X, act.Y)
            session.drainEvents()
            if model.SelectedUnit == unit && unit.X == beforeX && unit.Y == beforeY && unit.MovesLeft.Equals(before) {
                return errors.New("the unit can't move or attack there")
            }
        case *BattleDoneCommand:
            model.DoneTurn()
        case *BattleWaitCommand:
            model.NextUnit()
        case *BattleFleeCommand:
            session.army().Fled = true
        case *BattleAutoCommand:
            session.auto()
        case *BattleCastCommand:
            return session.cast(command.(*BattleCastCommand))
        default:
            return errors.New("answer with a battle command")
    }
    return nil
}

// cast a combat spell: the wizard's (once per turn, limited by the army's mana pool and the
// wizard's mana over the range) or the selected unit's (its spells within its casting skill, or
// item charges). The cost is paid only once the target is accepted, as in the battle screen.
func (session *BattleSession) cast(command *BattleCastCommand) error {
    model := session.Battle.Combat
    player := session.Player
    army := session.army()
    defendingCity := model.Zone.City != nil && army == model.DefendingArmy

    var spell spellbook.Spell
    var unitCaster *combat.ArmyUnit
    var castedCallback func(bool)

    if command.Caster == nil {
        if army.Casted {
            return errors.New("you already cast a spell this turn")
        }
        combatSpells := player.KnownSpells.CombatSpells(defendingCity)
        spell = combatSpells.FindByName(command.Spell)
        if !spell.Valid() {
            return fmt.Errorf("%v can't be cast here", command.Spell)
        }
        cost := player.ComputeEffectiveSpellCost(spell, false)
        if cost > min(army.ManaPool, int(float64(player.GetMana()) / army.Range.ToFloat())) {
            return errors.New("not enough mana or casting skill")
        }
        castedCallback = func(success bool) {
            spellCost := player.ComputeEffectiveSpellCost(spell, false)
            army.Casted = true
            army.ManaPool -= spellCost
            player.UseMana(int(float64(spellCost) * army.Range.ToFloat()))
            if success {
                model.AddLogEvent(fmt.Sprintf("%v casts %v", player.Wizard.Name, spell.Name))
            }
        }
    } else {
        unitCaster = model.SelectedUnit
        if unitCaster.Id != *command.Caster {
            return errors.New("only the unit that is up can cast")
        }
        if !unitCaster.CanCast() {
            return errors.New("the unit can't cast now")
        }
        unitSpells := unitCaster.Spells.CombatSpells(defendingCity)
        spell = unitSpells.FindByName(command.Spell)
        if spell.Valid() {
            if float32(spell.Cost(false)) > unitCaster.CastingSkill {
                return errors.New("not enough casting skill")
            }
        } else {
            for charged, charges := range unitCaster.SpellCharges {
                if charged.Name == command.Spell && charges > 0 {
                    spell = charged
                }
            }
            if !spell.Valid() {
                return fmt.Errorf("the unit can't cast %v", command.Spell)
            }
        }
        caster := unitCaster
        castedCallback = func(success bool) {
            if success {
                model.AddLogEvent(fmt.Sprintf("%v casts %v", caster.Unit.GetName(), spell.Name))
            }
            model.DoUnitCast(caster, spell)
        }
    }

    paid := false
    session.actions.flush()
    model.InvokeSpell(session.spells, army, unitCaster, spell, func(success bool) {
        paid = true
        castedCallback(success)
    })

    // the model asks for a target: answer with the command's, or wait for a battle-target
    questions := session.drainEvents()
    if len(questions) > 0 {
        pending := &pendingCast{question: questions[0], spell: spell, caster: command.Caster, paid: &paid}
        if command.Target == nil {
            session.casting = pending
            return nil
        }
        return session.finishCast(pending, command.Target)
    }
    return session.castDone(paid, spell, command.Caster)
}

// answer the pending cast's question; a target the spell refuses leaves the question open
func (session *BattleSession) finishCast(pending *pendingCast, target *BattleTarget) error {
    if err := session.answerTarget(pending.question, target); err != nil {
        return err
    }
    session.casting = nil
    session.drainEvents()
    return session.castDone(*pending.paid, pending.spell, pending.caster)
}

func (session *BattleSession) castDone(paid bool, spell spellbook.Spell, caster *uint64) error {
    model := session.Battle.Combat
    if !paid {
        return errors.New("the spell had no effect")
    }
    session.actions.add(BattleStepView{Kind: "spell", Unit: caster, Spell: spell.Name})

    // a unit that cast is done (as Update does after an action)
    if model.SelectedUnit != nil && model.InControl(session.Player) && (model.SelectedUnit.GetHealth() <= 0 || model.SelectedUnit.MovesLeft.LessThanEqual(fraction.Zero())) {
        model.DoneTurn()
    }
    return nil
}

func (session *BattleSession) answerTarget(question combat.CombatEvent, target *BattleTarget) error {
    model := session.Battle.Combat
    targetUnit := func() *combat.ArmyUnit {
        if target == nil || target.Unit == nil {
            return nil
        }
        return model.GetUnitById(*target.Unit)
    }

    switch question.(type) {
        case *combat.CombatEventSelectUnit:
            ask := question.(*combat.CombatEventSelectUnit)
            unit := targetUnit()
            if unit == nil {
                return fmt.Errorf("%v needs a target unit", ask.Spell.Name)
            }
            if ask.SelectTeam != combat.TeamEither && unit.Team != ask.SelectTeam {
                return fmt.Errorf("%v can't target that side", ask.Spell.Name)
            }
            if ask.CanTarget != nil && !ask.CanTarget(unit) {
                return fmt.Errorf("%v can't target %v", ask.Spell.Name, unit.Unit.GetName())
            }
            ask.SelectTarget(unit)
        case *combat.CombatEventSelectTile:
            ask := question.(*combat.CombatEventSelectTile)
            if target == nil || target.X == nil || target.Y == nil {
                return fmt.Errorf("%v needs a target tile", ask.Spell.Name)
            }
            if ask.CanTarget != nil && !ask.CanTarget(*target.X, *target.Y) {
                return fmt.Errorf("%v can't target that tile", ask.Spell.Name)
            }
            ask.SelectTile(*target.X, *target.Y)
        case *combat.CombatSelectTargets:
            // the choices can be killed units, which aren't in the armies any more
            ask := question.(*combat.CombatSelectTargets)
            var unit *combat.ArmyUnit
            if target != nil && target.Unit != nil {
                for _, choice := range ask.Targets {
                    if choice.Id == *target.Unit {
                        unit = choice
                    }
                }
            }
            if unit == nil {
                return fmt.Errorf("choose one of the units offered (%v)", ask.Title)
            }
            ask.Select(unit)
    }
    return nil
}

// battle commands; they answer a CommandSession waiting for a battle

// a target in a battle: a unit by id, or a tile
type BattleTarget struct {
    Unit *uint64 `json:"unit,omitempty"`
    X *int `json:"x,omitempty"`
    Y *int `json:"y,omitempty"`
}

// cast a combat spell; Caster is the id of the selected unit for a unit's spell, nil for the wizard
type BattleCastCommand struct {
    Spell string `json:"spell"`
    Caster *uint64 `json:"caster,omitempty"`
    Target *BattleTarget `json:"target,omitempty"`
}

func (command *BattleCastCommand) CommandType() string { return "battle-cast" }
func (command *BattleCastCommand) Execute(context *CommandContext) error { return errors.New("not in a battle") }

// the target for the spell a battle-cast without one left waiting
type BattleTargetCommand struct {
    BattleTarget
}

func (command *BattleTargetCommand) CommandType() string { return "battle-target" }
func (command *BattleTargetCommand) Execute(context *CommandContext) error { return errors.New("not in a battle") }

// drop the spell waiting for a target; nothing is paid
type BattleCancelCommand struct {
}

func (command *BattleCancelCommand) CommandType() string { return "battle-cancel" }
func (command *BattleCancelCommand) Execute(context *CommandContext) error { return errors.New("not in a battle") }

// the selected unit moves to, or attacks, the tile (as a click on the battle field)
type BattleActCommand struct {
    X int `json:"x"`
    Y int `json:"y"`
}

func (command *BattleActCommand) CommandType() string { return "battle-act" }
func (command *BattleActCommand) Execute(context *CommandContext) error { return errors.New("not in a battle") }

// the selected unit is done for this turn
type BattleDoneCommand struct {
}

func (command *BattleDoneCommand) CommandType() string { return "battle-done" }
func (command *BattleDoneCommand) Execute(context *CommandContext) error { return errors.New("not in a battle") }

// the selected unit waits; the next unit is up
type BattleWaitCommand struct {
}

func (command *BattleWaitCommand) CommandType() string { return "battle-wait" }
func (command *BattleWaitCommand) Execute(context *CommandContext) error { return errors.New("not in a battle") }

// the player's army flees
type BattleFleeCommand struct {
}

func (command *BattleFleeCommand) CommandType() string { return "battle-flee" }
func (command *BattleFleeCommand) Execute(context *CommandContext) error { return errors.New("not in a battle") }

// the AI fights the rest of the battle for the player
type BattleAutoCommand struct {
}

func (command *BattleAutoCommand) CommandType() string { return "battle-auto" }
func (command *BattleAutoCommand) Execute(context *CommandContext) error { return errors.New("not in a battle") }

// answer to choose-encounter: enter the lair / node / tower or not
type ConfirmEncounterCommand struct {
    Enter bool `json:"enter"`
}

func (command *ConfirmEncounterCommand) CommandType() string { return "confirm-encounter" }
func (command *ConfirmEncounterCommand) Execute(context *CommandContext) error { return errors.New("nothing is waiting for an answer") }

// answer to choose-raze: raze the captured city or keep it
type RazeCityCommand struct {
    Raze bool `json:"raze"`
}

func (command *RazeCityCommand) CommandType() string { return "raze-city" }
func (command *RazeCityCommand) Execute(context *CommandContext) error { return errors.New("nothing is waiting for an answer") }

// the battle as the player sees it
type BattleUnitView struct {
    Id uint64 `json:"id"`
    Name string `json:"name"`
    Owner *PlayerRef `json:"owner"`
    // true for the player's units
    Yours bool `json:"yours"`
    X int `json:"x"`
    Y int `json:"y"`
    Health int `json:"health"`
    MaxHealth int `json:"max-health"`
    MovesLeft float64 `json:"moves-left"`
    RangedAttacks int `json:"ranged-attacks,omitempty"`
    // the combat sprite for the way it faces (core/assets BattleUnit), in the owner's banner colour
    Asset string `json:"asset"`
    Banner string `json:"banner"`
    Facing string `json:"facing"`
    // where its living figures stand, in pixels from the tile's centre (units.CombatPoints)
    Figures [][2]int `json:"figures"`
    // flying units flap in place (the sprite's frames loop)
    Flying bool `json:"flying,omitempty"`
    // for unit details
    Melee int `json:"melee"`
    Ranged int `json:"ranged"`
    // the ranged attack's kind (units.Damage), if any
    RangedType string `json:"ranged-type,omitempty"`
    Defense int `json:"defense"`
    Resistance int `json:"resistance"`
    // per figure; the figures are FigureCount, the living ones len(Figures)
    HitPoints int `json:"hit-points"`
    FigureCount int `json:"figure-count"`
    Level string `json:"level,omitempty"`
    Abilities []string `json:"abilities"`
    Enchantments []string `json:"enchantments"`
}

// a line of the battle log
type BattleLogView struct {
    Turn int `json:"turn"`
    Text string `json:"text"`
}

// a battlefield tile: its ground, and a tree/rock/building on it
type BattleTileView struct {
    Asset string `json:"asset"`
    Top string `json:"top,omitempty"`
    // how the top sits on the tile: "middle" (centred) or "bottom" (standing on the tile's centre)
    TopAlign string `json:"top-align,omitempty"`
    Mud bool `json:"mud,omitempty"`
}

// an enemy the selected unit can attack now
type BattleTargetView struct {
    Id uint64 `json:"id"`
    // "ranged" or "melee", the attack a battle-act on its tile makes
    Kind string `json:"kind"`
}

// a combat spell the wizard or the selected unit can cast
type BattleSpellView struct {
    Name string `json:"name"`
    // the realm: Life, Death, Chaos, Nature, Sorcery or Arcane
    Magic string `json:"magic"`
    Cost int `json:"cost"`
    // whether a battle-cast of it would be accepted now (cost within what's available, not cast yet)
    Castable bool `json:"castable"`
    // for a spell from an item: the charges left
    Charges int `json:"charges,omitempty"`
}

// the wizard's combat magic: one spell per turn, paid from the army's mana pool (the casting
// skill) and from the wizard's mana times the range from the fortress
type BattleMagicView struct {
    ManaPool int `json:"mana-pool"`
    Mana int `json:"mana"`
    Range float64 `json:"range"`
    // the most a spell can cost now: the lower of the mana pool and mana / range
    Available int `json:"available"`
    // already cast a spell this turn
    Casted bool `json:"casted"`
    Spells []BattleSpellView `json:"spells"`
}

// the selected unit's own spells and item charges (battle-cast with caster = its id)
type BattleUnitMagicView struct {
    Caster uint64 `json:"caster"`
    CastingSkill int `json:"casting-skill"`
    Spells []BattleSpellView `json:"spells"`
}

// a unit offered by a spell with a choice (Raise Dead, Animate Dead), possibly a killed one
type BattleChoiceView struct {
    Id uint64 `json:"id"`
    Name string `json:"name"`
    Asset string `json:"asset"`
    Banner string `json:"banner"`
}

// a spell waiting for its target: answer with battle-target, or battle-cancel
type BattleCastingView struct {
    Spell string `json:"spell"`
    Caster *uint64 `json:"caster,omitempty"`
    // "unit" (one of units), "tile" (one of tiles) or "choice" (one of choices)
    Kind string `json:"kind"`
    Title string `json:"title,omitempty"`
    Units []uint64 `json:"units,omitempty"`
    Tiles [][2]int `json:"tiles,omitempty"`
    Choices []BattleChoiceView `json:"choices,omitempty"`
}

type BattleView struct {
    Turn int `json:"turn"`
    // the player's unit that is up, if any
    Selected *uint64 `json:"selected,omitempty"`
    YouAttack bool `json:"you-attack"`
    Width int `json:"width"`
    Height int `json:"height"`
    Units []BattleUnitView `json:"units"`
    State string `json:"state"`
    // tiles[y][x]
    Tiles [][]BattleTileView `json:"tiles"`
    // a battle in a town: the road through it, and clouds under a flying fortress
    Road bool `json:"road,omitempty"`
    Clouds bool `json:"clouds,omitempty"`
    // for the selected unit: the tiles battle-act moves it to, and the enemies it can attack
    Moves [][2]int `json:"moves"`
    Targets []BattleTargetView `json:"targets"`
    Magic *BattleMagicView `json:"magic,omitempty"`
    // only while the selected unit can cast
    UnitMagic *BattleUnitMagicView `json:"unit-magic,omitempty"`
    Casting *BattleCastingView `json:"casting,omitempty"`
    // what the last battle command, and the AI moves after it, did; replay these from the
    // previous view to animate the change
    Steps []BattleStepView `json:"steps"`
    // the latest lines of the battle log, oldest first
    Log []BattleLogView `json:"log"`
}

func (session *BattleSession) View() BattleView {
    model := session.Battle.Combat
    view := BattleView{
        Turn: model.CurrentTurn,
        YouAttack: model.AttackingArmy.Player == session.Player,
        Height: len(model.Tiles),
        Units: []BattleUnitView{},
        State: battleStateName(session.State()),
    }
    if len(model.Tiles) > 0 {
        view.Width = len(model.Tiles[0])
    }
    if model.SelectedUnit != nil && model.InControl(session.Player) {
        id := model.SelectedUnit.Id
        view.Selected = &id
    }

    view.Tiles = make([][]BattleTileView, len(model.Tiles))
    for y, row := range model.Tiles {
        view.Tiles[y] = make([]BattleTileView, len(row))
        for x, tile := range row {
            tileView := BattleTileView{Asset: assets.BattleTile(tile.Lbx, tile.Index), Mud: tile.Mud}
            if tile.ExtraObject.Special == combat.TileTopSpecialNone && tile.ExtraObject.Index != -1 {
                tileView.Top = assets.BattleTop(tile.ExtraObject.Lbx, tile.ExtraObject.Index)
                tileView.TopAlign = "middle"
                if tile.ExtraObject.Alignment == combat.TileAlignBottom {
                    tileView.TopAlign = "bottom"
                }
            }
            view.Tiles[y][x] = tileView
        }
    }
    if model.Zone.City != nil {
        view.Road = true
        view.Clouds = model.Zone.City.HasEnchantment(data.CityEnchantmentFlyingFortress)
    }

    view.Moves = [][2]int{}
    view.Targets = []BattleTargetView{}
    if view.Selected != nil {
        view.Moves, view.Targets = session.reach(model.SelectedUnit)
    }
    view.Magic = session.magicView(view.Selected != nil)
    if view.Selected != nil {
        view.UnitMagic = session.unitMagicView(model.SelectedUnit)
    }
    if session.casting != nil {
        view.Casting = session.castingView(session.casting)
    }
    view.Steps = session.actions.current()
    view.Log = []BattleLogView{}
    for _, entry := range model.Log[max(0, len(model.Log) - battleLogLines):] {
        view.Log = append(view.Log, BattleLogView{Turn: entry.Turn, Text: entry.Text})
    }

    for _, army := range []*combat.Army{model.AttackingArmy, model.DefendingArmy} {
        var owner *PlayerRef
        if player, ok := army.Player.(*playerlib.Player); ok {
            owner = MakePlayerRef(player)
        }
        for _, unit := range army.GetUnits() {
            if unit.GetHealth() <= 0 {
                continue
            }
            unitView := BattleUnitView{
                Id: unit.Id,
                Name: unit.Unit.GetName(),
                Owner: owner,
                Yours: army.Player == session.Player,
                X: unit.X,
                Y: unit.Y,
                Health: unit.GetHealth(),
                MaxHealth: unit.GetMaxHealth(),
                MovesLeft: unit.MovesLeft.ToFloat(),
                RangedAttacks: unit.GetRangedAttacks(),
                Asset: assets.BattleUnit(unit.Unit.GetRawUnit().Race, unit.Unit.GetRawUnit().Name, unit.Facing),
                Banner: unit.Unit.GetBanner().String(),
                Facing: assets.FacingName(unit.Facing),
                Figures: figurePoints(unit.VisibleFigures()),
                Flying: unit.Unit.IsFlying(),
                Melee: unit.GetMeleeAttackPower(),
                Ranged: unit.GetRangedAttackPower(),
                Defense: unit.GetDefense(),
                Resistance: unit.GetResistance(),
                HitPoints: unit.GetHitPoints(),
                FigureCount: unit.Figures(),
                Abilities: []string{},
                Enchantments: []string{},
            }
            if unitView.Ranged > 0 {
                unitView.RangedType = unit.GetRangedAttackDamageType().String()
            }
            if !unit.Unit.IsHero() && unit.Unit.GetRace() != data.RaceFantastic {
                unitView.Level = unit.GetExperienceLevel().Name()
            }
            for _, ability := range unit.GetAbilities() {
                unitView.Abilities = append(unitView.Abilities, ability.Name())
            }
            for _, enchantment := range unit.GetEnchantments() {
                unitView.Enchantments = append(unitView.Enchantments, enchantment.Name())
            }
            view.Units = append(view.Units, unitView)
        }
    }
    return view
}

const battleLogLines = 30

func figurePoints(count int) [][2]int {
    out := [][2]int{}
    for _, point := range units.CombatPoints(count) {
        out = append(out, [2]int{point.X, point.Y})
    }
    return out
}

// what a battle-act can do for the unit: the empty tiles it can walk (or teleport) to, and the
// enemies it can shoot or strike; the same checks CombatModel.Update makes for a click
func (session *BattleSession) reach(unit *combat.ArmyUnit) ([][2]int, []BattleTargetView) {
    model := session.Battle.Combat
    moves := [][2]int{}
    targets := []BattleTargetView{}
    if unit.MovesLeft.LessThanEqual(fraction.Zero()) {
        return moves, targets
    }

    for y, row := range model.Tiles {
        for x := range row {
            if model.TileIsEmpty(x, y) && model.CanMoveTo(unit, x, y, false) {
                moves = append(moves, [2]int{x, y})
            }
        }
    }

    for _, enemy := range model.GetOppositeArmyForPlayer(session.Player).GetUnits() {
        if enemy.GetHealth() <= 0 {
            continue
        }
        if model.WithinArrowRange(unit, enemy) && model.CanRangeAttack(unit, enemy) {
            targets = append(targets, BattleTargetView{Id: enemy.Id, Kind: "ranged"})
        } else if model.WithinMeleeRange(unit, enemy) && model.CanMeleeAttack(unit, enemy, true) {
            targets = append(targets, BattleTargetView{Id: enemy.Id, Kind: "melee"})
        }
    }
    return moves, targets
}

func (session *BattleSession) defendingCity() bool {
    model := session.Battle.Combat
    return model.Zone.City != nil && session.army() == model.DefendingArmy
}

// the wizard's combat spells; castable only while one of the player's units is up
func (session *BattleSession) magicView(inControl bool) *BattleMagicView {
    player := session.Player
    army := session.army()
    if army == nil {
        return nil
    }
    available := army.ManaPool
    if army.Range.ToFloat() > 0 {
        available = min(available, int(float64(player.GetMana()) / army.Range.ToFloat()))
    }
    view := &BattleMagicView{
        ManaPool: army.ManaPool,
        Mana: player.GetMana(),
        Range: army.Range.ToFloat(),
        Available: available,
        Casted: army.Casted,
        Spells: []BattleSpellView{},
    }
    for _, spell := range player.KnownSpells.CombatSpells(session.defendingCity()).Spells {
        cost := player.ComputeEffectiveSpellCost(spell, false)
        view.Spells = append(view.Spells, BattleSpellView{
            Name: spell.Name,
            Magic: spell.Magic.String(),
            Cost: cost,
            Castable: inControl && !army.Casted && cost <= available,
        })
    }
    return view
}

func (session *BattleSession) unitMagicView(unit *combat.ArmyUnit) *BattleUnitMagicView {
    if unit.CastingSkill <= 0 && len(unit.SpellCharges) == 0 {
        return nil
    }
    canCast := unit.CanCast()
    view := &BattleUnitMagicView{
        Caster: unit.Id,
        CastingSkill: int(unit.CastingSkill),
        Spells: []BattleSpellView{},
    }
    for _, spell := range unit.Spells.CombatSpells(session.defendingCity()).Spells {
        cost := spell.Cost(false)
        view.Spells = append(view.Spells, BattleSpellView{
            Name: spell.Name,
            Magic: spell.Magic.String(),
            Cost: cost,
            Castable: canCast && float32(cost) <= unit.CastingSkill,
        })
    }
    var charged []BattleSpellView
    for spell, charges := range unit.SpellCharges {
        charged = append(charged, BattleSpellView{
            Name: spell.Name,
            Magic: spell.Magic.String(),
            Charges: charges,
            Castable: canCast && charges > 0,
        })
    }
    slices.SortFunc(charged, func(a BattleSpellView, b BattleSpellView) int { return strings.Compare(a.Name, b.Name) })
    view.Spells = append(view.Spells, charged...)
    if len(view.Spells) == 0 {
        return nil
    }
    return view
}

// the targets the waiting spell accepts, by the model's own checks
func (session *BattleSession) castingView(pending *pendingCast) *BattleCastingView {
    model := session.Battle.Combat
    view := &BattleCastingView{Spell: pending.spell.Name, Caster: pending.caster}
    switch pending.question.(type) {
        case *combat.CombatEventSelectUnit:
            ask := pending.question.(*combat.CombatEventSelectUnit)
            view.Kind = "unit"
            view.Units = []uint64{}
            for _, army := range []*combat.Army{model.AttackingArmy, model.DefendingArmy} {
                for _, unit := range army.GetUnits() {
                    if unit.GetHealth() <= 0 {
                        continue
                    }
                    if ask.SelectTeam != combat.TeamEither && unit.Team != ask.SelectTeam {
                        continue
                    }
                    if ask.CanTarget != nil && !ask.CanTarget(unit) {
                        continue
                    }
                    view.Units = append(view.Units, unit.Id)
                }
            }
        case *combat.CombatEventSelectTile:
            ask := pending.question.(*combat.CombatEventSelectTile)
            view.Kind = "tile"
            view.Tiles = [][2]int{}
            for y, row := range model.Tiles {
                for x := range row {
                    if ask.CanTarget == nil || ask.CanTarget(x, y) {
                        view.Tiles = append(view.Tiles, [2]int{x, y})
                    }
                }
            }
        case *combat.CombatSelectTargets:
            ask := pending.question.(*combat.CombatSelectTargets)
            view.Kind = "choice"
            view.Title = ask.Title
            view.Choices = []BattleChoiceView{}
            for _, unit := range ask.Targets {
                view.Choices = append(view.Choices, BattleChoiceView{
                    Id: unit.Id,
                    Name: unit.Unit.GetName(),
                    Asset: assets.BattleUnit(unit.Unit.GetRawUnit().Race, unit.Unit.GetRawUnit().Name, units.FacingDown),
                    Banner: unit.Unit.GetBanner().String(),
                })
            }
    }
    return view
}

func (session *BattleSession) EncodeView() ([]byte, error) {
    return json.Marshal(session.View())
}
