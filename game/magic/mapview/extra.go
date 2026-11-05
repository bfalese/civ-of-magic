package mapview

import (
    "log"
    "math"
    "image"

    "github.com/bfalese/civ-of-magic/game/magic/maplib"
    "github.com/bfalese/civ-of-magic/game/magic/util"
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/game/magic/scale"
    "github.com/bfalese/civ-of-magic/game/magic/shaders"

    "github.com/hajimehoshi/ebiten/v2"
)

// draws an extra tile feature (road, node, lair, ...) for layer 1
func DrawExtraLayer1(extra maplib.ExtraTile, screen *ebiten.Image, imageCache *util.ImageCache, options *ebiten.DrawImageOptions, counter uint64, tileWidth int, tileHeight int){
    switch extra := extra.(type) {
        case *maplib.ExtraRoad: drawExtraRoadLayer1(extra, screen, imageCache, options, counter, tileWidth, tileHeight)
        case *maplib.ExtraBonus: drawExtraBonusLayer1(extra, screen, imageCache, options, counter, tileWidth, tileHeight)
        case *maplib.ExtraEncounter: drawExtraEncounterLayer1(extra, screen, imageCache, options, counter, tileWidth, tileHeight)
        case *maplib.ExtraOpenTower: drawExtraOpenTowerLayer1(extra, screen, imageCache, options, counter, tileWidth, tileHeight)
        case *maplib.ExtraMagicNode: drawExtraMagicNodeLayer1(extra, screen, imageCache, options, counter, tileWidth, tileHeight)
        case *maplib.ExtraVolcano: drawExtraVolcanoLayer1(extra, screen, imageCache, options, counter, tileWidth, tileHeight)
        case *maplib.ExtraCorruption: drawExtraCorruptionLayer1(extra, screen, imageCache, options, counter, tileWidth, tileHeight)
    }
}

// draws an extra tile feature (road, node, lair, ...) for layer 2
func DrawExtraLayer2(extra maplib.ExtraTile, screen *ebiten.Image, imageCache *util.ImageCache, options *ebiten.DrawImageOptions, counter uint64, tileWidth int, tileHeight int){
    switch extra := extra.(type) {
        case *maplib.ExtraRoad: drawExtraRoadLayer2(extra, screen, imageCache, options, counter, tileWidth, tileHeight)
        case *maplib.ExtraBonus: drawExtraBonusLayer2(extra, screen, imageCache, options, counter, tileWidth, tileHeight)
        case *maplib.ExtraEncounter: drawExtraEncounterLayer2(extra, screen, imageCache, options, counter, tileWidth, tileHeight)
        case *maplib.ExtraOpenTower: drawExtraOpenTowerLayer2(extra, screen, imageCache, options, counter, tileWidth, tileHeight)
        case *maplib.ExtraMagicNode: drawExtraMagicNodeLayer2(extra, screen, imageCache, options, counter, tileWidth, tileHeight)
        case *maplib.ExtraVolcano: drawExtraVolcanoLayer2(extra, screen, imageCache, options, counter, tileWidth, tileHeight)
        case *maplib.ExtraCorruption: drawExtraCorruptionLayer2(extra, screen, imageCache, options, counter, tileWidth, tileHeight)
    }
}

func drawExtraRoadLayer1(road *maplib.ExtraRoad, screen *ebiten.Image, imageCache *util.ImageCache, options *ebiten.DrawImageOptions, counter uint64, tileWidth int, tileHeight int){
    neighbors := road.Map.GetRoadNeighbors(road.X, road.Y)

    connected := false

    baseIndex := 45
    if road.Enchanted {
        baseIndex = 54
    }

    directionToIndex := map[maplib.Direction]int{
        maplib.DirectionNorth: 1,
        maplib.DirectionNorthEast: 2,
        maplib.DirectionEast: 3,
        maplib.DirectionSouthEast: 4,
        maplib.DirectionSouth: 5,
        maplib.DirectionSouthWest: 6,
        maplib.DirectionWest: 7,
        maplib.DirectionNorthWest: 8,
    }

    for direction, has := range neighbors {
        if has {
            index := directionToIndex[direction]
            pics, err := imageCache.GetImages("mapback.lbx", baseIndex + index)
            if err == nil {
                pic := pics[counter % uint64(len(pics))]
                scale.DrawScaled(screen, pic, options)
            }

            connected = true
        }
    }

    if !connected {
        pics, err := imageCache.GetImages("mapback.lbx", baseIndex)
        if err == nil {
            pic := pics[counter % uint64(len(pics))]
            scale.DrawScaled(screen, pic, options)
        }
    }
}

func drawExtraRoadLayer2(road *maplib.ExtraRoad, screen *ebiten.Image, imageCache *util.ImageCache, options *ebiten.DrawImageOptions, counter uint64, tileWidth int, tileHeight int){
}

func drawExtraBonusLayer1(bonus *maplib.ExtraBonus, screen *ebiten.Image, imageCache *util.ImageCache, options *ebiten.DrawImageOptions, counter uint64, tileWidth int, tileHeight int){
    index := bonus.Bonus.LbxIndex()

    if index == -1 {
        return
    }

    pic, err := imageCache.GetImage("mapback.lbx", index, 0)
    if err == nil {
        scale.DrawScaled(screen, pic, options)
    }
}

func drawExtraBonusLayer2(bonus *maplib.ExtraBonus, screen *ebiten.Image, imageCache *util.ImageCache, options *ebiten.DrawImageOptions, counter uint64, tileWidth int, tileHeight int){
    // nothing
}

