package game

import (
    "errors"
    "fmt"

    "github.com/bfalese/civ-of-magic/core/artifact"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    "github.com/bfalese/civ-of-magic/core/combat"
    "github.com/bfalese/civ-of-magic/core/data"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    "github.com/bfalese/civ-of-magic/core/maplib"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/spellbook"
    "github.com/bfalese/civ-of-magic/lib/coroutine"
)

// A CommandSession plays one (human) player's side of the game through commands, for frontends
// outside the process (the wasm bridge, a network client).
//
// Some commands can't finish right away: a stack walking into a lair asks whether to enter, a
// battle needs the player's orders, a captured city asks raze-or-capture. The command then runs
// as a coroutine that pauses at that point: Execute returns, WaitingFor tells what's needed
// (and an event announces it), and the next command answers it (confirm-encounter, the battle-*
// commands, raze-city). When the answer is complete the paused command continues to its end.
type CommandSession struct {
    Model *GameModel
    Player *playerlib.Player
    // names new outposts
    SuggestCityName func(race data.Race) string
    // ends the player's turn (end-turn); the frontend owns the turn loop
    EndTurn func()

    coro *coroutine.Coroutine
    yield coroutine.YieldFunc
    waiting string
    input Command
    inputErr error

    // the battle being fought, while WaitingFor is "battle"
    Battle *BattleSession
}

const (
    WaitingNothing = ""
    WaitingBattle = "battle"
    WaitingEncounter = "confirm-encounter"
    WaitingRaze = "raze-city"
)

func MakeCommandSession(model *GameModel, player *playerlib.Player) *CommandSession {
    return &CommandSession{
        Model: model,
        Player: player,
    }
}

// what the paused command needs: WaitingNothing, WaitingBattle, WaitingEncounter or WaitingRaze
func (session *CommandSession) WaitingFor() string {
    return session.waiting
}

func isAnswerCommand(command Command) bool {
    switch command.(type) {
        case *ConfirmEncounterCommand, *RazeCityCommand, *BattleActCommand, *BattleDoneCommand,
             *BattleWaitCommand, *BattleFleeCommand, *BattleAutoCommand:
            return true
    }
    return false
}

// run a command, or answer the question the paused command is waiting on
func (session *CommandSession) Execute(command Command) error {
    if session.coro != nil {
        if !isAnswerCommand(command) {
            return fmt.Errorf("waiting for %v first", session.waiting)
        }
        session.input = command
        session.inputErr = nil
        if err := session.resume(); err != nil {
            return err
        }
        return session.inputErr
    }

    if isAnswerCommand(command) {
        return errors.New("nothing is waiting for an answer")
    }

    context := &CommandContext{
        Model: session.Model,
        Player: session.Player,
        Handlers: session,
        SuggestCityName: session.SuggestCityName,
        EndTurn: session.EndTurn,
    }

    session.coro = coroutine.MakeCoroutine(func(yield coroutine.YieldFunc) error {
        session.yield = yield
        return session.Model.Execute(context, command)
    })
    return session.resume()
}

// run the coroutine until it pauses (nil) or ends (the command's error, if any)
func (session *CommandSession) resume() error {
    err := session.coro.Run()
    if err == nil {
        return nil
    }
    session.coro = nil
    session.waiting = WaitingNothing
    session.Battle = nil
    if errors.Is(err, coroutine.CoroutineFinished) {
        return nil
    }
    return err
}

// pause until an answer command arrives; the command is returned. Only called inside the
// coroutine.
func (session *CommandSession) waitForAnswer(kind string) Command {
    session.waiting = kind
    if session.yield() != nil {
        return nil
    }
    command := session.input
    session.input = nil
    return command
}

func (session *CommandSession) announce(event GameEvent) {
    select {
        case session.Model.Events <- event:
        default:
    }
}

// PlayerMovementHandler: the steps of a walking stack

func (session *CommandSession) ShowMovement(x int, y int, stack *playerlib.UnitStack, center bool) {
}

func (session *CommandSession) SteppedStack() {
}

func (session *CommandSession) NextUnit(player *playerlib.Player) {
}

func (session *CommandSession) DiscoverWizards() bool {
    return session.Model.AnnounceVisibleWizards()
}

func (session *CommandSession) ConfirmEncounter(encounter *maplib.ExtraEncounter) bool {
    session.announce(&GameEventChooseEncounter{Player: session.Player, Encounter: encounter.Type.Name(), Guards: len(encounter.Units)})
    for {
        command := session.waitForAnswer(WaitingEncounter)
        if command == nil {
            return false
        }
        if answer, ok := command.(*ConfirmEncounterCommand); ok {
            return answer.Enter
        }
        session.inputErr = errors.New("answer with confirm-encounter")
    }
}

