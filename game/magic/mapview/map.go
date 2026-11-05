package mapview

import (
    "log"
    "math"
    "image"
    "image/color"

    "github.com/bfalese/civ-of-magic/core/maplib"
    "github.com/bfalese/civ-of-magic/lib/functional"
    "github.com/bfalese/civ-of-magic/core/terrain"
    "github.com/bfalese/civ-of-magic/game/magic/util"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/game/magic/scale"
    cameralib "github.com/bfalese/civ-of-magic/game/magic/camera"

    "github.com/hajimehoshi/ebiten/v2"
    "github.com/hajimehoshi/ebiten/v2/vector"
)

func ResetCache(mapObject *maplib.Map) {
    getRenderCache(mapObject).tiles = make(map[int]*ebiten.Image)
}

func GetTileImage(mapObject *maplib.Map, tileX int, tileY int, animationCounter uint64) (*ebiten.Image, error) {
    tile := mapObject.Map.Terrain[tileX][tileY]
    tileInfo := mapObject.Data.Tiles[tile]

    animationIndex := animationCounter % uint64(len(tileInfo.Images))

    if image, ok := getRenderCache(mapObject).tiles[tile * 0x1000 + int(animationIndex)]; ok {
        return image, nil
    }

    gpuImage := ebiten.NewImageFromImage(tileInfo.Images[animationCounter % uint64(len(tileInfo.Images))])

    getRenderCache(mapObject).tiles[tile * 0x1000 + int(animationIndex)] = gpuImage
    return gpuImage, nil
}

func bannerColor(banner data.BannerType) color.RGBA {
    switch banner {
        case data.BannerBlue: return color.RGBA{R: 0, G: 0, B: 255, A: 255}
        case data.BannerGreen: return color.RGBA{R: 0, G: 255, B: 0, A: 255}
        case data.BannerPurple: return color.RGBA{R: 255, G: 0, B: 255, A: 255}
        case data.BannerRed: return color.RGBA{R: 255, G: 0, B: 0, A: 255}
        case data.BannerYellow: return color.RGBA{R: 255, G: 255, B: 0, A: 255}
        case data.BannerBrown: return color.RGBA{R: 0xdb, G: 0x7e, B: 0x1f, A: 255}
    }

    return color.RGBA{R: 0, G: 0, B: 0, A: 255}
}

