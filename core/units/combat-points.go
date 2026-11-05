package units

import (
    "image"
    "math"
)

// positions of a unit's figures on a combat tile, relative to the tile center.
// Note: for more than 8 figures the grid can have fewer points than figures, and the
// headless combat runner (combat/run.go) uses the number of points as the number of hits.
// hard coding the points is what the real master of magic does
// see Unit_Figure_Position() in UnitView.C
// https://github.com/jbalcomb/ReMoM/blob/8642bb8c46433cc31c058759b28f297947b3b501/src/UnitView.C#L2685
func CombatPoints(count int) []image.Point {
    switch count {
        case 0: return nil
        case 1: return []image.Point{image.Pt(0, 0)}
        case 2:
            // FIXME: this was just copied from case 8
            return []image.Point{
                image.Pt(2, -4),
                image.Pt(6, -2),
            }
        case 3:
            // FIXME: this was just copied from case 8
            return []image.Point{
                image.Pt(2, -4),
                image.Pt(6, -2),
                image.Pt(-1, 0),
            }
        case 4:
            // FIXME: this was just copied from case 8
            return []image.Point{
                image.Pt(1, -4),
                image.Pt(7, -2),
                image.Pt(-1, 3),
                image.Pt(-8, 0),
            }
        case 5:
            // FIXME: this was just copied from case 8
            return []image.Point{
                image.Pt(2, -4),
                image.Pt(6, -2),
                image.Pt(-1, 0),
                image.Pt(-8, 0),
                image.Pt(10, 0),
            }
        case 6:
            // FIXME: this was just copied from case 8
            return []image.Point{
                image.Pt(2, -4),
                image.Pt(6, -2),
                image.Pt(-1, 0),
                image.Pt(-8, 0),
                image.Pt(10, 0),
                image.Pt(3, 4),
            }
        case 7:
            // FIXME: this was just copied from case 8
            return []image.Point{
                image.Pt(2, -4),
                image.Pt(6, -2),
                image.Pt(-1, 0),
                image.Pt(-8, 0),
                image.Pt(10, 0),
                image.Pt(3, 1),
                image.Pt(-4, 3),
            }
        case 8:
            // fairly accurate
            return []image.Point{
                image.Pt(2, -4),
                image.Pt(6, -2),
                image.Pt(-1, 0),
                image.Pt(-8, 0),
                image.Pt(10, 0),
                image.Pt(3, 1),
                image.Pt(-4, 3),
                image.Pt(1, 5),
            }
    }

    rows := math.Round(math.Sqrt(float64(count)))
    columns := math.Round(float64(count) / rows)

    var points []image.Point
    total := 0

    startRow := -int(rows) * 4 / 2
    rowGap := 4
    startColumn := -int(columns) * 4 / 2
    columnGap := 4

    for row := range int(rows) {
        for column := range int(columns) {
            if total >= count {
                break
            }

            points = append(points, image.Pt(startColumn + column * columnGap, startRow + row * rowGap))
            total += 1
        }
    }

    return points
}
