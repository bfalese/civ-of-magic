package game

import (
    "encoding/json"
    "errors"
    "fmt"
    "slices"
    "strings"

    "github.com/bfalese/civ-of-magic/core/combat"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/gamedata"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/core/setup"
)

// A GameHost runs a whole game for one human player who plays through commands (JSON in, JSON
// out): the wasm bridge drives it from a web page, tests drive it directly. It owns the turn loop:
// the human's turn waits for commands; end-turn plays the AI wizards' turns until it's the human's
// turn again.
//
// Events for the frontend collect in an outbox (TakeEvents): the model's events that concern the
// human (concernsHuman). The AI players' own events (their offers, research, battles among
// themselves) are acted on here and not passed on.
//
// The other players' turns run inside the session's coroutine (end-turn is a command like any
// other), so a battle an AI player starts against the human pauses them: WaitingFor says
// "battle" and the battle commands answer it, as for the human's own attacks.
type GameHost struct {
    Model *GameModel
    Human *playerlib.Player
    Session *CommandSession
    // events that failed to encode, or model events without a JSON form
    EventErrors []error

    // the AI side of the game: their turns, battles, spell targets and offers
    ai *HeadlessGame
    // the AI players' moves: battles against the human go to the session
    aiHandlers *hostAiHandlers
    outbox [][]byte
}

// MovementHandler for the AI players' moves: battles involving the human are fought through the
// session (they pause for the human's orders), the others automatically
type hostAiHandlers struct {
    *HeadlessGame
    host *GameHost
}

// an AI stack's step, for the page to animate where the human sees it
func (handlers *hostAiHandlers) ShowMovement(x int, y int, stack *playerlib.UnitStack, center bool) {
    human := handlers.host.Human
    if human.IsVisible(x, y, stack.Plane()) || human.IsVisible(stack.X(), stack.Y(), stack.Plane()) {
        select {
            case handlers.host.Model.Events <- makeStackMoved(handlers.host.Model.GetStackOwner(stack), x, y, stack):
            default:
        }
    }
}

func (handlers *hostAiHandlers) DoCombat(attacker *playerlib.Player, attackerStack *playerlib.UnitStack, defender *playerlib.Player, defenderStack *playerlib.UnitStack, zone combat.ZoneType) combat.CombatState {
    human := handlers.host.Human
    if attacker == human || defender == human {
        return handlers.host.Session.DoCombat(attacker, attackerStack, defender, defenderStack, zone)
    }
    return handlers.HeadlessGame.DoCombat(attacker, attackerStack, defender, defenderStack, zone)
}

type HostSettings struct {
    // 1-4 AI wizards
    Opponents int `json:"opponents"`
    // 0-2
    LandSize int `json:"landSize"`
    Raiders bool `json:"raiders"`
    // intro, easy, average, hard, extreme, impossible (default average)
    Difficulty string `json:"difficulty,omitempty"`
    // weak, normal, powerful (default normal)
    Magic string `json:"magic,omitempty"`
    // the human wizard's name (a random wizard otherwise keeps its own name)
    WizardName string `json:"wizardName,omitempty"`
    // the human wizard; random if not set
    Wizard *setup.WizardCustom `json:"-"`
    // or a premade wizard by name (StartOptions wizards), with a race and banner (random if empty)
    WizardChoice string `json:"wizard,omitempty"`
    // or a wizard made on the start screen (books and retorts), with Race and Banner as above
    Custom *CustomWizardSettings `json:"custom,omitempty"`
    Race string `json:"race,omitempty"`
    Banner string `json:"banner,omitempty"`
    // a setup for tests (see applyScenario): "battle"
    Scenario string `json:"scenario,omitempty"`
}

func parseDifficulty(name string) (data.DifficultySetting, error) {
    switch strings.ToLower(name) {
        case "intro": return data.DifficultyIntro, nil
        case "easy": return data.DifficultyEasy, nil
        case "", "average": return data.DifficultyAverage, nil
        case "hard": return data.DifficultyHard, nil
        case "extreme": return data.DifficultyExtreme, nil
        case "impossible": return data.DifficultyImpossible, nil
    }
    return 0, fmt.Errorf("unknown difficulty %q", name)
}

func parseMagic(name string) (data.MagicSetting, error) {
    switch strings.ToLower(name) {
        case "weak": return data.MagicSettingWeak, nil
        case "", "normal": return data.MagicSettingNormal, nil
        case "powerful": return data.MagicSettingPowerful, nil
    }
    return 0, fmt.Errorf("unknown magic setting %q", name)
}

// the settings from JSON (see HostSettings); empty input means the defaults
func DecodeHostSettings(encoded []byte) (HostSettings, error) {
    settings := HostSettings{Opponents: 3, Raiders: true}
    if len(strings.TrimSpace(string(encoded))) == 0 {
        return settings, nil
    }
    if err := json.Unmarshal(encoded, &settings); err != nil {
        return settings, fmt.Errorf("settings: %v", err)
    }
    return settings, nil
}

