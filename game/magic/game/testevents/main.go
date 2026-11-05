package main

import (
    "log"
    "fmt"
    "github.com/bfalese/civ-of-magic/game/magic/lbxdata"
    "github.com/bfalese/civ-of-magic/lib/lbx"
)

func main(){
    cache := lbx.AutoCache()
    events, err := lbxdata.ReadEventData(cache)
    if err != nil {
        log.Fatal(err)
    }

    for i, event := range events.Events {
        fmt.Printf("%v: %s\n", i, event)
    }
}
