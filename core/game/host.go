package game

import (
    "encoding/json"
    "errors"
    "fmt"
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
// Events for the frontend collect in an outbox (TakeEvents). They are all the model's events,
// including the AI players' (offers to them, their battles); the frontend shows what concerns it.
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

    ai := makeHeadlessModel(gameData, headlessSettings)
    human, err := ai.addPlayers(settings.Wizard, headlessSettings)
    if err != nil {
        return nil, err
    }
    if settings.WizardName != "" {
        human.Wizard.Name = settings.WizardName
    }

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

    ai.Model.CurrentPlayer = 0
    host.startPlayerTurn(human)
    if err := applyScenario(host, settings.Scenario); err != nil {
        return nil, err
    }
    host.drainEvents()

    return host, nil
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

// the events since the last call, as JSON (see EncodeEvent)
func (host *GameHost) TakeEvents() [][]byte {
    out := host.outbox
    host.outbox = nil
    return out
}

// "" while the game goes on; "lost" if the human was defeated, "won" if every other wizard was
func (host *GameHost) GameOver() string {
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
            return
        }

        model.DoAiTurn(host.aiHandlers, player, host.ai.suggestCityName)
        host.drainEvents()
        host.ai.DiscoverWizards()
        host.drainEvents()
    }
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

// every model event goes to the outbox; the AI players and the session act on theirs
func (host *GameHost) drainEvents() {
    for {
        select {
            case event := <-host.Model.Events:
                encoded, ok, err := host.Model.EncodeEvent(event)
                if err != nil {
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
