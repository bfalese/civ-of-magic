package lbxdata

import (
    "bytes"
    "fmt"

    "github.com/bfalese/civ-of-magic/lib/lbx"
)

func ReadCityNames(cache *lbx.LbxCache) ([]string, error) {
    data, err := cache.GetLbxFile("cityname.lbx")
    if err != nil {
        return nil, fmt.Errorf("unable to read cityname.lbx: %v", err)
    }

    reader, err := data.GetReader(0)
    if err != nil {
        return nil, fmt.Errorf("unable to read entry 0 in cityname.lbx: %v", err)
    }

    entrySize, err := lbx.ReadUint16(reader)
    if err != nil {
        return nil, fmt.Errorf("read error: %v", err)
    }

    numEntries, err := lbx.ReadUint16(reader)
    if err != nil {
        return nil, fmt.Errorf("read error: %v", err)
    }

    var out []string

    for i := 0; i < int(numEntries); i++ {
        entry := make([]byte, entrySize)
        n, err := reader.Read(entry)
        if err != nil || n != int(entrySize) {
            return nil, fmt.Errorf("unable to read entry %v: %v", i, err)
        }
        out = append(out, string(bytes.Trim(entry, "\x00")))
    }

    return out, nil
}
