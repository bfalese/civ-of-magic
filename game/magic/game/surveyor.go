package game

import (
    "fmt"
    "image"

    "github.com/bfalese/civ-of-magic/lib/coroutine"
    // "github.com/bfalese/civ-of-magic/lib/lbx"
    "github.com/bfalese/civ-of-magic/lib/font"
    "github.com/bfalese/civ-of-magic/game/magic/mapview"
    "github.com/bfalese/civ-of-magic/game/magic/util"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/game/magic/scale"
    "github.com/bfalese/civ-of-magic/game/magic/inputmanager"
    fontslib "github.com/bfalese/civ-of-magic/game/magic/fonts"
    uilib "github.com/bfalese/civ-of-magic/game/magic/ui"
    playerlib "github.com/bfalese/civ-of-magic/core/player"
    citylib "github.com/bfalese/civ-of-magic/core/city"
    gamecore "github.com/bfalese/civ-of-magic/core/game"

    "github.com/hajimehoshi/ebiten/v2"
)

func (game *Game) doSurveyor(yield coroutine.YieldFunc) {
    fonts := fontslib.MakeSurveyorFonts(game.Cache)

    var cityMap map[image.Point]*citylib.City

    makeOverworld := func () Overworld {
        cityMap = make(map[image.Point]*citylib.City)

        var cities []*citylib.City
        var stacks []*playerlib.UnitStack
        var fog data.FogMap

        for i, player := range game.Model.Players {
            for _, city := range player.Cities {
                if city.Plane == game.Model.Plane {
                    cities = append(cities, city)
                    cityMap[image.Pt(city.X, city.Y)] = city
                }
            }

            for _, stack := range player.Stacks {
                if stack.Plane() == game.Model.Plane {
                    stacks = append(stacks, stack)
                }
            }

            if i == 0 {
                fog = player.GetFog(game.Model.Plane)
            }
        }

        return Overworld{
            Camera: game.Camera,
            Counter: game.Counter,
            Map: mapview.Renderer{Map: game.Model.CurrentMap()},
            Cities: cities,
            Stacks: stacks,
            SelectedStack: nil,
            ImageCache: &game.ImageCache,
            Fog: fog,
            ShowAnimation: game.State == GameStateUnitMoving,
            FogBlack: game.GetFogImage(),
        }
    }

    overworld := makeOverworld()

    selectedPoint := image.Pt(-1, -1)

    // what the surveyor shows for the tile under the mouse (core MakeTileInfo); nil when none
    var tileInfo *gamecore.TileInfoView

    cancelBackground, _ := game.ImageCache.GetImage("main.lbx", 47, 0)

    quit := false

    ui := &uilib.UI{
        Cache: game.Cache,
        HandleKeys: func(keys []ebiten.Key) {
            for _, key := range keys {
                switch key {
                    case ebiten.KeyF1:
                        quit = true
                }
            }
        },
        Draw: func(ui *uilib.UI, screen *ebiten.Image){
            var options ebiten.DrawImageOptions
            mainHud, _ := game.ImageCache.GetImage("main.lbx", 0, 0)
            scale.DrawScaled(screen, mainHud, &options)

            landImage, _ := game.ImageCache.GetImage("main.lbx", 57, 0)
            options.GeoM.Translate(float64(240), float64(77))
            scale.DrawScaled(screen, landImage, &options)

            options.GeoM.Reset()
            options.GeoM.Translate(float64(240), float64(174))
            scale.DrawScaled(screen, cancelBackground, &options)

            ui.StandardDraw(screen)

            player := game.Model.GetHumanPlayer()

            game.Fonts.WhiteFont.PrintRight(screen, float64(276), float64(68), scale.ScaleAmount, ebiten.ColorScale{}, fmt.Sprintf("%v GP", player.Gold))
            game.Fonts.WhiteFont.PrintRight(screen, float64(313), float64(68), scale.ScaleAmount, ebiten.ColorScale{}, fmt.Sprintf("%v MP", player.Mana))

            fonts.SurveyorFont.PrintCenter(screen, float64(280), float64(81), scale.ScaleAmount, ebiten.ColorScale{}, "Surveyor")

            if tileInfo != nil {
                info := tileInfo
                y := float64(93)
                width := float64(cancelBackground.Bounds().Dx() - 5)
                white := func(text string) {
                    fonts.WhiteFont.PrintWrapCenter(screen, float64(280), y, width, scale.ScaleAmount, ebiten.ColorScale{}, text)
                    y += float64(fonts.WhiteFont.Height())
                }
                yellow := func(text string) {
                    fonts.YellowFont.PrintWrapCenter(screen, float64(280), y, width, scale.ScaleAmount, ebiten.ColorScale{}, text)
                    y += float64(fonts.YellowFont.Height())
                }

                // Terrain and its bonuses
                yellow(info.Terrain)
                if info.Corrupted {
                    white("Corruption")
                }
                if info.FoodText != "" {
                    white(fmt.Sprintf("%v food", info.FoodText))
                }
                if info.ProductionBonus != 0 {
                    white(fmt.Sprintf("+%v%% production", info.ProductionBonus))
                }
                if info.GoldBonus != 0 {
                    white(fmt.Sprintf("+%v%% gold", info.GoldBonus))
                }
                y += float64(fonts.WhiteFont.Height())

                // Bonuses
                if bonus := info.Bonus; bonus != nil {
                    yellow(bonus.Name)
                    if bonus.Food != 0 {
                        white(fmt.Sprintf("+%v food", bonus.Food))
                    }
                    if bonus.Gold != 0 {
                        white(fmt.Sprintf("+%v gold", bonus.Gold))
                    }
                    if bonus.Power != 0 {
                        white(fmt.Sprintf("+%v power", bonus.Power))
                    }
                    if bonus.UnitCostReduction != 0 {
                        white(fmt.Sprintf("Reduces normal unit cost by %v%%", bonus.UnitCostReduction))
                    }
                }

                // Nodes
                if node := info.Node; node != nil {
                    yellow(node.Kind)
                    if node.Warped {
                        white("Warped")
                    } else if node.Spirit != "" {
                        white(node.Spirit)
                    }
                }

                // Lairs and their guards (a node's guards keep the node's name)
                if encounter := info.Encounter; encounter != nil {
                    if info.Node == nil {
                        yellow(encounter.Type)
                    }
                    white(encounter.Guards)
                }

                // FIXME: how should this behave for different fog types?
                if info.City != "" {
                    yellow(info.City)
                }

                cityText := "City Resources"
                if info.Site.Reason != "" && info.Site.MaximumPopulation == 0 {
                    cityText = info.Site.Reason
                }
                cityInfoText := fonts.YellowFont.CreateWrappedText(float64(cancelBackground.Bounds().Dx() - 9), 1, cityText)
                resources := info.Site.MaximumPopulation > 0 || info.Site.CanBuild

                y = float64(170) - cityInfoText.TotalHeight
                if resources {
                    y = float64(170) - float64(fonts.WhiteFont.Height()) * 3 - cityInfoText.TotalHeight
                }

                fonts.YellowFont.RenderWrapped(screen, float64(245), y, cityInfoText, font.FontOptions{Scale: scale.ScaleAmount})
                y += cityInfoText.TotalHeight

                if resources {
                    fonts.WhiteFont.Print(screen, float64(245), y, scale.ScaleAmount, ebiten.ColorScale{}, "Maximum Pop")
                    fonts.WhiteFont.PrintRight(screen, float64(308), y, scale.ScaleAmount, ebiten.ColorScale{}, fmt.Sprintf("%v", info.Site.MaximumPopulation))
                    y += float64(fonts.WhiteFont.Height())
                    fonts.WhiteFont.Print(screen, float64(245), y, scale.ScaleAmount, ebiten.ColorScale{}, "Prod Bonus")
                    fonts.WhiteFont.PrintRight(screen, float64(314), y, scale.ScaleAmount, ebiten.ColorScale{}, fmt.Sprintf("+%v%%", info.Site.ProductionBonus))
                    y += float64(fonts.WhiteFont.Height())
                    fonts.WhiteFont.Print(screen, float64(245), y, scale.ScaleAmount, ebiten.ColorScale{}, "Gold Bonus")
                    fonts.WhiteFont.PrintRight(screen, float64(314), y, scale.ScaleAmount, ebiten.ColorScale{}, fmt.Sprintf("+%v%%", info.Site.GoldBonus))
                }
            }
        },
    }

    ui.SetElementsFromArray(nil)

    makeButton := func(lbxIndex int, x int, y int) *uilib.UIElement {
        button, _ := game.ImageCache.GetImage("main.lbx", lbxIndex, 0)
        var options ebiten.DrawImageOptions
        options.GeoM.Translate(float64(x), float64(y))
        return &uilib.UIElement{
            Draw: func(element *uilib.UIElement, screen *ebiten.Image){
                scale.DrawScaled(screen, button, &options)
            },
        }
    }

    // game
    ui.AddElement(makeButton(1, 7, 4))

    // spells
    ui.AddElement(makeButton(2, 47, 4))

    // army button
    ui.AddElement(makeButton(3, 89, 4))

    // cities button
    ui.AddElement(makeButton(4, 140, 4))

    // magic button
    ui.AddElement(makeButton(5, 184, 4))

    // info button
    ui.AddElement(makeButton(6, 226, 4))

    // plane button
    ui.AddElement((func () *uilib.UIElement {
        buttons, _ := game.ImageCache.GetImages("main.lbx", 7)
        x := 270
        y := 4

        clicked := false

        var options ebiten.DrawImageOptions
        options.GeoM.Translate(float64(x), float64(y))

        return &uilib.UIElement{
            Rect: util.ImageRect(x, y, buttons[0]),
            PlaySoundLeftClick: true,
            LeftClick: func(element *uilib.UIElement){
                clicked = true
            },
            LeftClickRelease: func(element *uilib.UIElement){
                clicked = false
                game.Model.SwitchPlane()
                overworld = makeOverworld()
            },
            Draw: func(element *uilib.UIElement, screen *ebiten.Image){
                if clicked {
                    scale.DrawScaled(screen, buttons[1], &options)
                } else {
                    scale.DrawScaled(screen, buttons[0], &options)
                }
            },
        }
    })())

    // cancel button at bottom
    cancel, _ := game.ImageCache.GetImages("main.lbx", 41)
    cancelIndex := 0
    cancelRect := util.ImageRect(263, 182, cancel[0])
    ui.AddElement(&uilib.UIElement{
        Rect: cancelRect,
        LeftClick: func(element *uilib.UIElement){
            cancelIndex = 1
        },
        LeftClickRelease: func(element *uilib.UIElement){
            cancelIndex = 0
            quit = true
        },
        Draw: func(element *uilib.UIElement, screen *ebiten.Image){
            var options ebiten.DrawImageOptions
            options.GeoM.Translate(float64(cancelRect.Min.X), float64(cancelRect.Min.Y))
            scale.DrawScaled(screen, cancel[cancelIndex], &options)
        },
    })

    game.PushDrawer(func(screen *ebiten.Image){
        overworld.Camera = game.Camera

        overworld.DrawOverworld(screen, ebiten.GeoM{})

        var miniGeom ebiten.GeoM
        miniGeom.Translate(float64(250), float64(20))
        mx, my := miniGeom.Apply(0, 0)
        miniWidth := 60
        miniHeight := 31
        mini := screen.SubImage(scale.ScaleRect(image.Rect(int(mx), int(my), int(mx) + miniWidth, int(my) + miniHeight))).(*ebiten.Image)
        overworld.DrawMinimap(mini)

        ui.Draw(ui, screen)
    })
    defer game.PopDrawer()

    for !quit {
        overworld.Counter += 1
        zoomed := game.doInputZoom(yield)
        _ = zoomed

        ui.StandardUpdate()

        x, y := inputmanager.MousePosition()

        // within the viewable area
        if game.InOverworldArea(x, y) {
            newX, newY := game.ScreenToTile(float64(x), float64(y))
            newPoint := image.Pt(newX, newY)

            // right click should move the camera
            rightClick := inputmanager.RightClick()
            if rightClick /*|| zoomed*/ {
                game.doMoveCamera(yield, newX, newY)
            }

            if selectedPoint != newPoint {
                selectedPoint = newPoint
                tileInfo = nil
                info, err := game.Model.MakeTileInfo(game.Model.GetHumanPlayer(), gamecore.Location{Plane: game.Model.Plane, X: newX, Y: newY})
                if err == nil {
                    tileInfo = &info
                }
            }
        } else {
            tileInfo = nil
            selectedPoint = image.Pt(-1, -1)
        }

        yield()
    }
}
