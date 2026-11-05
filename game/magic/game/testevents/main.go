package main

import (
    "log"
    "fmt"
    "github.com/bfalese/civ-of-magic/lib/lbx"
    "github.com/bfalese/civ-of-magic/game/magic/game"
)

func main(){
    cache := lbx.AutoCache()
    events, err := game.ReadEventData(cache)
    if err != nil {
        log.Fatal(err)
    }

    for i, event := range events.Events {
        fmt.Printf("%v: %s\n", i, event)
    }
}