/* an experiment to draw the entire minimap as a paletted image
func DrawMinimap2(mapObject *maplib.Map, screen *ebiten.Image, cities []maplib.MiniMapCity, centerX int, centerY int, zoom float64, fog [][]bool, counter uint64, crosshairs bool){
    // draw entire minimap as a paletted image
    // find a subimage that corresponds to the centerX/centerY position
    // use writePixels to copy pixels from sub image to the screen

    cursorColorBlue := math.Sin(float64(counter) / 10.0) * 127.0 + 127.0
    if cursorColorBlue > 255 {
        cursorColorBlue = 255
    }
    cursorColor := util.PremultiplyAlpha(color.RGBA{R: 255, G: 255, B: byte(cursorColorBlue), A: 180})

    landColor := color.RGBA{R: 0, G: 0xad, B: 0x00, A: 255}
    oceanColor := color.RGBA{R: 0, G: 0, B: 255, A: 255}
    riverColor := color.RGBA{R: 0x3f, G: 0x88, B: 0xd3, A: 255}
    mountainColor := color.RGBA{R: 0xbc, G: 0xd0, B: 0xe4, A: 255}
    desertColor := color.RGBA{R: 0xdb, G: 0xbd, B: 0x29, A: 255}
    tundraColor := color.RGBA{R: 0xd6, G: 0xd4, B: 0xc9, A: 255}
    volcanoColor := color.RGBA{R: 0xff, G: 0x00, B: 0x00, A: 255}
    lakeColor := color.RGBA{R: 0x3f, G: 0x88, B: 0xd3, A: 255}
    natureNodeColor := color.RGBA{R: 0, G: 255, B: 0, A: 255}
    sorceryNodeColor := color.RGBA{R: 0, G: 0, B: 255, A: 255}
    chaosNodeColor := color.RGBA{R: 0xff, G: 0x00, B: 0x00, A: 255}
    defaultColor := color.RGBA{R: 64, G: 64, B: 64, A: 255}
    blackColor := color.RGBA{R: 0, G: 0, B: 0, A: 255}

    bannerGreen := bannerColor(data.BannerGreen)
    bannerBlue := bannerColor(data.BannerBlue)
    bannerPurple := bannerColor(data.BannerPurple)
    bannerRed := bannerColor(data.BannerRed)
    bannerYellow := bannerColor(data.BannerYellow)
    bannerBrown := bannerColor(data.BannerBrown)

    mapPalette := color.Palette{
        cursorColor,
        landColor,
        oceanColor,
        riverColor,
        mountainColor,
        desertColor,
        tundraColor,
        volcanoColor,
        lakeColor,
        natureNodeColor,
        sorceryNodeColor,
        chaosNodeColor,
        defaultColor,
        blackColor,

        bannerGreen,
        bannerBlue,
        bannerPurple,
        bannerRed,
        bannerYellow,
        bannerBrown,
    }

    const cursorIndex = 0
    const grassIndex = 1
    const shoreIndex = 1
    const hillIndex = 1
    const SwampIndex = 1
    const ForestIndex = 1
    const oceanIndex = 2
    const riverIndex = 3
    const mountainIndex = 4
    const desertIndex = 5
    const tundraIndex = 6
    const volcanoIndex = 7
    const lakeIndex = 8
    const natureNodeIndex = 9
    const sorceryNodeIndex = 10
    const chaosNodeIndex = 11
    const defaultIndex = 12
    const blackIndex = 13
    const bannerGreenIndex = 14
    const bannerBlueIndex = 15
    const bannerPurpleIndex = 16
    const bannerRedIndex = 17
    const bannerYellowIndex = 18
    const bannerBrownIndex = 19

    if len(getRenderCache(mapObject).miniMapPixels) != screen.Bounds().Dx() * screen.Bounds().Dy() * 4 {
        log.Printf("set minimap pixels to %v", screen.Bounds().Dx() * screen.Bounds().Dy() * 4)
        getRenderCache(mapObject).miniMapPixels = make([]byte, screen.Bounds().Dx() * screen.Bounds().Dy() * 4)
        mapObject.miniMapImage = image.NewPaletted(image.Rect(0, 0, mapObject.Map.Columns(), mapObject.Map.Rows()), mapPalette)
    }

    mapObject.miniMapImage.Palette = mapPalette

    for x := 0; x < mapObject.Map.Columns(); x++ {
        for y := 0; y < mapObject.Map.Rows(); y++ {
            if !fog[x][y] {
                mapObject.miniMapImage.SetColorIndex(x, y, uint8(blackIndex))
            } else {
                var index = 0
                switch terrain.GetTile(mapObject.Map.Terrain[x][y]).TerrainType() {
                    case terrain.Grass: index = grassIndex
                    case terrain.Ocean: index = oceanIndex
                    case terrain.River: index = riverIndex
                    case terrain.Shore: index = shoreIndex
                    case terrain.Mountain: index = mountainIndex
                    case terrain.Hill: index = hillIndex
                    case terrain.Swamp: index = SwampIndex
                    case terrain.Forest: index = ForestIndex
                    case terrain.Desert: index = desertIndex
                    case terrain.Tundra: index = tundraIndex
                    case terrain.Volcano: index = volcanoIndex
                    case terrain.Lake: index = lakeIndex
                    case terrain.NatureNode: index = natureNodeIndex
                    case terrain.SorceryNode: index = sorceryNodeIndex
                    case terrain.ChaosNode: index = chaosNodeIndex
                    default: index = defaultIndex
                }

                mapObject.miniMapImage.SetColorIndex(x, y, uint8(index))
            }
        }
    }

    for _, city := range cities {
        x, y := city.GetX(), city.GetY()
        if fog[x][y] {
            index := bannerGreenIndex
            switch city.GetBanner() {
                case data.BannerBlue: index = bannerBlueIndex
                case data.BannerGreen: index = bannerGreenIndex
                case data.BannerPurple: index = bannerPurpleIndex
                case data.BannerRed: index = bannerRedIndex
                case data.BannerYellow: index = bannerYellowIndex
                case data.BannerBrown: index = bannerBrownIndex
            }
            mapObject.miniMapImage.SetColorIndex(x, y, uint8(index))
        }
    }

    pixels := getRenderCache(mapObject).miniMapPixels
    rowSize := screen.Bounds().Dx()

    set := func(x int, y int, c color.Color){
        baseIndex := (y * rowSize + x) * 4

        / *
        if baseIndex > len(getRenderCache(mapObject).miniMapPixels) {
            return
        }
        * /

        r, g, b, a := c.RGBA()

        pixels[baseIndex + 0] = byte(r >> 8)
        pixels[baseIndex + 1] = byte(g >> 8)
        pixels[baseIndex + 2] = byte(b >> 8)
        pixels[baseIndex + 3] = byte(a >> 8)
    }


    for x := range mapObject.Map.Columns() {
        for y := range mapObject.Map.Rows() {
            c := mapObject.miniMapImage.At(x, y)
            // set all the pixels in a block

            if data.ScreenScale == 1 {
                set(x, y, c)
            } else {
                for dx := range data.ScreenScale {
                    for dy := range data.ScreenScale {
                        cx := (mapObject.WrapX(x + centerX)) * data.ScreenScale + dx
                        cy := y + dy + centerY
                        // log.Printf("set at %v, %v %v, %v", x, y, cx, cy)
                        if cx < 0 || cx >= screen.Bounds().Dx() || cy < 0 || cy >= screen.Bounds().Dy() {
                            continue
                        }
                        set(cx, cy, c)
                    }
                }
            }
        }
    }

    screen.Fill(color.RGBA{R: 0, G: 0, B: 0, A: 255})
    screen.WritePixels(getRenderCache(mapObject).miniMapPixels)
}
*/

