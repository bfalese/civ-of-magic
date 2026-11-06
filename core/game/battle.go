package game

import (
    "encoding/json"
    "errors"
    "fmt"
    "slices"

    "github.com/bfalese/civ-of-magic/core/combat"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/lib/fraction"
)

// A tactical battle played through commands (see CommandSession). The AI moves its own units
// (and the player's when auto is on); when one of the player's units is up, the session waits
// for a battle command. Battle commands act on the selected unit, like the buttons and clicks of
// the Ebitengine battle screen, including casting combat spells (battle-cast).
type BattleSession struct {
    Battle *Battle
    Player *playerlib.Player
    actions *combat.ProxyActions
    spells *combat.ProxySpellSystem
    // the outcome once known: CombatModel.FinalState has side effects when the battle ends (the
    // flee rolls, FinishCombat), so it must only be asked until it reports an end
    finalState combat.CombatState
}

func makeBattleSession(battle *Battle, player *playerlib.Player) *BattleSession {
    return &BattleSession{
        Battle: battle,
        Player: player,
        actions: &combat.ProxyActions{Model: battle.Combat},
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
    model.InvokeSpell(session.spells, army, unitCaster, spell, func(success bool) {
        paid = true
        castedCallback(success)
    })

    // answer the model's question for a target with the command's target
    var targetErr error
    for _, question := range session.drainEvents() {
        if err := session.answerTarget(question, command.Target); err != nil && targetErr == nil {
            targetErr = err
        }
    }
    session.drainEvents()

    if !paid {
        if targetErr != nil {
            return targetErr
        }
        return errors.New("the spell had no effect")
    }

    // a unit that cast is done (as Update does after an action)
    if model.SelectedUnit != nil && model.InControl(player) && (model.SelectedUnit.GetHealth() <= 0 || model.SelectedUnit.MovesLeft.LessThanEqual(fraction.Zero())) {
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
            ask := question.(*combat.CombatSelectTargets)
            unit := targetUnit()
            if unit == nil || !slices.Contains(ask.Targets, unit) {
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

    for _, army := range []*combat.Army{model.AttackingArmy, model.DefendingArmy} {
        var owner *PlayerRef
        if player, ok := army.Player.(*playerlib.Player); ok {
            owner = MakePlayerRef(player)
        }
        for _, unit := range army.GetUnits() {
            if unit.GetHealth() <= 0 {
                continue
            }
            view.Units = append(view.Units, BattleUnitView{
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
            })
        }
    }
    return view
}


func (session *BattleSession) EncodeView() ([]byte, error) {
    return json.Marshal(session.View())
}
