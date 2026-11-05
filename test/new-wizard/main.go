package main

import (
    "log"

    "github.com/bfalese/civ-of-magic/game/magic/setupview"

    "github.com/bfalese/civ-of-magic/lib/lbx"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/game/magic/scale"

    "github.com/hajimehoshi/ebiten/v2"
    "github.com/hajimehoshi/ebiten/v2/inpututil"
)

type Engine struct {
    LbxCache *lbx.LbxCache
    NewWizardScreen *setupview.NewWizardScreen
}

func NewEngine() (*Engine, error) {
    cache := lbx.AutoCache()

    screen := setupview.MakeNewWizardScreen(cache)

    // screen.Activate()

    return &Engine{
        LbxCache: cache,
        NewWizardScreen: screen,
    }, nil
}

func (engine *Engine) Update() error {

    keys := make([]ebiten.Key, 0)
    keys = inpututil.AppendJustPressedKeys(keys)

    /*
    for _, key := range keys {
        if key == ebiten.KeyCapsLock {
            return ebiten.Termination
        }
    }
    */

    switch engine.NewWizardScreen.Update() {
        case setupview.NewWizardScreenStateFinished:
            wizard := engine.NewWizardScreen.CustomWizard
            log.Printf("New wizard: %+v", wizard)
            return ebiten.Termination
        case setupview.NewWizardScreenStateCanceled:
            return ebiten.Termination
    }

    return nil
}

func (engine *Engine) Draw(screen *ebiten.Image) {
    engine.NewWizardScreen.Draw(screen)
}

func (engine *Engine) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
    return scale.Scale2(data.ScreenWidth, data.ScreenHeight)
}

func main(){

    log.SetFlags(log.Ldate | log.Lshortfile | log.Lmicroseconds)

    monitorWidth, _ := ebiten.Monitor().Size()

    size := monitorWidth / 390

    ebiten.SetWindowSize(data.ScreenWidth * size, data.ScreenHeight * size)

    ebiten.SetWindowTitle("new wizard")
    ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

    engine, err := NewEngine()

    if err != nil {
        log.Printf("Error: unable to load engine: %v", err)
        return
    }

    err = ebiten.RunGame(engine)
    if err != nil {
        log.Printf("Error: %v", err)
    }

}