func DrawMinimap(mapObject *maplib.Map, screen *ebiten.Image, cities []maplib.MiniMapCity, centerX int, centerY int, zoom float64, fog data.FogMap, counter uint64, crosshairs bool){
    if len(getRenderCache(mapObject).miniMapPixels) != screen.Bounds().Dx() * screen.Bounds().Dy() * 4 {
        // log.Printf("set minimap pixels to %v", screen.Bounds().Dx() * screen.Bounds().Dy() * 4)
        getRenderCache(mapObject).miniMapPixels = make([]byte, screen.Bounds().Dx() * screen.Bounds().Dy() * 4)
    }

    rowSize := screen.Bounds().Dx()

    cameraX := scale.Scale(centerX) - screen.Bounds().Dx() / 2
    cameraY := scale.Scale(centerY) - screen.Bounds().Dy() / 2

    /*
    if cameraX < 0 {
        cameraX = 0
    }
    */

    if cameraY < 0 {
        cameraY = 0
    }

    /*
    if cameraX > mapObject.Map.Columns() - screen.Bounds().Dx() {
        cameraX = mapObject.Map.Columns() - screen.Bounds().Dx()
    }
    */

    if cameraY > scale.Scale(mapObject.Map.Rows()) - screen.Bounds().Dy() {
        cameraY = scale.Scale(mapObject.Map.Rows()) - screen.Bounds().Dy()
    }

    set := func(x int, y int, c color.RGBA){
        baseIndex := (y * rowSize + x) * 4

        /*
        if baseIndex > len(getRenderCache(mapObject).miniMapPixels) {
            return
        }
        */

        r, g, b, a := c.RGBA()

        getRenderCache(mapObject).miniMapPixels[baseIndex + 0] = byte(r >> 8)
        getRenderCache(mapObject).miniMapPixels[baseIndex + 1] = byte(g >> 8)
        getRenderCache(mapObject).miniMapPixels[baseIndex + 2] = byte(b >> 8)
        getRenderCache(mapObject).miniMapPixels[baseIndex + 3] = byte(a >> 8)
    }

    black := color.RGBA{R: 0, G: 0, B: 0, A: 255}

    cityLocations := make(map[image.Point]color.RGBA)

    for _, city := range cities {
        // FIXME: how should this behave for different fog types?
        if fog[city.GetX()][city.GetY()] != data.FogTypeUnexplored {
            cityLocations[image.Pt(city.GetX(), city.GetY())] = bannerColor(city.GetBanner())
        }
    }

    var getMapColor func(terrain.TerrainType, data.FogType) color.RGBA

    oceanColor := color.RGBA{R: 0x11, G: 0x0f, B: 0x8d, A: 255}
    chaosNodeColor := color.RGBA{R: 0x80, G: 0x47, B: 0x26, A: 255}

    switch mapObject.Plane {
        case data.PlaneArcanus:
            getMapColor = functional.Memoize2(func (terrainType terrain.TerrainType, explored data.FogType) color.RGBA {
                var use color.RGBA

                landColor := color.RGBA{R: 0x7, G: 0x92, B: 0x07, A: 255}

                switch terrainType {
                    case terrain.Grass: use = landColor
                    case terrain.Ocean: use = oceanColor
                    case terrain.River: use = color.RGBA{R: 0x36, G: 0x74, B: 0xb5, A: 255}
                    case terrain.Shore: use = landColor
                    case terrain.Mountain: use = color.RGBA{R: 0xbc, G: 0xd0, B: 0xe4, A: 255}
                    case terrain.Hill: use = landColor
                    case terrain.Swamp: use = landColor
                    case terrain.Forest: use = landColor
                    case terrain.Desert: use = color.RGBA{R: 0xc0, G: 0x90, B: 0x1b, A: 255}
                    case terrain.Tundra: use = color.RGBA{R: 0xd6, G: 0xd4, B: 0xc9, A: 255}
                    case terrain.Volcano: use = color.RGBA{R: 0xff, G: 0x00, B: 0x00, A: 255}
                    case terrain.Lake: use = color.RGBA{R: 0x3f, G: 0x88, B: 0xd3, A: 255}
                    case terrain.NatureNode: use = color.RGBA{R: 0, G: 255, B: 0, A: 255}
                    case terrain.SorceryNode: use = color.RGBA{R: 0, G: 0, B: 255, A: 255}
                    case terrain.ChaosNode: use = chaosNodeColor
                    default: use = color.RGBA{R: 64, G: 64, B: 64, A: 255}
                }

                if explored != data.FogTypeVisible {
                    use = util.ToRGBA(util.Darken2(use, 25))
                }

                return use
            })
        case data.PlaneMyrror:
            getMapColor = functional.Memoize2(func (terrainType terrain.TerrainType, explored data.FogType) color.RGBA {
                var use color.RGBA

                landColor := color.RGBA{R: 0x81, G: 0x54, B: 0x29, A: 255}

                switch terrainType {
                    case terrain.Grass: use = landColor
                    case terrain.Ocean: use = oceanColor
                    case terrain.River: use = color.RGBA{R: 0x3f, G: 0x88, B: 0xd3, A: 255}
                    case terrain.Shore: use = landColor
                    case terrain.Mountain: use = color.RGBA{R: 0xbc, G: 0xd0, B: 0xe4, A: 255}
                    case terrain.Hill: use = landColor
                    case terrain.Swamp: use = landColor
                    case terrain.Forest: use = landColor
                    case terrain.Desert: use = color.RGBA{R: 0xdb, G: 0xbd, B: 0x29, A: 255}
                    case terrain.Tundra: use = color.RGBA{R: 0xd6, G: 0xd4, B: 0xc9, A: 255}
                    case terrain.Volcano: use = color.RGBA{R: 0xff, G: 0x00, B: 0x00, A: 255}
                    case terrain.Lake: use = color.RGBA{R: 0x3f, G: 0x88, B: 0xd3, A: 255}
                    case terrain.NatureNode: use = color.RGBA{R: 0, G: 255, B: 0, A: 255}
                    case terrain.SorceryNode: use = color.RGBA{R: 0, G: 0, B: 255, A: 255}
                    case terrain.ChaosNode: use = chaosNodeColor
                    default: use = color.RGBA{R: 64, G: 64, B: 64, A: 255}
                }

                if explored != data.FogTypeVisible {
                    use = util.ToRGBA(util.Darken2(use, 25))
                }

                return use
            })
    }

    getCityColor := functional.Memoize2(func (cityColor color.RGBA, explored data.FogType) color.RGBA {
        use := cityColor
        if explored == data.FogTypeExplored {
            use = util.ToRGBA(util.Lighten(use, -50))
        }
        return use
    })

    for x := range screen.Bounds().Dx() {
        for y := range screen.Bounds().Dy() {
            tileX := mapObject.WrapX(scale.Unscale(x + cameraX))
            tileY := scale.Unscale(y + cameraY)

            if tileX < 0 || tileX >= mapObject.Map.Columns() || tileY < 0 || tileY >= mapObject.Map.Rows() || fog[tileX][tileY] == data.FogTypeUnexplored {
                set(x, y, black)
                continue
            }

            use := getMapColor(terrain.GetTile(mapObject.Map.Terrain[tileX][tileY]).TerrainType(), fog[tileX][tileY])

            if cityColor, ok := cityLocations[image.Pt(tileX, tileY)]; ok {
                use = getCityColor(cityColor, fog[tileX][tileY])
            }

            set(x, y, use)
        }
    }

    if crosshairs {
        cursorColorBlue := math.Sin(float64(counter) * 3 * math.Pi / 180) * 127.0 + 127.0
        if cursorColorBlue > 255 {
            cursorColorBlue = 255
        }
        cursorColor := util.PremultiplyAlpha(color.RGBA{R: 255, G: 255, B: byte(cursorColorBlue), A: 180})

        cursorRadius := int(float64(scale.Scale(5.0)) / zoom)
        x1 := scale.Scale(centerX) - cursorRadius - cameraX
        y1 := scale.Scale(centerY) - cursorRadius - cameraY
        x2 := scale.Scale(centerX) + cursorRadius - cameraX
        y2 := y1
        x3 := x1
        y3 := scale.Scale(centerY) + cursorRadius - cameraY
        x4 := x2
        y4 := y3
        points := []image.Point{
            image.Pt(x1, y1),
            image.Pt(x1+scale.Scale(1), y1),
            image.Pt(x1, y1+scale.Scale(1)),

            image.Pt(x2, y2),
            image.Pt(x2-scale.Scale(1), y2),
            image.Pt(x2, y2+scale.Scale(1)),

            image.Pt(x3, y3),
            image.Pt(x3+scale.Scale(1), y3),
            image.Pt(x3, y3-scale.Scale(1)),

            image.Pt(x4, y4),
            image.Pt(x4-scale.Scale(1), y4),
            image.Pt(x4, y4-scale.Scale(1)),
        }

        drawSquare := func(x int, y int, c color.RGBA){
            if scale.ScaleAmount == 1 {
                set(x, y, cursorColor)
            } else {
                for dx := range int(scale.ScaleAmount) {
                    for dy := range int(scale.ScaleAmount) {
                        cx := x + dx
                        cy := y + dy

                        if cx >= 0 && cy >= 0 && cx < screen.Bounds().Dx() && cy < screen.Bounds().Dy() {
                            set(cx, cy, cursorColor)
                        }
                    }
                }
            }
        }

        for _, point := range points {
            x := scale.Scale(mapObject.WrapX(scale.Unscale(point.X)))
            y := point.Y

            if x >= 0 && y >= 0 && x < screen.Bounds().Dx() && y < screen.Bounds().Dy(){
                drawSquare(x, y, cursorColor)
            }
        }
    }

    screen.WritePixels(getRenderCache(mapObject).miniMapPixels)
    /*
    red := ebiten.NewImage(screen.Bounds().Dx(), screen.Bounds().Dy())
    red.Fill(color.RGBA{R: 255, G: 0, B: 0, A: 255})
    var options ebiten.DrawImageOptions
    screen.DrawImage(red, &options)
    */
}

