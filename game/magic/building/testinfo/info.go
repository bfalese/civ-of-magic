package main

import (
    "log"

    "github.com/bfalese/civ-of-magic/game/magic/lbxdata"

    "github.com/bfalese/civ-of-magic/lib/lbx"
)

func main(){
    cache := lbx.AutoCache()
    infos, err := lbxdata.ReadBuildingInfo(cache)
    if err != nil {
        log.Printf("Error: %v", err)
    } else {
        for i, info := range infos {
            log.Printf("Building %v: %+v", i, info)
        }
    }
}
