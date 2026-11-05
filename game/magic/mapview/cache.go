package mapview

import (
    "github.com/bfalese/civ-of-magic/core/data"
    "github.com/bfalese/civ-of-magic/game/magic/maplib"
    cameralib "github.com/bfalese/civ-of-magic/game/magic/camera"
    "github.com/bfalese/civ-of-magic/game/magic/util"

    "github.com/hajimehoshi/ebiten/v2"
)

// per-map rendering state, stored in maplib.Map.RenderCache so the model stays free of Ebitengine
type renderCache struct {
    tiles map[int]*ebiten.Image
    miniMapPixels []byte
}

func getRenderCache(mapObject *maplib.Map) *renderCache {
    cache, ok := mapObject.RenderCache.(*renderCache)
    if !ok {
        cache = &renderCache{
            tiles: make(map[int]*ebiten.Image),
        }
        mapObject.RenderCache = cache
    }
    return cache
}

// Renderer wraps a map with drawing methods, so it can be used where an interface expects a
// map that draws itself (e.g. the overworld). Embed it to override individual draw methods.
type Renderer struct {
    *maplib.Map
}

func (renderer Renderer) DrawLayer1(camera cameralib.Camera, animationCounter uint64, imageCache *util.ImageCache, screen *ebiten.Image, geom ebiten.GeoM) {
    DrawLayer1(renderer.Map, camera, animationCounter, imageCache, screen, geom)
}

func (renderer Renderer) DrawLayer2(camera cameralib.Camera, animationCounter uint64, imageCache *util.ImageCache, screen *ebiten.Image, geom ebiten.GeoM) {
    DrawLayer2(renderer.Map, camera, animationCounter, imageCache, screen, geom)
}

func (renderer Renderer) DrawMinimap(screen *ebiten.Image, cities []maplib.MiniMapCity, centerX int, centerY int, zoom float64, fog data.FogMap, counter uint64, crosshairs bool) {
    DrawMinimap(renderer.Map, screen, cities, centerX, centerY, zoom, fog, counter, crosshairs)
}

func (renderer Renderer) DrawTileLayer2(screen *ebiten.Image, imageCache *util.ImageCache, getOptions func() *ebiten.DrawImageOptions, animationCounter uint64, tileX int, tileY int) {
    DrawTileLayer2(renderer.Map, screen, imageCache, getOptions, animationCounter, tileX, tileY)
}
