package main

import (
    "log"

    "github.com/bfalese/civ-of-magic/lib/lbx"
    "github.com/bfalese/civ-of-magic/lib/coroutine"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/core/setup"
    "github.com/bfalese/civ-of-magic/game/magic/diplomacy"
    "github.com/bfalese/civ-of-magic/game/magic/scale"
    herolib "github.com/bfalese/civ-of-magic/core/hero"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    "github.com/bfalese/civ-of-magic/game/magic/lbxdata"

    "github.com/hajimehoshi/ebiten/v2"
    "github.com/hajimehoshi/ebiten/v2/inpututil"
)

type Engine struct {
    LbxCache *lbx.LbxCache
    DiplomacyDraw func(*ebiten.Image)
    Coroutine *coroutine.Coroutine
}

func NewEngine() (*Engine, error) {
    cache := lbx.AutoCache()

    player := playerlib.MakePlayer(
        setup.WizardCustom{
            Name: "Gandalf",
        },
        true, 1, 1, make(map[herolib.HeroType]string),
        &playerlib.NoGlobalEnchantments{},
    )

    player.Gold = 234
    player.Mana = 981
    player.CastingSkillPower = 28

    allSpells, err := lbxdata.ReadSpellsFromCache(cache)
    if err != nil {
        return nil, err
    }

    player.KnownSpells.AddAllSpells(allSpells.GetSpellsByMagic(data.LifeMagic))

    player.Wizard.ToggleRetort(data.RetortAlchemy, 2)

    enemy1 := playerlib.MakePlayer(
        setup.WizardCustom{
            Base: data.WizardTauron,
            Name: "Merlin",
            Banner: data.BannerPurple,
        },
        false, 1, 1, make(map[herolib.HeroType]string),
        &playerlib.NoGlobalEnchantments{},
    )

    enemy1.KnownSpells.AddAllSpells(allSpells.GetSpellsByMagic(data.NatureMagic))
    enemy1.AwarePlayer(player)

    logic, draw := diplomacy.ShowDiplomacyScreen(cache, player, enemy1, 1458)

    run := func(yield coroutine.YieldFunc) error {
        logic(yield)
        return ebiten.Termination
    }

    return &Engine{
        LbxCache: cache,
        Coroutine: coroutine.MakeCoroutine(run),
        DiplomacyDraw: draw,
    }, nil
}

func (engine *Engine) Update() error {

    keys := make([]ebiten.Key, 0)
    keys = inpututil.AppendJustPressedKeys(keys)

    for _, key := range keys {
        if key == ebiten.KeyEscape || key == ebiten.KeyCapsLock {
            return ebiten.Termination
        }
    }

    if engine.Coroutine.Run() != nil {
        return ebiten.Termination
    }

    return nil
}

func (engine *Engine) Draw(screen *ebiten.Image) {
    engine.DiplomacyDraw(screen)
}

func (engine *Engine) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
    return scale.Scale2(data.ScreenWidth, data.ScreenHeight)
}

func main(){
    log.SetFlags(log.Ldate | log.Lshortfile | log.Lmicroseconds)

    ebiten.SetWindowSize(data.ScreenWidth * 4, data.ScreenHeight * 4)
    ebiten.SetWindowTitle("diplomacy")
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