// a new game: the human is player 0 and starts its first turn
func MakeGameHost(gameData *gamedata.GameData, settings HostSettings) (*GameHost, error) {
    if settings.Opponents < 1 || settings.Opponents > 4 {
        return nil, fmt.Errorf("opponents must be 1-4, not %v", settings.Opponents)
    }
    if settings.LandSize < 0 || settings.LandSize > 2 {
        return nil, fmt.Errorf("land size must be 0-2, not %v", settings.LandSize)
    }
    difficulty, err := parseDifficulty(settings.Difficulty)
    if err != nil {
        return nil, err
    }
    magic, err := parseMagic(settings.Magic)
    if err != nil {
        return nil, err
    }

    headlessSettings := HeadlessSettings{
        Opponents: settings.Opponents,
        LandSize: settings.LandSize,
        Raiders: settings.Raiders,
        Difficulty: difficulty,
        Magic: magic,
    }

    if settings.Wizard == nil && settings.Custom != nil {
        wizard, err := makeCustomWizard(*settings.Custom, settings.Race, settings.Banner, gameData.Spells)
        if err != nil {
            return nil, err
        }
        settings.Wizard = wizard
    } else if settings.Wizard == nil && settings.WizardChoice != "" {
        wizard, err := makeStartWizard(settings.WizardChoice, settings.Race, settings.Banner, gameData.Spells)
        if err != nil {
            return nil, err
        }
        settings.Wizard = wizard
    } else if settings.Race != "" || settings.Banner != "" {
        return nil, fmt.Errorf("race and banner go with a wizard")
    }

    ai := makeHeadlessModel(gameData, headlessSettings)
    human, err := ai.addPlayers(settings.Wizard, headlessSettings)
    if err != nil {
        return nil, err
    }
    if settings.WizardName != "" {
        human.Wizard.Name = settings.WizardName
    }

    host := makeHost(ai, human)

    ai.Model.CurrentPlayer = 0
    host.startPlayerTurn(human)
    if err := applyScenario(host, settings.Scenario); err != nil {
        return nil, err
    }
    // the first research (the turn loop asks only from the second turn on)
    if !human.ResearchingSpell.Valid() && len(human.ResearchCandidateSpells.Spells) > 0 {
        select {
            case host.Model.Events <- &GameEventResearchSpell{Player: human}:
            default:
        }
    }
    host.drainEvents()

    return host, nil
}

// the host around a model whose players are in place: the session for the human, the AI side
func makeHost(ai *HeadlessGame, human *playerlib.Player) *GameHost {
    host := &GameHost{
        Model: ai.Model,
        Human: human,
        ai: ai,
    }

    host.aiHandlers = &hostAiHandlers{HeadlessGame: ai, host: host}
    host.Session = MakeCommandSession(ai.Model, human)
    host.Session.SuggestCityName = ai.suggestCityName
    // runs inside the session's coroutine, so the AI turns can pause for a battle
    host.Session.EndTurn = host.playOthers
    return host
}

// run a command (or answer what the paused command waits for). An end-turn plays the other
// players' turns before returning, unless one of them attacks the human: then it returns waiting
// for the battle, and the AI turns go on once the battle is over.
func (host *GameHost) Execute(command Command) error {
    if host.GameOver() != "" && host.Session.WaitingFor() == WaitingNothing {
        return errors.New("the game is over")
    }

    err := host.Session.Execute(command)
    host.drainEvents()
    return err
}

// a command as JSON (see DecodeCommand)
func (host *GameHost) ExecuteJSON(encoded []byte) error {
    command, err := DecodeCommand(encoded)
    if err != nil {
        return err
    }
    return host.Execute(command)
}

// what the paused command waits for (see CommandSession.WaitingFor)
func (host *GameHost) WaitingFor() string {
    return host.Session.WaitingFor()
}

// the events since the last call that concern the human, as JSON (see EncodeEvent)
func (host *GameHost) TakeEvents() [][]byte {
    out := host.outbox
    host.outbox = nil
    return out
}

// "" while the game goes on; "won" if the human cast the Spell of Mastery or every other wizard was
// defeated, "lost" if the human was defeated or another wizard cast the Spell of Mastery
func (host *GameHost) GameOver() string {
    if winner := host.Model.MasteryWinner; winner != nil {
        if winner == host.Human {
            return "won"
        }
        return "lost"
    }
    if host.Human.Defeated {
        return "lost"
    }
    if len(host.ai.WizardsLeft()) == 0 {
        return "won"
    }
    return ""
}

func (host *GameHost) PlayerView() ([]byte, error) {
    return host.Model.EncodePlayerView(host.Human)
}

func (host *GameHost) MapView(plane data.Plane) ([]byte, error) {
    return host.Model.EncodeMapView(host.Human, plane)
}

// the surveyor's view of a tile (MakeTileInfo): a location as JSON -> TileInfoView as JSON
func (host *GameHost) TileInfo(encoded []byte) ([]byte, error) {
    var location Location
    if err := json.Unmarshal(encoded, &location); err != nil {
        return nil, fmt.Errorf("tile info: %v", err)
    }
    info, err := host.Model.MakeTileInfo(host.Human, location)
    if err != nil {
        return nil, err
    }
    return json.Marshal(info)
}

