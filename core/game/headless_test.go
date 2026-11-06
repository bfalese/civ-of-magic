package game

import (
    "io"
    "log"
    "os"
    "testing"

    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/gamedata"
)

// a whole AI-vs-AI game for a few decades, no frontend. Needs the exported data tables:
// CIV_GAMEDATA=<dir with gamedata.json> (see util/export-gamedata); skipped otherwise.
func TestHeadlessGame(test *testing.T) {
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

    game, err := MakeHeadlessGame(gameData, HeadlessSettings{Opponents: 3, Raiders: true, Difficulty: data.DifficultyAverage, Magic: data.MagicSettingNormal})
    if err != nil {
        test.Fatalf("setup: %v", err)
    }

    if len(game.WizardsLeft()) != 3 {
        test.Fatalf("expected 3 wizards, got %v", len(game.WizardsLeft()))
    }
    for _, wizard := range game.WizardsLeft() {
        if len(wizard.Cities) != 1 || len(wizard.Stacks) == 0 {
            test.Errorf("%v should start with a capital and units", wizard.Wizard.Name)
        }
    }

    years := game.RunYears(40)
    test.Log(game.Summary())

    if years == 0 {
        test.Fatalf("no years were played")
    }
    if game.Model.TurnNumber < 40 && len(game.WizardsLeft()) > 1 {
        test.Errorf("game stopped early at turn %v", game.Model.TurnNumber)
    }
}