// iterate through screen space, compute the tile at the x/y screen coordinate, draw that tile
func DrawLayer1_y(mapObject *maplib.Map, camera cameralib.Camera, animationCounter uint64, imageCache *util.ImageCache, screen *ebiten.Image, geom ebiten.GeoM){
    tileWidth := mapObject.TileWidth()
    tileHeight := mapObject.TileHeight()

    /*
    tilesPerRow := mapObject.TilesPerRow(screen.Bounds().Dx())
    tilesPerColumn := mapObject.TilesPerColumn(screen.Bounds().Dy())
    */

    var options ebiten.DrawImageOptions

    // minX, minY, maxX, maxY := camera.GetTileBounds()

    // log.Printf("Camera x=%d minx=%d maxx=%d total=%d zoom=%d", camera.GetX(), minX, maxX, maxX - minX, camera.Zoom)

    minX := float64(screen.Bounds().Min.X)
    maxX := float64(screen.Bounds().Max.X)
    minY := float64(screen.Bounds().Min.Y)
    maxY := float64(screen.Bounds().Max.Y)

    tileHorizontalStride := scale.Scale(float64(tileWidth) * camera.GetZoom())
    tileVerticalStride := scale.Scale(float64(tileHeight) * camera.GetZoom())

    var tileToScreenGeom ebiten.GeoM

    tileToScreenGeom.Translate(-camera.GetZoomedX() * float64(tileWidth), -camera.GetZoomedY() * float64(tileHeight))
    tileToScreenGeom.Scale(camera.GetAnimatedZoom(), camera.GetAnimatedZoom())
    tileToScreenGeom.Concat(scale.ScaledGeom)
    tileToScreenGeom.Invert()

    screenToTile := func(x float64, y float64) (float64, float64) {
        tileX, tileY := tileToScreenGeom.Apply(x, y)

        tileX /= float64(tileWidth)
        tileY /= float64(tileHeight)

        // return mapObject.WrapX(int(math.Floor(tileX))), int(math.Floor(tileY))
        return tileX, tileY
    }

    var x float64

    // overdraw by one tile
    maxX += scale.Scale(float64(tileWidth))
    maxY += scale.Scale(float64(tileHeight))

    // draw all tiles first
    for x = minX; x <= maxX; x += tileHorizontalStride {
        for y := minY; y <= maxY; y += tileVerticalStride {

            if int(x) == 950 && int(tileHorizontalStride) == 50 {
                x += 0
            }

            /*
            tileX := mapObject.WrapX(x)
            tileY := y
            */
            tileXRaw, tileYRaw := screenToTile(x, y)

            tileX, tileY := mapObject.WrapX(int(math.Floor(tileXRaw))), int(math.Floor(tileYRaw))

            tileXFrac := tileXRaw - math.Floor(tileXRaw)
            tileYFrac := tileYRaw - math.Floor(tileYRaw)

            // for debugging
            // util.DrawRect(screen, image.Rect(x * tileWidth, y * tileHeight, (x + 1) * tileWidth, (y + 1) * tileHeight), color.RGBA{R: 255, G: 0, B: 0, A: 255})

            if tileX < 0 || tileX >= mapObject.Map.Columns() || tileY < 0 || tileY >= mapObject.Map.Rows() {
                continue
            }


            tileImage, err := GetTileImage(mapObject, tileX, tileY, animationCounter)
            if err == nil {
                options.GeoM.Reset()
                // options.GeoM = geom
                // options.GeoM.Reset()
                // options.GeoM.Concat(geom)
                options.GeoM.Scale(camera.GetZoom(), camera.GetZoom())
                options.GeoM.Translate(scale.Unscale(x) - tileXFrac * float64(tileWidth), scale.Unscale(y) - tileYFrac * float64(tileHeight))

                /*
                if (x == minX && y == minY) || (x == maxX && y == maxY) {
                    a, b := options.GeoM.Apply(0, 0)
                    log.Printf("Tile draw at minX,minY geoM result: %f, %f", a, b)
                }
                */

                scale.DrawScaled(screen, tileImage, &options)

                for _, extraKind := range maplib.ExtraDrawOrder {
                    extra, ok := mapObject.ExtraMap[image.Pt(tileX, tileY)][extraKind]
                    if ok {
                        DrawExtraLayer1(extra, screen, imageCache, &options, animationCounter, tileWidth, tileHeight)
                    }
                }
            } else {
                log.Printf("Unable to render tile at %d, %d: %v", tileX, tileY, err)
            }
        }
    }

    // log.Printf("Last draw x=%v maxX=%v stride=%v", x, maxX, tileHorizontalStride)

    vector.FillRect(screen, scale.Scale[float32](data.ScreenWidth / 2 - 5), scale.Scale[float32](data.ScreenHeight / 2 - 5), scale.Scale[float32](10), scale.Scale[float32](10), color.RGBA{R: 255, G: 0, B: 0, A: 255}, false)

}