// the road build-road would make: {"stack": location, "to": location} -> a JSON list of locations
func (host *GameHost) RoadPath(encoded []byte) ([]byte, error) {
    var query struct {
        Stack Location `json:"stack"`
        To Location `json:"to"`
    }
    if err := json.Unmarshal(encoded, &query); err != nil {
        return nil, fmt.Errorf("road path: %v", err)
    }
    path, err := host.Model.RoadPathFor(host.Human, query.Stack, query.To)
    if err != nil {
        return nil, err
    }
    return json.Marshal(path)
}

// the battlefield, while WaitingFor is "battle"
func (host *GameHost) BattleView() ([]byte, error) {
    if host.Session.Battle == nil {
        return nil, errors.New("no battle")
    }
    return host.Session.Battle.EncodeView()
}

// the other players' turns, until it's the human's turn again (or the game ends)
func (host *GameHost) playOthers() {
    model := host.Model
    for {
        model.DoNextTurn(false, false, host.startPlayerTurn)
        host.drainEvents()

        if host.GameOver() != "" {
            return
        }

        player := model.Players[model.CurrentPlayer]
        if player == host.Human {
            host.continueHumanMoves()
            return
        }

        model.DoAiTurn(host.aiHandlers, player, host.ai.suggestCityName)
        host.drainEvents()
        host.ai.DiscoverWizards()
        host.drainEvents()
    }
}

// at the start of the human's turn, stacks sent further than their moves reached walk on along
// the rest of their path (the desktop does it when it selects them); battles and encounters on the
// way pause the turn as a move-stack would
func (host *GameHost) continueHumanMoves() {
    player := host.Human
    selected := player.SelectedStack
    for _, stack := range slices.Clone(player.Stacks) {
        if len(stack.CurrentPath) == 0 || len(stack.ActiveUnits()) == 0 || !slices.Contains(player.Stacks, stack) {
            continue
        }
        player.SelectedStack = stack
        host.Model.MoveSelectedStack(host.Session, player)
        host.drainEvents()
        if host.GameOver() != "" {
            break
        }
    }
    if !slices.Contains(player.Stacks, selected) {
        selected = nil
    }
    player.SelectedStack = selected
}

func (host *GameHost) startPlayerTurn(player *playerlib.Player) {
    if player != host.Human {
        host.ai.startPlayerTurn(player)
        return
    }

    if !host.Model.StartPlayerTurn(player, host.Model.FinishCast) {
        return
    }
    if player.GlobalEnchantments.Contains(data.EnchantmentAwareness) {
        host.Model.ExploreFogForAwareness(player)
    }
    host.Model.AnnounceVisibleWizards()
}

// whether the frontend should see the event: those naming a player only when it's the human (a
// battle when the human fights in it, a random event when it hits the human or one of their
// cities); events without a player (notices, scrolls, refresh, a banishment) always
func (host *GameHost) concernsHuman(event GameEvent) bool {
    human := host.Human
    switch event := event.(type) {
        case *GameEventStackMoved: return true
        case *GameEventVault: return event.Player == human
        case *GameEventDiplomacy: return event.Player == human
        case *GameEventHireMercenaries: return event.Player == human
        case *GameEventMerchant: return event.Player == human
        case *GameEventHireHero: return event.Player == human
        case *GameEventLearnedSpell: return event.Player == human
        case *GameEventTradeOffer: return event.Player == human
        case *GameEventResearchSpell: return event.Player == human
        case *GameEventNewBuilding: return event.Player == human
        case *GameEventNewOutpost: return event.Player == human
        case *GameEventTreasure: return event.Player == human
        case *GameEventChooseEncounter: return event.Player == human
        case *GameEventChooseRaze: return event.Player == human
        case *GameEventChooseSpellTarget: return event.Player == human
        case *GameEventTreatyProposal: return event.Player == human
        // a war the human declared or started by attacking is news to the human too
        case *GameEventWarDeclared: return event.Player == human || event.Wizard == human
        case *GameEventBattle: return event.Attacker == human || event.Defender == human
        case *GameEventShowRandomEvent:
            if event.Event == nil {
                return true
            }
            if target := event.Event.TargetPlayer; target != nil {
                return target == human
            }
            if city := event.Event.TargetCity; city != nil {
                return city.GetBanner() == human.GetBanner()
            }
            return true
    }
    return true
}

// the model's events that concern the human go to the outbox; the AI players and the session act
// on every event
func (host *GameHost) drainEvents() {
    for {
        select {
            case event := <-host.Model.Events:
                encoded, ok, err := host.Model.EncodeEvent(event)
                if !host.concernsHuman(event) {
                    // the AI side still acts on it below
                } else if err != nil {
                    host.EventErrors = append(host.EventErrors, fmt.Errorf("%T: %v", event, err))
                } else if !ok {
                    host.EventErrors = append(host.EventErrors, fmt.Errorf("%T has no JSON form", event))
                } else {
                    host.outbox = append(host.outbox, encoded)
                }
                host.ai.handleAiEvent(event)
                host.Session.ProcessEvent(event)
            default:
                return
        }
    }
}
