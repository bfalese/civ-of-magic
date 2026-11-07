// serve-web builds the wasm core and serves the web frontend (frontend/web) for development,
// together with the user's exported data (never part of the repo):
//
//   /                  frontend/web
//   /core.wasm         the game core (GOOS=js), built at start
//   /wasm_exec.js      Go's JS glue for that build
//   /gamedata.json     from util/export-gamedata
//   /assets/...        assets.json and sheets from util/export-assets
//
//   go run ./util/serve-web            then open http://localhost:8080
package main

import (
    "flag"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
)

func dataPath(name string) string {
    home, err := os.UserHomeDir()
    if err != nil {
        return name
    }
    return filepath.Join(home, ".local", "share", "civ-of-magic", name)
}

func goEnv(name string) (string, error) {
    out, err := exec.Command("go", "env", name).Output()
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(string(out)), nil
}

func buildCore(outPath string) error {
    command := exec.Command("go", "build", "-o", outPath, "./frontend/wasm")
    command.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
    command.Stdout = os.Stdout
    command.Stderr = os.Stderr
    return command.Run()
}

// no caching while developing: a rebuilt core or edited script shows up on reload
func noCache(handler http.Handler) http.Handler {
    return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
        writer.Header().Set("Cache-Control", "no-store")
        handler.ServeHTTP(writer, request)
    })
}

func main() {
    var address string
    var webPath string
    var gamedataPath string
    var assetsPath string
    var build bool
    flag.StringVar(&address, "addr", "localhost:8080", "address to listen on")
    flag.StringVar(&webPath, "web", "frontend/web", "the web frontend's directory")
    flag.StringVar(&gamedataPath, "gamedata", dataPath("gamedata"), "directory with gamedata.json (util/export-gamedata)")
    flag.StringVar(&assetsPath, "assets", dataPath("assets"), "directory with assets.json (util/export-assets)")
    flag.BoolVar(&build, "build", true, "build the wasm core at start")
    flag.Parse()

    for _, required := range []string{filepath.Join(webPath, "index.html"), filepath.Join(gamedataPath, "gamedata.json"), filepath.Join(assetsPath, "assets.json")} {
        if _, err := os.Stat(required); err != nil {
            log.Fatalf("%v is missing (run from the repo root; export the data with util/export-gamedata and util/export-assets)", required)
        }
    }

    buildDirectory, err := os.MkdirTemp("", "civ-web")
    if err != nil {
        log.Fatal(err)
    }
    defer os.RemoveAll(buildDirectory)
    corePath := filepath.Join(buildDirectory, "core.wasm")
    if build {
        log.Printf("building the wasm core...")
        if err := buildCore(corePath); err != nil {
            log.Fatalf("build: %v", err)
        }
    }

    goRoot, err := goEnv("GOROOT")
    if err != nil {
        log.Fatalf("go env GOROOT: %v", err)
    }

    mux := http.NewServeMux()
    mux.Handle("/", http.FileServer(http.Dir(webPath)))
    mux.HandleFunc("/core.wasm", func(writer http.ResponseWriter, request *http.Request) {
        writer.Header().Set("Content-Type", "application/wasm")
        http.ServeFile(writer, request, corePath)
    })
    mux.HandleFunc("/wasm_exec.js", func(writer http.ResponseWriter, request *http.Request) {
        http.ServeFile(writer, request, filepath.Join(goRoot, "lib", "wasm", "wasm_exec.js"))
    })
    mux.HandleFunc("/gamedata.json", func(writer http.ResponseWriter, request *http.Request) {
        http.ServeFile(writer, request, filepath.Join(gamedataPath, "gamedata.json"))
    })
    mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(assetsPath))))

    fmt.Printf("serving on http://%v\n", address)
    log.Fatal(http.ListenAndServe(address, noCache(mux)))
}
