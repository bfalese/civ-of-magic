package combat

import (
    "testing"

    "github.com/bfalese/civ-of-magic/lib/set"
)

func TestWallPieces(test *testing.T) {
    var empty Tile
    if len(empty.WallPieces(3, 4)) != 0 {
        test.Errorf("a tile without walls has no pieces")
    }

    // a north-west corner of the city wall is one image
    corner := Tile{Wall: set.NewSet(WallKindNorth, WallKindWest)}
    pieces := corner.WallPieces(3, 4)
    if len(pieces) != 1 || pieces[0].Index != 0 || pieces[0].Kind != WallPieceStone || pieces[0].Animated {
        test.Errorf("corner: %+v", pieces)
    }

    corner.WallDestroyed = true
    if !corner.WallPieces(3, 4)[0].Destroyed {
        test.Errorf("a destroyed wall's pieces are marked")
    }

    // fire and darkness on the south side: darkness (order 1) before fire (order 2), variants by
    // position
    both := Tile{Fire: set.NewSet(FireSideSouth), Darkness: set.NewSet(DarknessSideSouth)}
    pieces = both.WallPieces(1, 1)
    if len(pieces) != 2 || pieces[0].Kind != WallPieceDarkness || pieces[1].Kind != WallPieceFire || !pieces[0].Animated {
        test.Fatalf("fire and darkness: %+v", pieces)
    }
    if pieces[1].Index != []int{43, 44, 48}[2 % 3] || pieces[0].Index != []int{57, 58, 62}[2 % 3] {
        test.Errorf("variants at (1, 1): %+v", pieces)
    }
}