func drawExtraEncounterLayer1(extra *maplib.ExtraEncounter, screen *ebiten.Image, imageCache *util.ImageCache, options *ebiten.DrawImageOptions, counter uint64, tileWidth int, tileHeight int){
    index := extra.Type.LbxIndex()

    if index == -1 {
        return
    }

    pic, err := imageCache.GetImage("mapback.lbx", index, 0)
    if err == nil {
        scale.DrawScaled(screen, pic, options)
    }
}

func drawExtraEncounterLayer2(extra *maplib.ExtraEncounter, screen *ebiten.Image, imageCache *util.ImageCache, options *ebiten.DrawImageOptions, counter uint64, tileWidth int, tileHeight int){
    // nuthin'
}

func drawExtraOpenTowerLayer1(extra *maplib.ExtraOpenTower, screen *ebiten.Image, imageCache *util.ImageCache, options *ebiten.DrawImageOptions, counter uint64, tileWidth int, tileHeight int){
    index := 70

    pic, err := imageCache.GetImage("mapback.lbx", index, 0)
    if err == nil {
        scale.DrawScaled(screen, pic, options)
    }
}

func drawExtraOpenTowerLayer2(extra *maplib.ExtraOpenTower, screen *ebiten.Image, imageCache *util.ImageCache, options *ebiten.DrawImageOptions, counter uint64, tileWidth int, tileHeight int){
}

func drawExtraMagicNodeLayer1(node *maplib.ExtraMagicNode, screen *ebiten.Image, imageCache *util.ImageCache, options *ebiten.DrawImageOptions, counter uint64, tileWidth int, tileHeight int){
}

func drawExtraMagicNodeLayer2(node *maplib.ExtraMagicNode, screen *ebiten.Image, imageCache *util.ImageCache, options *ebiten.DrawImageOptions, counter uint64, tileWidth int, tileHeight int){
    // if the node is melded then show the zone of influence with the sparkly images

    if node.MeldingWizard != nil {
        index := 63
        switch node.MeldingWizard.GetBanner() {
            case data.BannerBlue: index = 63
            case data.BannerGreen: index = 64
            case data.BannerPurple: index = 65
            case data.BannerRed: index = 66
            case data.BannerYellow: index = 67
        }

        sparkle, err := imageCache.GetImages("mapback.lbx", index)
        if err != nil {
            return
        }

        for i, point := range node.Zone {
            use := sparkle[(counter + uint64(i)) % uint64(len(sparkle))]

            options2 := *options
            options2.GeoM.Reset()
            options2.GeoM.Translate(float64(point.X * tileWidth), float64(point.Y * tileHeight))
            options2.GeoM.Concat(options.GeoM)
            scale.DrawScaled(screen, use, &options2)
        }
    }

    if node.Warped && node.MeldingWizard != nil {
        shader, err := imageCache.GetShader(shaders.ShaderWarp)
        if err != nil {
            log.Printf("error getting shader %v", err)
            return
        }

        // FIXME get the mask to work, but scale it based on what the options.GeoM has
        /*
        mask, err := imageCache.GetImage("mapback.lbx", 93, 0)
        if err != nil {
            return
        }
        */

        x1, y1 := options.GeoM.Apply(0, 0)
        x2, y2 := options.GeoM.Apply(float64(tileWidth), float64(tileHeight))

        // log.Printf("x1 %v y1 %v x2 %v y2 %v", x1, y1, x2, y2)

        if x1 < 0 || y1 < 0 || x2 <= x1 || y2 <= y1 {
            return
        }
        rect := image.Rect(int(x1), int(y1), int(x2), int(y2))
        image := screen.SubImage(scale.ScaleRect(rect))
        if image.Bounds().Dx() == 0 || image.Bounds().Dy() == 0 {
            return
        }

        // the mask needs to be the same size as the subimage. This is a hack for now
        /*
        mask := ebiten.NewImage(image.Bounds().Dx(), image.Bounds().Dy())
        mask.Fill(color.RGBA{0, 0, 0, 255})
        */

        sourceImage := ebiten.NewImageFromImage(image)

        // log.Printf("warp at %v, %v bounds image=%v source=%v", point.X, point.Y, image.Bounds(), sourceImage.Bounds())

        var options2 ebiten.DrawRectShaderOptions
        options2.GeoM.Translate(scale.Scale(x1), scale.Scale(y1))

        options2.Images[0] = sourceImage
        // options2.Images[1] = mask
        options2.Uniforms = make(map[string]interface{})
        options2.Uniforms["Time"] = float32(math.Abs(float64(counter/5)))
        screen.DrawRectShader(image.Bounds().Dx(), image.Bounds().Dy(), shader, &options2)
    }
}

func drawExtraVolcanoLayer1(node *maplib.ExtraVolcano, screen *ebiten.Image, imageCache *util.ImageCache, options *ebiten.DrawImageOptions, counter uint64, tileWidth int, tileHeight int){
}

func drawExtraVolcanoLayer2(node *maplib.ExtraVolcano, screen *ebiten.Image, imageCache *util.ImageCache, options *ebiten.DrawImageOptions, counter uint64, tileWidth int, tileHeight int){
}

func drawExtraCorruptionLayer1(node *maplib.ExtraCorruption, screen *ebiten.Image, imageCache *util.ImageCache, options *ebiten.DrawImageOptions, counter uint64, tileWidth int, tileHeight int){
    pic, err := imageCache.GetImage("mapback.lbx", 77, 0)
    if err == nil {
        scale.DrawScaled(screen, pic, options)
    }
}

func drawExtraCorruptionLayer2(node *maplib.ExtraCorruption, screen *ebiten.Image, imageCache *util.ImageCache, options *ebiten.DrawImageOptions, counter uint64, tileWidth int, tileHeight int){
}
