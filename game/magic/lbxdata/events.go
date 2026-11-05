package lbxdata

import (
    "fmt"

    gamecore "github.com/bfalese/civ-of-magic/core/game"
    "github.com/bfalese/civ-of-magic/lib/lbx"
)

func ReadEventData(cache *lbx.LbxCache) (*gamecore.EventData, error) {
    lbxData, err := cache.GetLbxFile("eventmsg.lbx")
    if err != nil {
        return nil, err
    }

    reader, err := lbxData.GetReader(0)
    if err != nil {
        return nil, err
    }

    count, err := lbx.ReadUint16(reader)
    if err != nil {
        return nil, err
    }

    size, err := lbx.ReadUint16(reader)
    if err != nil {
        return nil, err
    }

    events := make([]string, count)

    for i := 0; i < int(count); i++ {
        data := make([]byte, size)
        n, err := reader.Read(data)
        if err != nil {
            return nil, err
        }
        if n != int(size) {
            return nil, fmt.Errorf("did not read all %v bytes", size)
        }

        events[i] = string(parseString(data))
    }

    return &gamecore.EventData{Events: events}, nil
}

func parseString(s []byte) []byte {
    // strings are null terminated
    for i, c := range s {
        if c == 0 {
            return s[:i]
        }
    }
    return s
}
