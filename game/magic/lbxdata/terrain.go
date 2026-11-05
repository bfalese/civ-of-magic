package lbxdata

import (
    "bytes"
    "image"

    "github.com/bfalese/civ-of-magic/core/terrain"
    "github.com/bfalese/civ-of-magic/lib/lbx"
)

// pass in terrain.lbx
func ReadTerrainData(lbxFile *lbx.LbxFile) (*terrain.TerrainData, error) {
    data, err := lbxFile.RawData(1)
    if err != nil {
        return nil, err
    }

    images, err := lbxFile.ReadTerrainImages(0)
    if err != nil {
        return nil, err
    }

    // TODO: lbxFile entry 2 is the terrain palette for the minimap

    reader := bytes.NewReader(data)

    // larger than we need, but should be fine
    tiles := make([]terrain.TerrainTile, 0, len(images))

    tileIndex := 0

    for reader.Len() > 0 {
        var animation = false
        value1, err := reader.ReadByte()
        if err != nil {
            return nil, err
        }

        value2, err := reader.ReadByte()
        if err != nil {
            return nil, err
        }

        if value1 & 0x80 != 0 {
            value1 &= 0x7f
            animation = true
            // value2 should be incremented based on an animation counter
        }

        // the index into the terrain image
        var index int

        if value1 == 0 {
            index = int(value2) - 2
        } else {
            // this formula comes from
            // terrain_lbx_000_offset = (terrain_001_0 * 16384) + (terrain_001_1 * 384) - 0xC0 - 384;
            // this only works if value1 is a multiple of 3
            // 3 -> 126, 6 -> 254, 9 -> 382
            index = int(value1) * 16384 / 384 + int(value2) - 2
        }

        var tileImages []image.Image
        if animation {
            // animation tiles are always 4 images
            for i := 0; i < 4; i++ {
                tileImages = append(tileImages, images[index + i])
            }
        } else {
            tileImages = append(tileImages, images[index])
        }

        tiles = append(tiles, terrain.TerrainTile{
            ImageIndex: index,
            TileIndex: tileIndex,
            Tile: terrain.GetTile(tileIndex),
            Images: tileImages,
        })

        tileIndex += 1
    }

    return terrain.MakeTerrainData(images, tiles), nil
}
