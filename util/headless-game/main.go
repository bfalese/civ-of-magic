// headless-game plays a game between AI wizards without a frontend (core only: no Ebitengine,
// no LBX). It needs the data tables exported by util/export-gamedata.
//
//   go run ./util/headless-game -years 100 -opponents 4
package main

import (
    "flag"
    "fmt"
    "io"
    "log"
    "os"
    "path/filepath"
    "time"

    "github.com/bfalese/civ-of-magic/core/data"
    gamecore "github.com/bfalese/civ-of-magic/core/game"
    "github.com/bfalese/civ-of-magic/core/gamedata"
)

func defaultDataPath() string {
    home, err := os.UserHomeDir()
    if err != nil {
        return ""
    }
    return filepath.Join(home, ".local", "share", "civ-of-magic", "gamedata")
}

func main() {
    var dataPath string
    var years int
    var opponents int
    var landSize int
    var raiders bool
    var verbose bool
    var every int
    var printEvents bool
    var printView bool

    flag.StringVar(&dataPath, "gamedata", defaultDataPath(), "directory with gamedata.json (from util/export-gamedata)")
    flag.IntVar(&years, "years", 100, "years (game turns) to play at most")
    flag.IntVar(&opponents, "opponents", 4, "number of AI wizards (1-4)")
    flag.IntVar(&landSize, "land", 0, "land size 0-2")
    flag.BoolVar(&raiders, "raiders", true, "add the raiders (neutral towns)")
    flag.BoolVar(&verbose, "v", false, "show the game's log")
    flag.IntVar(&every, "every", 25, "print a summary every N years")
    flag.BoolVar(&printEvents, "events", false, "print every model event as JSON")
    flag.BoolVar(&printView, "view", false, "print the first remaining wizard's view (JSON) at the end")
    flag.Parse()

    if !verbose {
        log.SetOutput(io.Discard)
    }

    gameData, err := gamedata.Load(dataPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\nExport the data first: go run ./util/export-gamedata -data <MoM install> -out %v\n", err, dataPath)
        os.Exit(1)
    }

    game, err := gamecore.MakeHeadlessGame(gameData, gamecore.HeadlessSettings{
        Opponents: opponents,
        LandSize: landSize,
        Raiders: raiders,
        Difficulty: data.DifficultyAverage,
        Magic: data.MagicSettingNormal,
    })
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }

    if printEvents {
        game.OnEventJSON = func(encoded []byte) {
            fmt.Println(string(encoded))
        }
    }

    start := time.Now()
    played := 0
    for played < years && len(game.WizardsLeft()) > 1 {
        step := min(every, years - played)
        played += game.RunYears(step)
        fmt.Println(game.Summary())
        if step == 0 {
            break
        }
    }

    fmt.Printf("%v years in %v\n", played, time.Since(start).Round(time.Millisecond))

    for _, err := range game.EventErrors {
        fmt.Fprintf(os.Stderr, "event error: %v\n", err)
    }

    if printView && len(game.WizardsLeft()) > 0 {
        encoded, err := game.Model.EncodePlayerView(game.WizardsLeft()[0])
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        } else {
            fmt.Println(string(encoded))
        }
    }
}
