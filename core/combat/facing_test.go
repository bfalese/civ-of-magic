package combat

import (
    "testing"

    "github.com/bfalese/civ-of-magic/core/units"
)

func TestFaceTowards(test *testing.T) {
    cases := []struct {
        dx, dy int
        facing units.Facing
    }{
        {1, 0, units.FacingUpRight},
        {0, 1, units.FacingDownRight},
        {1, 1, units.FacingRight},
        {-1, -1, units.FacingLeft},
        {1, -1, units.FacingUp},
        {-1, 1, units.FacingDown},
        {-1, 0, units.FacingDownLeft},
        {0, -1, units.FacingUpLeft},
        {5, 1, units.FacingUpRight},
    }
    for _, check := range cases {
        if got := FaceTowards(3, 3, 3 + check.dx, 3 + check.dy); got != check.facing {
            test.Errorf("(%v, %v): got %v, want %v", check.dx, check.dy, got, check.facing)
        }
    }
}
