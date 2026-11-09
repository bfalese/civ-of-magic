package combat

import (
    "slices"
)

// One image of a tile's walls: a piece of city wall, wall of fire or wall of darkness, as the
// Ebitengine battle screen (combatview DrawWall) picks them. Index is the image in citywall.lbx;
// the image's top left is at the tile centre + (DX, DY) - (h/2, h/2), h the image's height.
type WallPieceKind int
const (
    WallPieceStone WallPieceKind = iota
    WallPieceFire
    WallPieceDarkness
)

type WallPiece struct {
    Kind WallPieceKind
    Index int
    DX int
    DY int
    // fire and darkness animate (loop their frames); a city wall shows frame 0, or 1 when
    // Destroyed
    Animated bool
    Destroyed bool
    // pieces of one tile are drawn in this order
    Order int
}

// The pieces of a tile's walls, in drawing order; empty when it has none. The variant (one of a
// few images for the same side) depends on the tile's position, so a wall doesn't repeat one image.
func (tile *Tile) WallPieces(x int, y int) []WallPiece {
    if tile.Fire == nil && tile.Darkness == nil && tile.Wall == nil {
        return nil
    }

    choose := func(choices []int) int {
        return choices[(x + y) % len(choices)]
    }

    var pieces []WallPiece
    animated := func(order int, index int, dx int, dy int) {
        kind := WallPieceFire
        if index >= 50 {
            kind = WallPieceDarkness
        }
        pieces = append(pieces, WallPiece{Kind: kind, Index: index, DX: dx, DY: dy, Animated: true, Order: order})
    }

    fireWest := []int{37, 38, 39}
    fireNorth := []int{40, 41, 42}
    fireSouth := []int{43, 44, 48}
    fireEast := []int{46, 47, 49}

    if fire := tile.Fire; fire != nil {
        drewNorth, drewSouth, drewEast, drewWest := false, false, false, false
        if fire.Contains(FireSideNorth) && fire.Contains(FireSideWest) {
            animated(0, 36, -1, -8)
            drewNorth, drewWest = true, true
        }
        if fire.Contains(FireSideSouth) && fire.Contains(FireSideEast) {
            animated(2, 45, -2, -3)
            drewSouth, drewEast = true, true
        }
        if !drewSouth && fire.Contains(FireSideSouth) {
            animated(2, choose(fireSouth), -4, -4)
        }
        if !drewWest && fire.Contains(FireSideWest) {
            animated(0, choose(fireWest), -3, -6)
        }
        if !drewNorth && fire.Contains(FireSideNorth) {
            animated(0, choose(fireNorth), 2, -6)
        }
        if !drewEast && fire.Contains(FireSideEast) {
            animated(2, choose(fireEast), 2, -4)
        }
    }

    darknessWest := []int{51, 52, 53}
    darknessNorth := []int{54, 55, 56}
    darknessSouth := []int{57, 58, 62}
    darknessEast := []int{60, 61, 63}

    if darkness := tile.Darkness; darkness != nil {
        drewNorth, drewSouth, drewEast, drewWest := false, false, false, false
        if darkness.Contains(DarknessSideNorth) && darkness.Contains(DarknessSideWest) {
            animated(1, 50, -1, -8)
            drewNorth, drewWest = true, true
        }
        if darkness.Contains(DarknessSideSouth) && darkness.Contains(DarknessSideEast) {
            animated(1, 59, -2, -3)
            drewSouth, drewEast = true, true
        }
        if !drewSouth && darkness.Contains(DarknessSideSouth) {
            animated(1, choose(darknessSouth), -4, -4)
        }
        if !drewWest && darkness.Contains(DarknessSideWest) {
            animated(1, choose(darknessWest), -3, -6)
        }
        if !drewNorth && darkness.Contains(DarknessSideNorth) {
            animated(1, choose(darknessNorth), 2, -6)
        }
        if !drewEast && darkness.Contains(DarknessSideEast) {
            animated(1, choose(darknessEast), 2, -4)
        }
    }

    if wall := tile.Wall; wall != nil {
        // the normal stone wall (citywall.lbx also has a dark one at 12 and an ivy one at 24)
        wallBase := 0
        west := []int{1, 2}
        north := []int{4, 5}
        south := []int{7, 8}
        east := []int{10}
        gate := 11
        northWest := 0
        southWest := 3
        northEast := 6
        southEast := 9

        stone := func(order int, index int, dx int, dy int) {
            pieces = append(pieces, WallPiece{Kind: WallPieceStone, Index: wallBase + index, DX: dx, DY: dy, Destroyed: tile.WallDestroyed, Order: order})
        }

        drewNorth, drewSouth, drewEast, drewWest := false, false, false, false
        if wall.Contains(WallKindNorth) && wall.Contains(WallKindWest) {
            stone(2, northWest, -1, -8)
            drewNorth, drewWest = true, true
        }
        if wall.Contains(WallKindSouth) && wall.Contains(WallKindEast) {
            stone(0, southEast, -2, -3)
            drewSouth, drewEast = true, true
        }
        if wall.Contains(WallKindSouth) && wall.Contains(WallKindWest) {
            stone(2, southWest, -2, -3)
            drewSouth, drewWest = true, true
            // fire and darkness on the outside of a corner go over it
            if tile.Darkness != nil && tile.Darkness.Contains(DarknessSideSouth) {
                animated(3, choose(darknessSouth), -4, -4)
            }
            if tile.Fire != nil && tile.Fire.Contains(FireSideSouth) {
                animated(4, choose(fireSouth), -4, -4)
            }
        }
        if wall.Contains(WallKindNorth) && wall.Contains(WallKindEast) {
            stone(2, northEast, -2, -3)
            drewNorth, drewEast = true, true
            if tile.Darkness != nil && tile.Darkness.Contains(DarknessSideEast) {
                animated(3, choose(darknessEast), 2, -4)
            }
            if tile.Fire != nil && tile.Fire.Contains(FireSideEast) {
                animated(4, choose(fireEast), 2, -4)
            }
        }
        if !drewSouth && wall.Contains(WallKindSouth) {
            stone(0, choose(south), -4, -4)
        }
        if !drewWest && wall.Contains(WallKindWest) {
            stone(2, choose(west), -3, -6)
        }
        if !drewNorth && wall.Contains(WallKindNorth) {
            stone(2, choose(north), 2, -6)
        }
        if !drewEast && wall.Contains(WallKindEast) {
            stone(0, choose(east), 2, -4)
        }
        if wall.Contains(WallKindGate) {
            stone(0, gate, -2, -4)
        }
    }

    slices.SortStableFunc(pieces, func(a WallPiece, b WallPiece) int { return a.Order - b.Order })
    return pieces
}