// draw base map tiles, in general stuff that should go under cities/units
func DrawLayer1(mapObject *maplib.Map, camera cameralib.Camera, animationCounter uint64, imageCache *util.ImageCache, screen *ebiten.Image, geom ebiten.GeoM){
    tileWidth := mapObject.TileWidth()
    tileHeight := mapObject.TileHeight()

    /*
    tilesPerRow := mapObject.TilesPerRow(screen.Bounds().Dx())
    tilesPerColumn := mapObject.TilesPerColumn(screen.Bounds().Dy())
    */

    var options ebiten.DrawImageOptions

    minX, minY, maxX, maxY := camera.GetTileBounds()

    // log.Printf("Camera x=%d minx=%d maxx=%d total=%d zoom=%d", camera.GetX(), minX, maxX, maxX - minX, camera.Zoom)

    // draw all tiles first
    for x := minX; x <= maxX; x++ {
        for y := minY; y <= maxY; y++ {
            tileX := mapObject.WrapX(x)
            tileY := y

            // for debugging
            // util.DrawRect(screen, image.Rect(x * tileWidth, y * tileHeight, (x + 1) * tileWidth, (y + 1) * tileHeight), color.RGBA{R: 255, G: 0, B: 0, A: 255})

            if tileX < 0 || tileX >= mapObject.Map.Columns() || tileY < 0 || tileY >= mapObject.Map.Rows() {
                continue
            }

            tileImage, err := GetTileImage(mapObject, tileX, tileY, animationCounter)
            if err == nil {
                options.GeoM.Reset()
                // options.GeoM = geom
                // options.GeoM.Reset()
                options.GeoM.Translate(float64(x * tileWidth), float64(y * tileHeight))
                options.GeoM.Concat(geom)

                /*
                if (x == minX && y == minY) || (x == maxX && y == maxY) {
                    a, b := options.GeoM.Apply(0, 0)
                    log.Printf("Tile draw at minX,minY geoM result: %f, %f", a, b)
                }
                */

                scale.DrawScaled(screen, tileImage, &options)

                for _, extraKind := range maplib.ExtraDrawOrder {
                    extra, ok := mapObject.ExtraMap[image.Pt(tileX, tileY)][extraKind]
                    if ok {
                        DrawExtraLayer1(extra, screen, imageCache, &options, animationCounter, tileWidth, tileHeight)
                    }
                }
            } else {
                log.Printf("Unable to render tile at %d, %d: %v", tileX, tileY, err)
            }
        }
    }

    // vector.FillRect(screen, scale.Scale[float32](data.ScreenWidth / 2 - 5), scale.Scale[float32](data.ScreenHeight / 2 - 5), scale.Scale[float32](10), scale.Scale[float32](10), color.RGBA{R: 255, G: 0, B: 0, A: 255}, false)
}

