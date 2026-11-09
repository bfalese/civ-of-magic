package combat

import (
    "math"

    "github.com/bfalese/civ-of-magic/core/units"
)

// the way a unit on tile (x1, y1) faces to look at tile (x2, y2), as seen on the isometric
// battlefield: the grid is turned 45 degrees on screen, so grid +x is up-right and +y down-right
func FaceTowards(x1 int, y1 int, x2 int, y2 int) units.Facing {
    if x1 == x2 && y1 == y2 {
        return units.FacingDownRight
    }
    angle := math.Atan2(float64(y2 - y1), float64(x2 - x1))
    // on screen, counterclockwise from the right; y grows downwards
    screenAngle := -(angle - math.Pi/4)
    octant := int(math.Round(screenAngle / (math.Pi / 4)))
    octant = ((octant % 8) + 8) % 8
    facings := []units.Facing{
        units.FacingRight, units.FacingUpRight, units.FacingUp, units.FacingUpLeft,
        units.FacingLeft, units.FacingDownLeft, units.FacingDown, units.FacingDownRight,
    }
    return facings[octant]
}
