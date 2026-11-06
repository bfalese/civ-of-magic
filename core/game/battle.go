package game

import (
    "encoding/json"
    "errors"

    "github.com/bfalese/civ-of-magic/core/combat"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
)

// A tactical battle played through commands (see CommandSession). The AI moves its own units
// (and the player's when auto is on); when one of the player's units is up, the session waits
// for a battle command. Battle commands act on the selected unit, like the buttons and clicks of
// the Ebitengine battle screen. Casting combat spells isn't available through commands yet.
type BattleSession struct {
    Battle *Battle
    Player *playerlib.Player
    actions *combat.ProxyActions
    spells *combat.ProxySpellSystem
}

func makeBattleSession(battle *Battle, player *playerlib.Player) *BattleSession {
    return &BattleSession{
        Battle: battle,
        Player: player,
        actions: &combat.ProxyActions{Model: battle.Combat},
        spells: &combat.ProxySpellSystem{Model: battle.Combat},
    }
}

func (session *BattleSession) army() *combat.Army {
    return session.Battle.Combat.GetArmyForPlayer(session.Player)
}

func (session *BattleSession) drainEvents() {
    for {
        select {
            case <-session.Battle.Combat.Events:
            default:
                return
        }
    }
}

// let the AI act until one of the player's units is up or the battle is over
func (session *BattleSession) advance() {
    model := session.Battle.Combat
    for steps := 0; steps < 100000; steps++ {
        if model.FinalState() != combat.CombatStateRunning {
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
        default:
            return errors.New("answer with a battle command")
    }
    return nil
}

// battle commands; they answer a CommandSession waiting for a battle

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
        State: battleStateName(model.FinalState()),
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