func DrawLayer2(mapObject *maplib.Map, camera cameralib.Camera, animationCounter uint64, imageCache *util.ImageCache, screen *ebiten.Image, geom ebiten.GeoM){
    DrawLayer2Internal(mapObject, camera, animationCounter, imageCache, screen, geom, Renderer{Map: mapObject})
}

// give the extra nodes a chance to draw on top of cities/units, but still under the fog
func DrawLayer2Internal(mapObject *maplib.Map, camera cameralib.Camera, animationCounter uint64, imageCache *util.ImageCache, screen *ebiten.Image, geom ebiten.GeoM, drawer DrawTile){
    tileWidth := mapObject.TileWidth()
    tileHeight := mapObject.TileHeight()

    var options ebiten.DrawImageOptions

    minX, minY, maxX, maxY := camera.GetTileBounds()

    // draw all tiles first
    for x := minX; x <= maxX; x++ {
        for y := minY; y <= maxY; y++ {
            tileX := mapObject.WrapX(x)
            tileY := y

            // for debugging
            // util.DrawRect(screen, image.Rect(x * tileWidth, y * tileHeight, (x + 1) * tileWidth, (y + 1) * tileHeight), color.RGBA{R: 255, G: 0, B: 0, A: 255})

            if tileX < 0 || tileX >= mapObject.Map.Columns() || tileY < 0 || tileY >= mapObject.Map.Rows() {
                continue
            }

            getOptions := func() *ebiten.DrawImageOptions {
                options.GeoM.Reset()
                options.GeoM.Translate(float64(x * tileWidth), float64(y * tileHeight))
                options.GeoM.Concat(geom)
                return &options
            }

            drawer.DrawTileLayer2(screen, imageCache, getOptions, animationCounter, tileX, tileY)
        }
    }
}

// draw a single tile
func DrawTileLayer2(mapObject *maplib.Map, screen *ebiten.Image, imageCache *util.ImageCache, getOptions func() *ebiten.DrawImageOptions, animationCounter uint64, tileX int, tileY int){
    use, ok := mapObject.ExtraMap[image.Pt(tileX, tileY)]
    if !ok || len(use) == 0 {
        return
    }

    options := getOptions()
    tileWidth := mapObject.TileWidth()
    tileHeight := mapObject.TileHeight()

    for _, extraKind := range maplib.ExtraDrawOrder {
        extra, ok := use[extraKind]
        if ok {
            DrawExtraLayer2(extra, screen, imageCache, options, animationCounter, tileWidth, tileHeight)
        }
    }
}

type DrawTile interface {
    DrawTileLayer2(screen *ebiten.Image, imageCache *util.ImageCache, getOptions func() *ebiten.DrawImageOptions, animationCounter uint64, tileX int, tileY int)
}