// the attacker won at a city: an AI decides itself, the session's player is asked
func (session *CommandSession) DefeatCity(attacker *playerlib.Player, attackerStack *playerlib.UnitStack, defender *playerlib.Player, city *citylib.City) (bool, int) {
    raze := false
    if attacker == session.Player {
        session.announce(&GameEventChooseRaze{Player: attacker, City: city})
        for {
            command := session.waitForAnswer(WaitingRaze)
            if command == nil {
                break
            }
            if answer, ok := command.(*RazeCityCommand); ok {
                raze = answer.Raze
                break
            }
            session.inputErr = errors.New("answer with raze-city")
        }
    } else if attacker.AIBehavior != nil {
        raze = attacker.AIBehavior.ConfirmRazeTown(city)
    }

    return raze, session.Model.DefeatCity(attacker, defender, city, raze)
}

func (session *CommandSession) DoCombat(attacker *playerlib.Player, attackerStack *playerlib.UnitStack, defender *playerlib.Player, defenderStack *playerlib.UnitStack, zone combat.ZoneType) combat.CombatState {
    model := session.Model
    events := make(chan combat.CombatEvent, 1000)
    battle := model.MakeBattle(model.AllSpells, attacker, attackerStack, defender, defenderStack, zone, events)

    var result BattleResult
    if attacker != session.Player && defender != session.Player {
        result = battle.Run(false)
    } else {
        session.Battle = makeBattleSession(battle, session.Player)
        session.announce(&GameEventBattle{Attacker: attacker, Defender: defender, Location: Location{Plane: defenderStack.Plane(), X: defenderStack.X(), Y: defenderStack.Y()}, Started: true})

        for {
            session.Battle.advance()
            state := session.Battle.State()
            if state != combat.CombatStateRunning {
                result = battle.Result(state)
                break
            }

            command := session.waitForAnswer(WaitingBattle)
            if command == nil {
                // the session was dropped: let the AI finish the battle
                session.Battle.auto()
                continue
            }
            session.inputErr = session.Battle.apply(command)
        }

        session.announce(&GameEventBattle{Attacker: attacker, Defender: defender, Location: Location{Plane: defenderStack.Plane(), X: defenderStack.X(), Y: defenderStack.Y()}, State: battleStateName(result.State)})
        session.Battle = nil
        session.waiting = WaitingNothing
    }

    model.ApplyBattleOutcome(battle, result, session.DefeatCity)
    model.ApplyBattleAftermath(battle, result)
    return result.State
}

func (session *CommandSession) DoEncounter(player *playerlib.Player, stack *playerlib.UnitStack, encounter *maplib.ExtraEncounter, mapUse *maplib.Map, x int, y int) combat.CombatState {
    model := session.Model
    point := data.PlanePoint{X: x, Y: y, Plane: mapUse.Plane}

    createTreasure := func() {
        if model.ArtifactPowers != nil {
            model.CreateTreasure(model.AllSpells.Copy(), model.ArtifactPowers, encounter.Type, encounter.Budget, point, player)
        }
    }

    if len(encounter.Units) == 0 {
        mapUse.RemoveEncounter(x, y)
        createTreasure()
        return combat.CombatStateNoCombat
    }

    defender, enemies, zone := model.EncounterDefenders(encounter, mapUse.Plane, x, y)
    result := session.DoCombat(player, stack, defender, playerlib.MakeUnitStackFromUnits(enemies), zone)
    if model.ResolveEncounter(encounter, mapUse, x, y, enemies, result) {
        createTreasure()
    }
    return result
}

// events that need the model to act for the session's player (the Ebitengine UI does this in its
// event loop): frontends pass every event they take from model.Events through here
func (session *CommandSession) ProcessEvent(event GameEvent) {
    switch event.(type) {
        case *GameEventTreasure:
            treasure := event.(*GameEventTreasure)
            if treasure.Player == session.Player {
                session.Model.ApplyTreasure(&sessionTreasureHandler{session: session, point: treasure.Treasure.Point}, treasure.Player, treasure.Treasure, session.Model.AllSpells)
            }
    }
}

// treasure for the session's player: items wait in the vault, prisoners become an offer
type sessionTreasureHandler struct {
    session *CommandSession
    point data.PlanePoint
}

func (handler *sessionTreasureHandler) GiveArtifact(player *playerlib.Player, item *artifact.Artifact) {
    handler.session.Model.SendVaultItem(&GameEventVault{CreatedArtifact: item, Player: player})
}

func (handler *sessionTreasureHandler) OfferPrisonerHero(player *playerlib.Player, hero *herolib.Hero, point data.PlanePoint) {
    handler.session.Model.SendOffer(&GameEventHireHero{Hero: hero, Player: player, Cost: 0})
}

func (handler *sessionTreasureHandler) ShowLearnedSpell(player *playerlib.Player, spell spellbook.Spell) {
    handler.session.announce(&GameEventLearnedSpell{Player: player, Spell: spell})
}

func battleStateName(state combat.CombatState) string {
    switch state {
        case combat.CombatStateAttackerWin: return "attacker-won"
        case combat.CombatStateDefenderWin: return "defender-won"
        case combat.CombatStateAttackerFlee: return "attacker-fled"
        case combat.CombatStateDefenderFlee: return "defender-fled"
        case combat.CombatStateNoCombat: return "no-combat"
    }
    return "running"
}
