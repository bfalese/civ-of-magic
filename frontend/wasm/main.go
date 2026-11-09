//go:build js && wasm

// The game core as WebAssembly, for web frontends (the Three.js renderer to come; for now the test
// page next to this file). Only core: no Ebitengine, no LBX. The page loads the user's own
// gamedata.json (exported with util/export-gamedata; never shipped) and drives a GameHost:
//
//   civ.newGame(gamedataJSON, settingsJSON) -> status
//   civ.execute(commandJSON)                -> status
//   civ.events()                            -> JSON array of the events since the last call
//   civ.view()                              -> the player view (JSON)
//   civ.map("Arcanus" | "Myrror")           -> the map view (JSON)
//   civ.battle()                            -> the battle view (JSON), while waiting is "battle"
//   civ.setLogging(true)                    -> the game's log on the console (off by default)
//
// Every function returns a JSON string. status is {"error"?, "waiting", "gameOver"?}; views are
// {"error": ...} when they fail. The JSON forms are in docs/frontend-protocol.md.
//
//   GOOS=js GOARCH=wasm go build -o frontend/wasm/core.wasm ./frontend/wasm   (or: make core-wasm)
package main

import (
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "log"
    "os"
    "syscall/js"

    "github.com/bfalese/civ-of-magic/core/data"
    gamecore "github.com/bfalese/civ-of-magic/core/game"
    "github.com/bfalese/civ-of-magic/core/gamedata"
)

var host *gamecore.GameHost

type status struct {
    Error string `json:"error,omitempty"`
    Waiting string `json:"waiting"`
    GameOver string `json:"gameOver,omitempty"`
}

func errorJSON(err error) string {
    encoded, _ := json.Marshal(struct {
        Error string `json:"error"`
    }{err.Error()})
    return string(encoded)
}

func statusJSON(err error) string {
    out := status{}
    if err != nil {
        out.Error = err.Error()
    }
    if host != nil {
        out.Waiting = host.WaitingFor()
        out.GameOver = host.GameOver()
    }
    encoded, _ := json.Marshal(out)
    return string(encoded)
}

func stringArgument(arguments []js.Value, index int) (string, error) {
    if index >= len(arguments) || arguments[index].Type() != js.TypeString {
        return "", fmt.Errorf("argument %v must be a string", index + 1)
    }
    return arguments[index].String(), nil
}

// a JS function returning a JSON string; a Go panic becomes an error instead of killing the module
func export(name string, call func(arguments []js.Value) string, onPanic func(err error) string) js.Func {
    return js.FuncOf(func(this js.Value, arguments []js.Value) (result any) {
        defer func() {
            if recovered := recover(); recovered != nil {
                result = onPanic(fmt.Errorf("%v: panic: %v", name, recovered))
            }
        }()
        return call(arguments)
    })
}

func newGame(arguments []js.Value) string {
    encodedData, err := stringArgument(arguments, 0)
    if err != nil {
        return statusJSON(err)
    }
    gameData, err := gamedata.Parse([]byte(encodedData))
    if err != nil {
        return statusJSON(err)
    }

    encodedSettings := ""
    if len(arguments) > 1 && arguments[1].Type() == js.TypeString {
        encodedSettings = arguments[1].String()
    }
    settings, err := gamecore.DecodeHostSettings([]byte(encodedSettings))
    if err != nil {
        return statusJSON(err)
    }

    made, err := gamecore.MakeGameHost(gameData, settings)
    if err != nil {
        return statusJSON(err)
    }
    host = made
    return statusJSON(nil)
}

var errNoGame = errors.New("no game: call newGame first")

func execute(arguments []js.Value) string {
    if host == nil {
        return statusJSON(errNoGame)
    }
    encoded, err := stringArgument(arguments, 0)
    if err != nil {
        return statusJSON(err)
    }
    return statusJSON(host.ExecuteJSON([]byte(encoded)))
}

func events(arguments []js.Value) string {
    if host == nil {
        return "[]"
    }
    var raw []json.RawMessage
    for _, event := range host.TakeEvents() {
        raw = append(raw, event)
    }
    if raw == nil {
        return "[]"
    }
    encoded, err := json.Marshal(raw)
    if err != nil {
        return errorJSON(err)
    }
    return string(encoded)
}

func viewResult(encoded []byte, err error) string {
    if err != nil {
        return errorJSON(err)
    }
    return string(encoded)
}

func view(arguments []js.Value) string {
    if host == nil {
        return errorJSON(errNoGame)
    }
    return viewResult(host.PlayerView())
}

func mapView(arguments []js.Value) string {
    if host == nil {
        return errorJSON(errNoGame)
    }
    plane := data.PlaneArcanus
    if name, err := stringArgument(arguments, 0); err == nil {
        switch name {
            case "Arcanus":
            case "Myrror": plane = data.PlaneMyrror
            default: return errorJSON(fmt.Errorf("unknown plane %q", name))
        }
    }
    return viewResult(host.MapView(plane))
}

func battle(arguments []js.Value) string {
    if host == nil {
        return errorJSON(errNoGame)
    }
    return viewResult(host.BattleView())
}

// the choices for a new game (wizards, races, banners, difficulties, ...): core StartOptions
func startOptions(arguments []js.Value) string {
    encoded, err := json.Marshal(gamecore.MakeStartOptions())
    if err != nil {
        return errorJSON(err)
    }
    return string(encoded)
}

func setLogging(arguments []js.Value) string {
    if len(arguments) > 0 && arguments[0].Truthy() {
        log.SetOutput(os.Stderr)
    } else {
        log.SetOutput(io.Discard)
    }
    return "{}"
}

func main() {
    log.SetOutput(io.Discard)

    api := js.Global().Get("Object").New()
    api.Set("newGame", export("newGame", newGame, statusJSON))
    api.Set("execute", export("execute", execute, statusJSON))
    api.Set("events", export("events", events, errorJSON))
    api.Set("view", export("view", view, errorJSON))
    api.Set("map", export("map", mapView, errorJSON))
    api.Set("battle", export("battle", battle, errorJSON))
    api.Set("setLogging", export("setLogging", setLogging, errorJSON))
    api.Set("startOptions", export("startOptions", startOptions, errorJSON))
    js.Global().Set("civ", api)

    // tell the page the API is there
    if ready := js.Global().Get("onCivReady"); ready.Type() == js.TypeFunction {
        ready.Invoke()
    }

    select {}
}
